package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"

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

	// Leaf node
	node := ast.GetNodeAtPos(file.Ast, toCorePos(params.Position))

	if core.IsNil(node) {
		s.warn(ctx, "failed to get leaf node at position")
		return nil, nil
	}

	// Location
	if _, ok := node.(*ast.Leaf); ok {
		for {
			if _, ok := node.(ast.Expr); ok {
				break
			}

			parent := node.Parent()
			if core.IsNil(parent) {
				break
			}

			node = parent
		}
	}

	if expr, ok := node.(ast.Expr); ok {
		if info, ok := file.ExprInfos[expr]; ok && !core.IsNil(info.Node) {
			// Create
			link := protocol.LocationLink{
				TargetURI:   protocol.DocumentURI("file://" + ast.GetFile(info.Node).Path),
				TargetRange: toLspRange(info.Node.Range()),
			}

			if decl, ok := info.Node.(ast.Decl); ok && decl.Name() != nil {
				link.TargetSelectionRange = toLspRange(decl.Name().Range())
			}

			// Return
			if s.definitionLinkSupport {
				return []protocol.LocationLink{link}, nil
			}

			return &protocol.Location{
				URI:   link.TargetURI,
				Range: link.TargetRange,
			}, nil
		}
	}

	return nil, nil
}
