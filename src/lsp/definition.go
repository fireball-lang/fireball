package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"

	"github.com/owenrumney/go-lsp/lsp"
)

func (h *Handler) Definition(ctx context.Context, params *lsp.DefinitionParams) ([]lsp.Location, error) {
	// Get file
	file, locker := h.getFile(uriPath(params.TextDocument.URI))
	if file == nil {
		return nil, nil
	}

	// Lock file
	locker.Lock()
	defer locker.Unlock()

	// Leaf node
	node := ast.GetNodeAtPos(file.Ast, toCorePos(params.Position))

	if core.IsNil(node) {
		h.warning(ctx, "failed to get leaf node at position")
		return nil, nil
	}

	// Location
	if _, ok := node.(*ast.Leaf); ok {
		for {
			if _, ok := node.(ast.Expr); ok {
				break
			}

			node = node.Parent()
		}
	}

	if expr, ok := node.(ast.Expr); ok {
		if info, ok := file.ExprInfos[expr]; ok && !core.IsNil(info.Node) {
			return []lsp.Location{{
				URI:   lsp.DocumentURI("file://" + ast.GetFile(info.Node).Path),
				Range: toLspRange(info.Node.Range()),
			}}, nil
		}
	}

	return nil, nil
}
