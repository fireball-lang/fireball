package lsp

import (
	"context"
	"fireball/ast"
	"fireball/lexer"
	"github.com/MineGame159/protocol"
	"go.lsp.dev/uri"
)

func (s *server) Definition(_ context.Context, params *protocol.DefinitionParams) (result []protocol.Location, err error) {
	defer stop(start(s, "Definition"))

	// Get file
	file := s.getFile(params.TextDocument.URI)
	if file == nil {
		return nil, nil
	}

	// Get leaf
	s.astMutex.RLock()
	defer s.astMutex.RUnlock()

	leaf := ast.GetLeafAtPos(file.Ast(), lexer.Pos{
		Line:   params.Position.Line + 1,
		Column: params.Position.Character,
	})

	if leaf == nil {
		return nil, nil
	}

	// Get location
	node := getDefinitionNode(leaf)

	if ast.IsValid(node) {
		return []protocol.Location{{
			URI:   uri.New(ast.Root(node).AbsolutePath),
			Range: rangeToProtocol(node.Range()),
		}}, nil
	}

	return nil, nil
}

func getDefinitionNode(l *ast.Leaf) ast.Node {
	switch p1 := l.Parent().(type) {
	case *ast.Path:
		switch p2 := p1.Parent().(type) {
		case *ast.DeclType:
			return p2.Decl
		case *ast.Identifier:
			return p2.Resolved
		}

	case *ast.Member:
		return p1.Resolved
	}

	return nil
}
