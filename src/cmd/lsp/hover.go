package lsp

import (
	"context"
	"fireball/ast"
	"fireball/lexer"
	"github.com/MineGame159/protocol"
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
	case *ast.Param:
		return node.Type.String()

	case *ast.StructInitializerField:
		return node.Value.Result().Type.String()

	case *ast.Var:
		return node.ActualType().String()

	case ast.Expr:
		return node.Result().Type.String()

	default:
		return ""
	}
}
