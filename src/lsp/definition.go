package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/project"
	"fireball/symbols"
	"fireball/types"
	"iter"

	"github.com/fireball-lang/protocol"
)

func (s *Server) Definition(ctx context.Context, params *protocol.DefinitionParams) (interface{}, error) {
	// Get file
	file, locker := s.getFile(uriPath(params.TextDocument.URI))
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

	defNode := s.resolveDefinition(file, node)
	if core.IsNil(defNode) {
		return nil, nil
	}

	return s.buildDefinitionResult(defNode), nil
}

func (s *Server) resolveDefinition(file *project.File, node ast.Node) ast.Node {
	for !core.IsNil(node) {
		switch n := node.(type) {

		// Variable, function, static-method identifier
		case *ast.Identifier:
			if info, ok := file.ExprInfos[n]; ok && !core.IsNil(info.Node) {
				return info.Node
			}

		// Member access: field, method
		case *ast.Member:
			if info, ok := file.ExprInfos[n]; ok && !core.IsNil(info.Node) {
				return info.Node
			}

		// Type annotation
		case *ast.IdentifierType:
			if typ, ok := file.NodeTypes[n]; ok {
				return s.findTypeDeclaration(typ)
			}

		// Leaf
		case *ast.Leaf:
			// Import symbols
			if _, ok := n.Parent().(*ast.Import); ok {
				if typ, ok := file.NodeTypes[n]; ok {
					return s.findTypeDeclaration(typ)
				}
			}

			// Intermediate path entry in a qualified identifier
			if path, ok := n.Parent().(*ast.IdentifierPath); ok {
				isLast := path.Entries[len(path.Entries)-1] == n

				if !isLast {
					if typ, ok := file.NodeTypes[n]; ok {
						return s.findTypeDeclaration(typ)
					}
				}
			}
		}

		node = node.Parent()
	}

	return nil
}

func (s *Server) findTypeDeclaration(typ types.Type) ast.Node {
	switch t := typ.(type) {
	case *types.Struct:
		template := t
		if t.Generic != nil {
			template = t.Generic
		}

		return s.findStructNode(template)

	case *types.Interface:
		template := t.AsImmutable()
		if template.Generic != nil {
			template = template.Generic
		}

		return s.findInterfaceNode(template)

	case *types.Func:
		return s.findFuncNode(t)
	}

	return nil
}

func (s *Server) findStructNode(st *types.Struct) *ast.Struct {
	for sym := range s.allSymbols() {
		if sym.Kind == symbols.Struct && sym.Type == st {
			if n, ok := sym.Node.(*ast.Struct); ok {
				return n
			}
		}
	}

	return nil
}

func (s *Server) findInterfaceNode(in *types.Interface) *ast.Interface {
	for sym := range s.allSymbols() {
		if sym.Kind == symbols.Interface && sym.Type == in {
			if n, ok := sym.Node.(*ast.Interface); ok {
				return n
			}
		}
	}

	return nil
}

func (s *Server) findFuncNode(fn *types.Func) *ast.Func {
	for sym := range s.allSymbols() {
		if sym.Kind == symbols.Func && sym.Type == fn {
			if n, ok := sym.Node.(*ast.Func); ok {
				return n
			}
		}
	}

	return nil
}

func (s *Server) allSymbols() iter.Seq[symbols.Symbol] {
	return func(yield func(symbols.Symbol) bool) {
		for _, workspace := range s.workspaces {
			workspace.mutex.RLock()

			cont := true
			for _, proj := range workspace.projMap {
				for _, file := range proj.Files {
					for _, sym := range file.Symbols {
						if !yield(sym) {
							cont = false
							break
						}
					}
					if !cont {
						break
					}
				}
				if !cont {
					break
				}
			}

			workspace.mutex.RUnlock()

			if !cont {
				return
			}
		}
	}
}

func (s *Server) buildDefinitionResult(defNode ast.Node) interface{} {
	nodeFile := ast.GetFile(defNode)
	if nodeFile == nil {
		return nil
	}

	link := protocol.LocationLink{
		TargetURI:            protocol.DocumentURI("file://" + nodeFile.Path),
		TargetRange:          toLspRange(defNode.Range()),
		TargetSelectionRange: toLspRange(defNode.Range()),
	}

	switch n := defNode.(type) {
	case ast.Decl:
		if n.Name() != nil {
			link.TargetSelectionRange = toLspRange(n.Name().Range())
		}
	case *ast.Field:
		link.TargetSelectionRange = toLspRange(n.Name.Range())
	case *ast.Param:
		link.TargetSelectionRange = toLspRange(n.Name.Range())
	case *ast.Var:
		link.TargetSelectionRange = toLspRange(n.Name.Range())
	}

	if s.definitionLinkSupport {
		return []protocol.LocationLink{link}
	}

	return &protocol.Location{
		URI:   link.TargetURI,
		Range: link.TargetRange,
	}
}
