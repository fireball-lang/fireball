package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/project"

	"github.com/fireball-lang/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	// Get file
	file, locker := s.getFile(params.TextDocument.URI.Filename())
	if file == nil {
		return nil, nil
	}

	// Lock file
	locker.Lock()
	defer locker.Unlock()

	// Deepest node at the cursor position
	node := ast.GetNodeAtPos(file.Ast, toCorePos(params.Position))

	if core.IsNil(node) {
		s.warn(ctx, "failed to get leaf node at position")
		return nil, nil
	}

	// Only resolve references when the cursor is positioned on an identifier
	// token, so that empty space doesn't resolve to an enclosing declaration.
	leaf, ok := node.(*ast.Leaf)
	if !ok || leaf.Token.Kind != lexer.Identifier {
		return nil, nil
	}

	// Resolve the target, either a referenced definition or the declaration itself
	defNode := s.resolveDefinition(file, node)
	if core.IsNil(defNode) {
		defNode = s.declarationAt(node)
	}

	if core.IsNil(defNode) {
		return nil, nil
	}

	// Collect references
	var locations []protocol.Location
	seen := make(map[referenceKey]any)

	add := func(nodeFile *project.File, rng core.Range) {
		key := referenceKey{path: nodeFile.Path, range_: rng}
		if _, ok := seen[key]; ok {
			return
		}

		seen[key] = nil

		locations = append(locations, protocol.Location{
			URI:   uri.File(nodeFile.Path),
			Range: toLspRange(rng),
		})
	}

	s.collectReferences(defNode, add)

	if params.Context.IncludeDeclaration {
		if rng := declNameRange(defNode); rng != (core.Range{}) {
			if nodeFile := ast.GetFile(defNode); nodeFile != nil {
				if projectFile, _ := s.getFile(nodeFile.Path); projectFile != nil {
					add(projectFile, rng)
				}
			}
		}
	}

	return locations, nil
}

type referenceKey struct {
	path   string
	range_ core.Range
}

func (s *Server) declarationAt(node ast.Node) ast.Node {
	leaf, ok := node.(*ast.Leaf)
	if !ok || leaf.Token.Kind != lexer.Identifier {
		return nil
	}

	for parent := leaf.Parent(); !core.IsNil(parent); parent = parent.Parent() {
		switch n := parent.(type) {
		case ast.Decl:
			if n.Name() == leaf {
				return n
			}

		case *ast.Field:
			if n.Name == leaf {
				return n
			}

		case *ast.Param:
			if n.Name == leaf {
				return n
			}

		case *ast.TypeParam:
			if n.Name == leaf {
				return n
			}

		case *ast.AssociatedType:
			if n.Name == leaf {
				return n
			}

		case *ast.Var:
			if n.Name == leaf {
				return n
			}

		case *ast.Case:
			if n.Name == leaf {
				return n
			}

		case *ast.Import:
			return nil
		}
	}

	return nil
}

func (s *Server) collectReferences(defNode ast.Node, add func(*project.File, core.Range)) {
	for _, workspace := range s.workspaces {
		workspace.mutex.RLock()

		for _, proj := range workspace.projMap {
			for _, file := range proj.Files {
				s.collectFileReferences(file, defNode, add)
			}
		}

		workspace.mutex.RUnlock()
	}
}

func (s *Server) collectFileReferences(file *project.File, defNode ast.Node, add func(*project.File, core.Range)) {
	// Expression references: variables, functions, members, enum cases, ...
	for expr, info := range file.ExprInfos {
		if core.IsNil(info.Node) || info.Node != defNode {
			continue
		}

		switch n := expr.(type) {
		case *ast.Identifier:
			if len(n.Path) > 0 {
				add(file, n.Path[len(n.Path)-1].Name.Range())
			}

		case *ast.Member:
			add(file, n.Name.Range())

		case *ast.FieldInitializer:
			add(file, n.Name.Range())
		}
	}

	// Type references: type annotations, casts, struct literals, generic args, ...
	for node, typ := range file.NodeTypes {
		if node == defNode {
			continue
		}

		if decl := s.findTypeDeclaration(typ); decl != defNode {
			continue
		}

		switch n := node.(type) {
		case *ast.IdentifierType:
			if len(n.Path) > 0 {
				add(file, n.Path[len(n.Path)-1].Name.Range())
			}

		case *ast.IdentifierEntry:
			if n.Name != nil {
				add(file, n.Name.Range())
			}

		case *ast.Leaf:
			add(file, n.Range())

		case *ast.SelfType:
			add(file, n.Range())
		}
	}
}

func declNameRange(node ast.Node) core.Range {
	switch n := node.(type) {
	case ast.Decl:
		if n.Name() != nil {
			return n.Name().Range()
		}

	case *ast.Field:
		return n.Name.Range()

	case *ast.TypeParam:
		return n.Name.Range()

	case *ast.AssociatedType:
		return n.Name.Range()

	case *ast.Param:
		if n.Name != nil {
			return n.Name.Range()
		}

	case *ast.Var:
		return n.Name.Range()

	case *ast.Case:
		return n.Name.Range()
	}

	return core.Range{}
}
