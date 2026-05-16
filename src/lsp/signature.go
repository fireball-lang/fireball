package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"

	"github.com/fireball-lang/protocol"
)

func (s *Server) SignatureHelp(_ context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	// Get file
	file, locker := s.getFile(uriPath(params.TextDocument.URI))
	if file == nil {
		return nil, nil
	}

	// Lock file
	locker.Lock()
	defer locker.Unlock()

	// Find deepest node at cursor position
	pos := toCorePos(params.Position)

	node := ast.GetNodeAtPos(file.Ast, pos)
	if core.IsNil(node) {
		return nil, nil
	}

	// Walk up to find the enclosing call expression.
	call := findEnclosingCall(node, pos)
	if call == nil {
		return nil, nil
	}

	// Resolve the callee to an ast.Func declaration
	defNode := s.resolveDefinition(file, call.Callee)
	if core.IsNil(defNode) {
		return nil, nil
	}

	funcNode, ok := defNode.(*ast.Func)
	if !ok {
		return nil, nil
	}

	// Build the full signature label
	label := funcNode.String(true)

	// Build parameter information (labels are substrings of the full label for highlighting)
	paramInfos := buildParamInfos(funcNode)

	// Determine the active parameter index
	activeParam := activeParamIndex(call, pos)

	return &protocol.SignatureHelp{
		Signatures: []protocol.SignatureInformation{
			{
				Label:      label,
				Parameters: paramInfos,
			},
		},
		ActiveSignature: 0,
		ActiveParameter: activeParam,
	}, nil
}

func findEnclosingCall(node ast.Node, pos core.Pos) *ast.Call {
	for !core.IsNil(node) {
		if call, ok := node.(*ast.Call); ok {
			if !call.Callee.Range().Contains(pos) {
				return call
			}
		}

		node = node.Parent()
	}

	return nil
}

func buildParamInfos(fn *ast.Func) []protocol.ParameterInformation {
	infos := make([]protocol.ParameterInformation, 0, len(fn.Params))

	for _, p := range fn.Params {
		infos = append(infos, protocol.ParameterInformation{
			Label: p.Name.Token.Text + ": " + p.Type.String(),
		})
	}

	return infos
}

func activeParamIndex(call *ast.Call, pos core.Pos) uint32 {
	for i, arg := range call.Args {
		end := arg.Range().End

		if pos.Line < end.Line || (pos.Line == end.Line && pos.Column <= end.Column) {
			return uint32(i)
		}
	}

	return uint32(len(call.Args))
}
