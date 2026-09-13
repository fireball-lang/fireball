package comptime

import (
	"fireball/ast"
	"fireball/core"
	"fireball/types"
)

func (ct *compTime) CheckDuplicateEnumCaseValues(decl *ast.Enum, typ *types.Enum) {
	values := make(map[core.Integer]any)

	for i, cas := range decl.Cases {
		value := typ.Cases[i].Value

		if _, ok := values[value]; ok {
			node := ast.Node(cas.Value)
			if core.IsNil(node) {
				node = cas.Name
			}

			ct.Error(node, "case with value '%s' already exists", value)
		}

		values[value] = nil
	}
}

func (ct *compTime) InferEnumCaseType(decl *ast.Enum, typ *types.Enum, valueMin, valueMax core.Integer) {
	if core.IsNil(typ.CaseType) {
		if valueMin.Negative() || valueMax.Negative() {
			if integerFitsInKind(valueMin, types.I8) && integerFitsInKind(valueMax, types.I8) {
				typ.CaseType = types.PrimitiveI8
			} else if integerFitsInKind(valueMin, types.I16) && integerFitsInKind(valueMax, types.I16) {
				typ.CaseType = types.PrimitiveI16
			} else if integerFitsInKind(valueMin, types.I32) && integerFitsInKind(valueMax, types.I32) {
				typ.CaseType = types.PrimitiveI32
			} else if integerFitsInKind(valueMin, types.I64) && integerFitsInKind(valueMax, types.I64) {
				typ.CaseType = types.PrimitiveI64
			} else {
				typ.CaseType = types.Invalid
			}
		} else {
			if integerFitsInKind(valueMin, types.U8) && integerFitsInKind(valueMax, types.U8) {
				typ.CaseType = types.PrimitiveU8
			} else if integerFitsInKind(valueMin, types.U16) && integerFitsInKind(valueMax, types.U16) {
				typ.CaseType = types.PrimitiveU16
			} else if integerFitsInKind(valueMin, types.U32) && integerFitsInKind(valueMax, types.U32) {
				typ.CaseType = types.PrimitiveU32
			} else if integerFitsInKind(valueMin, types.U64) && integerFitsInKind(valueMax, types.U64) {
				typ.CaseType = types.PrimitiveU64
			} else {
				typ.CaseType = types.Invalid
			}
		}

		if typ.CaseType == types.Invalid {
			ct.Error(decl.Name(), "failed to infer enum case type for '%s'", decl.Name().Token.Text)
		}
	}
}
