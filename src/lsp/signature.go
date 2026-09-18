package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/project"
	"fireball/types"

	"github.com/fireball-lang/protocol"
)

func (s *Server) SignatureHelp(_ context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	// Get file
	file, locker := s.getFile(params.TextDocument.URI.Filename())
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

	return s.buildSignatureHelp(file, call, pos), nil
}

func (s *Server) buildSignatureHelp(file *project.File, call *ast.Call, pos core.Pos) *protocol.SignatureHelp {
	fn, typ := s.resolveFuncSignature(file, call.Callee)

	var signature protocol.SignatureInformation

	if fn != nil {
		signature = protocol.SignatureInformation{
			Label:         fn.String(true),
			Documentation: s.markup(fn.Documentation()),
			Parameters:    buildParamInfos(fn),
		}
	} else if typ != nil {
		signature = protocol.SignatureInformation{
			Label:      typ.String(),
			Parameters: buildTypeParamInfos(typ),
		}
	} else {
		return nil
	}

	return &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{signature},
		ActiveSignature: 0,
		ActiveParameter: activeParamIndex(call, pos),
	}
}

func (s *Server) resolveFuncSignature(file *project.File, callee ast.Expr) (*ast.Func, *types.Func) {
	// Direct reference to a function declaration
	if defNode := s.resolveDefinition(file, callee); !core.IsNil(defNode) {
		if fn, ok := defNode.(*ast.Func); ok {
			return fn, nil
		}
	}

	// Value of function type (function pointer)
	info, ok := file.ExprInfos[callee]
	if !ok {
		return nil, nil
	}

	typ, ok := info.Type.(*types.Func)
	if !ok {
		return nil, nil
	}

	template := typ
	if typ.Generic != nil {
		template = typ.Generic
	}

	// Function type with a known declaration
	if fn := s.findFuncNode(template); fn != nil {
		return fn, nil
	}

	// Anonymous function type without a declaration
	return nil, typ
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

func buildTypeParamInfos(fn *types.Func) []protocol.ParameterInformation {
	infos := make([]protocol.ParameterInformation, 0, len(fn.Params))

	for _, param := range fn.Params {
		infos = append(infos, protocol.ParameterInformation{
			Label: param.String(),
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
