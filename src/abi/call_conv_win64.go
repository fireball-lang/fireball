package abi

import (
	"fireball/ast"
)

type callConvWin64 struct{}

var Win64 CallConv = &callConvWin64{}

func (c *callConvWin64) Classify(type_ ast.Type) []Reg {
	switch type_ := type_.(type) {
	case *ast.PrimitiveType:
		if type_.Kind == ast.Void {
			return nil
		}

		size, _ := TypeInfo(AMD64, type_)
		class := SSE

		if type_.Kind == ast.Bool || type_.Kind.IsInteger() {
			class = Integer
		}

		return []Reg{{class, size}}

	case *ast.PointerType, ast.FuncType:
		return []Reg{{Integer, AMD64.WordSize}}

	case *ast.DeclType, *ast.ArrayType:
		size, _ := TypeInfo(AMD64, type_)

		if size > AMD64.WordSize {
			return []Reg{{Memory, size}}
		}

		return []Reg{{Integer, size}}

	default:
		panic("abi.Win64.Classify() - Invalid type")
	}
}
