package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"math"
)

func (a *analyzer) ResolveSymbol(symbol *symbols.Symbol) {
	switch symbol.Kind {
	case symbols.Struct:
		s := symbol.Node.(*ast.Struct)
		t := symbol.Type.(*types.Struct)

		a.typeEnv.RegisterStruct(t, s)

		if a.resolveTypeParams(s.TypeParams, t.TypeParams) {
			defer a.scopes.Pop()
		}

		t.Fields = make([]types.Field, len(s.Fields))

		for i := 0; i < len(s.Fields); i++ {
			typ := a.AnalyzeType(s.Fields[i].Type)

			if typ == types.PrimitiveVoid {
				typ = types.Invalid
			}

			t.Fields[i] = types.Field{
				Name:   s.Fields[i].Name.Token.Text,
				Type:   typ,
				Public: s.Fields[i].Public,
			}
		}

	case symbols.Enum:
		e := symbol.Node.(*ast.Enum)
		t := symbol.Type.(*types.Enum)

		allowedMin := core.Unsigned(true, math.MaxUint64)
		allowedMax := core.Unsigned(false, math.MaxUint64)

		// Custom case type
		if !core.IsNil(e.Type) {
			typ := a.AnalyzeType(e.Type)

			if typ != types.Invalid {
				if p, ok := typ.(*types.Primitive); !ok || !types.IsInteger(p.Kind) {
					a.Error(e.Type, "the underlying type of an enum can only be an integer type, not '%s'", typ)
					typ = types.Invalid
				}
			}

			if typ != types.Invalid {
				allowedMin, allowedMax = typ.(*types.Primitive).Kind.IntegerRange()
			}

			t.CaseType = typ
		}

		// Cases
		t.Cases = make([]types.Case, 0, len(e.Cases))

		current := core.Signed(0)

		valueMin := core.Unsigned(false, math.MaxUint64)
		valueMax := core.Unsigned(true, math.MaxUint64)

		for _, c := range e.Cases {
			if c.Value != nil {
				current = lexer.ParseInteger(c.Value.Token)
			}

			t.Cases = append(t.Cases, types.Case{
				Name:  c.Name.Token.Text,
				Value: current,
			})

			if !core.IsNil(e.Type) && (current.LessThan(allowedMin) || current.GreaterThan(allowedMax)) {
				node := c.Value
				if node == nil {
					node = c.Name
				}

				a.Error(node, "value '%s' doesn't fit inside type '%s'", current, t.CaseType)
			}

			valueMin = valueMin.Min(current)
			valueMax = valueMax.Max(current)

			current = current.AddOne()
		}

		// Inferred case type
		if core.IsNil(e.Type) {
			if valueMin.Negative() || valueMax.Negative() {
				if integerFitsInKind(valueMin, types.I8) && integerFitsInKind(valueMax, types.I8) {
					t.CaseType = types.PrimitiveI8
				} else if integerFitsInKind(valueMin, types.I16) && integerFitsInKind(valueMax, types.I16) {
					t.CaseType = types.PrimitiveI16
				} else if integerFitsInKind(valueMin, types.I32) && integerFitsInKind(valueMax, types.I32) {
					t.CaseType = types.PrimitiveI32
				} else if integerFitsInKind(valueMin, types.I64) && integerFitsInKind(valueMax, types.I64) {
					t.CaseType = types.PrimitiveI64
				} else {
					t.CaseType = types.Invalid
				}
			} else {
				if integerFitsInKind(valueMin, types.U8) && integerFitsInKind(valueMax, types.U8) {
					t.CaseType = types.PrimitiveU8
				} else if integerFitsInKind(valueMin, types.U16) && integerFitsInKind(valueMax, types.U16) {
					t.CaseType = types.PrimitiveU16
				} else if integerFitsInKind(valueMin, types.U32) && integerFitsInKind(valueMax, types.U32) {
					t.CaseType = types.PrimitiveU32
				} else if integerFitsInKind(valueMin, types.U64) && integerFitsInKind(valueMax, types.U64) {
					t.CaseType = types.PrimitiveU64
				} else {
					t.CaseType = types.Invalid
				}
			}

			if t.CaseType == types.Invalid {
				a.Error(e.Name(), "failed to infer enum case type for '%s'", e.Name().Token.Text)
			}
		}

		a.typeEnv.RegisterEnum(t, e)

	case symbols.Interface:
		in := symbol.Node.(*ast.Interface)
		inType := symbol.Type.(*types.Interface)

		a.typeEnv.RegisterInterface(inType, in)

		prevSelf := a.selfType
		a.selfType = inType.SelfParam
		defer func() { a.selfType = prevSelf }()

		if a.resolveTypeParams(in.TypeParams, inType.TypeParams) {
			defer a.scopes.Pop()
		}

		inType.InstanceMethods = nil
		inType.StaticMethods = nil

		for _, f := range in.Methods {
			m := types.Method{
				Name: f.Name().Token.Text,
				Type: &types.Func{},
			}

			a.resolveFunc(f, m.Type)

			if f.Receiver != nil {
				selfPtr := &types.Pointer{Mutable: f.Receiver.Mutable, Pointee: inType.SelfParam}
				m.Type.Params = append([]types.Type{selfPtr}, m.Type.Params...)

				inType.InstanceMethods = append(inType.InstanceMethods, m)
			} else {
				inType.StaticMethods = append(inType.StaticMethods, m)
			}
		}

	case symbols.Func:
		f := symbol.Node.(*ast.Func)
		t := symbol.Type.(*types.Func)

		a.resolveFunc(f, t)

	default:
		panic("sema.analyzer.ResolveSymbol() - Invalid symbol kind")
	}
}

func (a *analyzer) resolveFunc(f *ast.Func, t *types.Func) {
	if a.resolveTypeParams(f.TypeParams, t.TypeParams) {
		defer a.scopes.Pop()
	}

	t.Params = make([]types.Type, len(f.Params))
	t.VarArgs = f.VarArgs

	for i := 0; i < len(f.Params); i++ {
		typ := a.AnalyzeType(f.Params[i].Type)

		if typ == types.PrimitiveVoid {
			typ = types.Invalid
		}

		t.Params[i] = typ
	}

	t.Returns = a.AnalyzeType(f.Returns)
}

func (a *analyzer) resolveTypeParams(astParams []*ast.TypeParam, typeParams []*types.Param) bool {
	if len(astParams) == 0 {
		return false
	}

	a.scopes.Push(&symbols.ParamScope{
		Params: typeParams,
		Nodes:  astParams,
	})

	for i, param := range astParams {
		a.nodeTypes[param] = typeParams[i]

		for _, constraintAst := range param.Constraints {
			constraint := a.AnalyzeType(constraintAst)

			if in, ok := constraint.(*types.Interface); ok {
				typeParams[i].Constraints = append(typeParams[i].Constraints, in)
			} else {
				a.Error(constraintAst, "constraint must be an interface type, got '%s'", constraint)
			}
		}
	}

	return true
}

func integerFitsInKind(value core.Integer, kind types.PrimitiveKind) bool {
	kMin, kMax := kind.IntegerRange()
	return value.GreaterThanEqual(kMin) && value.LessThanEqual(kMax)
}
