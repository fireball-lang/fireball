package comptime

import (
	"fireball/ast"
	"fireball/core"
	"fireball/ir/eval"
	"fireball/types"
	"math"
)

func (e *Evaluator) ensureEnum(decl *ast.Enum, errNode ast.Node) bool {
	ok := e.request(decl, errNode, func() evaluationState {
		file := ast.GetFile(decl)
		fd := e.fileData(decl)

		typ, ok := fd.NodeTypes[decl].(*types.Enum)
		if !ok || typ.CaseType == types.Invalid {
			return failed
		}

		// Evaluate cases separately
		result := *typ
		result.Cases = append([]types.EnumCase(nil), typ.Cases...)

		allowedMin := core.Unsigned(true, math.MaxUint64)
		allowedMax := core.Unsigned(false, math.MaxUint64)

		if !core.IsNil(typ.CaseType) {
			allowedMin, allowedMax = typ.CaseType.(*types.Primitive).Kind.IntegerRange()
		}

		current := core.Signed(0)
		valueMin := core.Unsigned(false, math.MaxUint64)
		valueMax := core.Unsigned(true, math.MaxUint64)
		valid := true

		for i, cas := range decl.Cases {
			if !core.IsNil(cas.Value) {
				info := fd.ExprInfos[cas.Value]
				var kind types.PrimitiveKind

				switch typ := info.Type.(type) {
				case *types.Integer:
					kind = typ.ToPrimitive().Kind
				case *types.Primitive:
					if !types.IsInteger(typ.Kind) {
						return failed
					}
					kind = typ.Kind
				default:
					return failed
				}

				if !e.ensureExpr(cas.Value, info.Type, cas.Name) {
					return failed
				}

				value := fd.Evaluations[cas.Value].(*eval.IntValue)

				if types.IsUnsignedInteger(kind) {
					current = core.Unsigned(false, value.Value)
				} else {
					current = core.TwosComplementWidth(value.Value, kind.Size()*8)
				}
			}

			if current.LessThan(allowedMin) || current.GreaterThan(allowedMax) {
				node := ast.Node(cas.Value)
				if core.IsNil(node) {
					node = cas.Name
				}

				e.error(node, "value '%s' doesn't fit inside type '%s'", current, typ.CaseType)
				valid = false
			}

			result.Cases[i].Value = current
			valueMin = valueMin.Min(current)
			valueMax = valueMax.Max(current)
			current = current.AddOne()
		}

		before := len(e.Diagnostics(file))

		e.checkDuplicateEnumCaseValues(decl, &result)
		e.inferEnumCaseType(decl, &result, valueMin, valueMax)

		if !valid || result.CaseType == types.Invalid || len(e.Diagnostics(file)) != before {
			return failed
		}

		typ.Cases = result.Cases
		typ.CaseType = result.CaseType
		return done
	})

	if !ok && e.states[decl] == failed {
		if typ, ok := e.fileData(decl).NodeTypes[decl].(*types.Enum); ok && core.IsNil(typ.CaseType) {
			typ.CaseType = types.Invalid
		}
	}

	return ok
}

func (e *Evaluator) checkDuplicateEnumCaseValues(decl *ast.Enum, typ *types.Enum) {
	values := make(map[core.Integer]any)

	for i, cas := range decl.Cases {
		value := typ.Cases[i].Value

		if _, ok := values[value]; ok {
			node := ast.Node(cas.Value)
			if core.IsNil(node) {
				node = cas.Name
			}

			e.error(node, "case with value '%s' already exists", value)
		}

		values[value] = nil
	}
}

func (e *Evaluator) inferEnumCaseType(decl *ast.Enum, typ *types.Enum, valueMin, valueMax core.Integer) {
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
			e.error(decl.Name(), "failed to infer enum case type for '%s'", decl.Name().Token.Text)
		}
	}
}
