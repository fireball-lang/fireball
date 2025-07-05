package lsp

import (
	"context"
	"fireball/ast"
	"fireball/lexer"
	"github.com/MineGame159/protocol"
	"slices"
)

func (s *server) Hover(_ context.Context, params *protocol.HoverParams) (result *protocol.Hover, err error) {
	defer stop(start(s, "Hover"))

	// Get file
	file := s.getFile(params.TextDocument.URI)
	if file == nil {
		return nil, nil
	}

	// Compute
	s.astMutex.RLock()
	defer s.astMutex.RUnlock()

	leaf := ast.GetLeafAtPos(file.Ast(), lexer.Pos{
		Line:   params.Position.Line + 1,
		Column: params.Position.Character,
	})

	if leaf == nil {
		return nil, nil
	}

	if hover := getHover(leaf); hover != "" {
		return &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.PlainText,
				Value: hover,
			},
			Range: rangeToProtocolPtr(leaf.Range()),
		}, nil
	}

	return nil, nil
}

func getHover(leaf *ast.Leaf) string {
	switch node := leaf.Parent().(type) {
	case *ast.Import:
		if node.ResolvedSymbols == nil {
			return ""
		}

		index := slices.Index(node.Symbols, leaf)
		resolved := node.ResolvedSymbols[index]

		if ast.IsValid(resolved) {
			switch resolved := resolved.(type) {
			case *ast.Struct:
				return resolved.Name()
			case *ast.Func:
				return resolved.StringWithParamNames()
			default:
				panic("cmd.lsp.getHover() - Invalid import type")
			}
		}

		return ""

	case *ast.Param:
		return node.Type.String()

	case *ast.StructInitializerField:
		return node.Value.Result().Type.String()

	case *ast.Var:
		if ast.IsValid(node.ActualType()) {
			return node.ActualType().String()
		}

		return ""

	case ast.Expr:
		return node.Result().Type.String()

	case *ast.Path:
		if expr, ok := node.Parent().(ast.Expr); ok {
			return expr.Result().Type.String()
		}

		return ""

	default:
		return ""
	}
}
