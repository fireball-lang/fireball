package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/project"
	"fireball/types"
	"strings"

	"github.com/fireball-lang/protocol"
)

func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (result *protocol.Hover, err error) {
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

	// Resolve hover
	return s.resolveHover(file, node), nil
}

func (s *Server) resolveHover(file *project.File, node ast.Node) *protocol.Hover {
	// Only resolve hover when the cursor is positioned on an identifier token (or
	// the 'Self' type), so that hovering empty space (e.g. between parenthesis
	// or brackets) doesn't match an enclosing declaration.
	if leaf, ok := node.(*ast.Leaf); ok {
		if leaf.Token.Kind != lexer.Identifier {
			return nil
		}
	} else if _, ok := node.(*ast.SelfType); !ok {
		return nil
	}

	rng := node.Range()

outer:
	for !core.IsNil(node) {
		switch n := node.(type) {

		// Variable, function, static-method identifier, member access
		// (field, method), struct initializer field and offsetof field
		case *ast.Identifier, *ast.Member, *ast.FieldInitializer, *ast.OffsetOf:
			info, ok := file.ExprInfos[n]
			if !ok || core.IsNil(info.Node) {
				break outer
			}

			return s.buildHover(file, info.Node, rng)

		// Type annotation
		case *ast.IdentifierType:
			typ, ok := resolveIdentifierType(file, n)
			if !ok {
				break outer
			}

			if decl := s.findTypeDeclaration(typ); !core.IsNil(decl) {
				return s.buildHover(file, decl, rng)
			}

			return s.buildTypeHover(typ, rng)

		// Self type
		case *ast.SelfType:
			typ := s.resolveSelf(file, n)
			if core.IsNil(typ) {
				break outer
			}

			if decl := s.findTypeDeclaration(typ); !core.IsNil(decl) {
				return s.buildHover(file, decl, rng)
			}

			return s.buildTypeHover(typ, rng)

		// Leaf
		case *ast.Leaf:
			// Import symbols
			if _, ok := n.Parent().(*ast.Import); ok {
				typ, ok := file.NodeTypes[n]
				if !ok {
					break outer
				}

				if decl := s.findTypeDeclaration(typ); !core.IsNil(decl) {
					return s.buildHover(file, decl, rng)
				}
			}

		// IdentifierEntry
		case *ast.IdentifierEntry:
			var path []*ast.IdentifierEntry

			switch parent := n.Parent().(type) {
			case *ast.Identifier:
				path = parent.Path
			case *ast.IdentifierType:
				path = parent.Path
			}

			isLast := path[len(path)-1] == n

			if !isLast {
				typ, ok := file.NodeTypes[n]
				if !ok {
					break outer
				}

				if decl := s.findTypeDeclaration(typ); !core.IsNil(decl) {
					return s.buildHover(file, decl, rng)
				}
			}
		}

		// Declarations: hovering over their own identifiers (buildHover
		// returns nil for every other node type)
		if hover := s.buildHover(file, node, rng); hover != nil {
			return hover
		}

		node = node.Parent()
	}

	return nil
}

func (s *Server) buildHover(file *project.File, node ast.Node, rng core.Range) *protocol.Hover {
	var label string

	switch n := node.(type) {
	case *ast.Func:
		label = n.String(true)

	case *ast.Struct:
		label = declTypeString(file, n, n.Name().Token.Text)

	case *ast.Enum:
		label = declTypeString(file, n, n.Name().Token.Text)

	case *ast.Interface:
		label = declTypeString(file, n, n.Name().Token.Text)

	case *ast.TypeAlias:
		var sb strings.Builder

		sb.WriteString("type ")
		sb.WriteString(n.Name().Token.Text)

		if len(n.TypeParams) > 0 {
			sb.WriteRune('[')

			for i, param := range n.TypeParams {
				if i > 0 {
					sb.WriteString(", ")
				}

				sb.WriteString(param.Name.Token.Text)
			}

			sb.WriteRune(']')
		}

		sb.WriteString(" = ")
		sb.WriteString(n.Type.String())

		label = sb.String()

	case *ast.GlobalVar:
		label = typeString(file, n, n.Type)

	case *ast.Field:
		label = typeString(file, n, n.Type)

	case *ast.Param:
		label = typeString(file, n, n.Type)

	case *ast.Var:
		label = typeString(file, n, n.Type)

	case *ast.TypeParam:
		label = n.Name.Token.Text

		if len(n.Constraints) > 0 {
			var sb strings.Builder

			sb.WriteString(label)
			sb.WriteString(": ")

			for i, c := range n.Constraints {
				if i > 0 {
					sb.WriteString(" + ")
				}

				sb.WriteString(c.String())
			}

			label = sb.String()
		}

	case *ast.AssociatedType:
		label = n.Name.Token.Text

		if !core.IsNil(n.Type) {
			label += ": " + n.Type.String()
		}

	default:
		return nil
	}

	documentation := hoverDocumentation(node)

	return s.formatHover(label, documentation, rng)
}

func hoverDocumentation(node ast.Node) []*ast.Leaf {
	if decl, ok := node.(ast.Decl); ok {
		return decl.Documentation()
	}

	if field, ok := node.(*ast.Field); ok {
		return field.Documentation
	}

	return nil
}

func (s *Server) buildTypeHover(typ types.Type, rng core.Range) *protocol.Hover {
	if typ == nil || typ == types.Invalid {
		return nil
	}

	return s.formatHover(typ.String(), nil, rng)
}

func (s *Server) formatHover(label string, documentation []*ast.Leaf, rng core.Range) *protocol.Hover {
	if label == "" {
		return nil
	}

	var value strings.Builder

	value.WriteString("```fireball\n")
	value.WriteString(label)
	value.WriteString("\n```")

	if text := s.markup(documentation); len(text.Value) > 0 {
		value.WriteString("\n\n---\n\n")
		value.WriteString(text.Value)
	}

	lspRange := toLspRange(rng)

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: value.String(),
		},
		Range: &lspRange,
	}
}

func typeString(file *project.File, node ast.Node, astType ast.Type) string {
	if !core.IsNil(astType) {
		return astType.String()
	}

	if typ, ok := file.NodeTypes[node]; ok && typ != nil && typ != types.Invalid {
		return typ.String()
	}

	return ""
}

func declTypeString(file *project.File, node ast.Node, fallback string) string {
	if typ, ok := file.NodeTypes[node]; ok && typ != nil {
		return typ.String()
	}

	return fallback
}
