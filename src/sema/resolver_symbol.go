package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"math"
)

func (r *resolver) ResolveSymbol(symbol *symbols.Symbol) {
	switch symbol.Kind {
	case symbols.Struct:
		s := symbol.Node.(*ast.Struct)
		t := symbol.Type.(*types.Struct)

		r.typeEnv.RegisterTypeDeclNode(t, s)
		r.nodeTypes[s] = t

		if r.ResolveTypeParams(s.TypeParams, t.TypeParams) {
			defer r.scopes.Pop()
		}

		t.Fields = make([]types.Field, len(s.Fields))

		for i := 0; i < len(s.Fields); i++ {
			typ := r.ResolveAndAnalyzeType(s.Fields[i].Type)

			if typ == types.PrimitiveVoid {
				typ = types.Invalid
			} else if typ == t {
				r.Error(s.Fields[i].Type, "recursive structs are not allowed without pointers")
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
			typ := r.ResolveAndAnalyzeType(e.Type)

			if typ != types.Invalid {
				if p, ok := typ.(*types.Primitive); !ok || !types.IsInteger(p.Kind) {
					r.Error(e.Type, "the underlying type of an enum can only be an integer type, not '%s'", typ)
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

				r.Error(node, "value '%s' doesn't fit inside type '%s'", current, t.CaseType)
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
				r.Error(e.Name(), "failed to infer enum case type for '%s'", e.Name().Token.Text)
			}
		}

		r.typeEnv.RegisterTypeDeclNode(t, e)
		r.nodeTypes[e] = t

	case symbols.Interface:
		in := symbol.Node.(*ast.Interface)
		inType := symbol.Type.(*types.Interface)

		r.typeEnv.RegisterTypeDeclNode(inType, in)
		r.nodeTypes[in] = inType

		prevSelf := r.selfType
		r.selfType = inType.SelfParam
		defer func() { r.selfType = prevSelf }()

		if r.ResolveTypeParams(in.TypeParams, inType.TypeParams) {
			defer r.scopes.Pop()
		}
		if r.ResolveAssociatedTypeParams(in.AssociatedTypes, inType.AssociatedTypes) {
			defer r.scopes.Pop()
		}

		inType.InstanceMethods = nil
		inType.StaticMethods = nil

		for _, f := range in.Methods {
			m := types.Method{
				Name: f.Name().Token.Text,
				Type: &types.Func{},
			}

			r.ResolveFunc(f, m.Type)

			if f.Receiver != nil {
				selfRef := &types.Reference{Mutable: f.Receiver.Mutable, Pointee: inType.SelfParam}
				m.Type.Params = append([]types.Type{selfRef}, m.Type.Params...)

				inType.InstanceMethods = append(inType.InstanceMethods, m)
			} else {
				inType.StaticMethods = append(inType.StaticMethods, m)
			}
		}

		inType.CopyMethodsToOppositeMutabilityVariant()

	case symbols.Var:
		g := symbol.Node.(*ast.GlobalVar)

		typ := r.ResolveAndAnalyzeType(g.Type)

		if typ == types.PrimitiveVoid {
			typ = types.Invalid
		}

		r.nodeTypes[g] = typ
		symbol.Type = typ

	case symbols.Func:
		f := symbol.Node.(*ast.Func)
		t := symbol.Type.(*types.Func)

		r.ResolveFunc(f, t)

	default:
		panic("sema.analyzer.ResolveSymbol() - Invalid symbol kind")
	}
}

func (r *resolver) ResolveFunc(f *ast.Func, t *types.Func) {
	if r.ResolveTypeParams(f.TypeParams, t.TypeParams) {
		defer r.scopes.Pop()
	}

	t.Params = make([]types.Type, len(f.Params))
	t.VarArgs = f.VarArgs

	for i := 0; i < len(f.Params); i++ {
		typ := r.ResolveAndAnalyzeType(f.Params[i].Type)

		if typ == types.PrimitiveVoid {
			typ = types.Invalid
		}

		t.Params[i] = typ
	}

	t.Returns = r.ResolveAndAnalyzeType(f.Returns)

	r.nodeTypes[f] = t
}

func (r *resolver) ResolveTypeParams(astParams []*ast.TypeParam, typeParams []*types.Param) bool {
	if len(astParams) == 0 {
		return false
	}

	r.scopes.Push(&symbols.ParamScope{
		Params: typeParams,
		Nodes:  astParams,
	})

	for i, param := range astParams {
		r.nodeTypes[param] = typeParams[i]

		for _, constraintAst := range param.Constraints {
			constraint := r.ResolveAndAnalyzeType(constraintAst)

			if in, ok := constraint.(*types.Interface); ok {
				typeParams[i].Constraints = append(typeParams[i].Constraints, in)
			} else {
				r.Error(constraintAst, "constraint must be an interface type, got '%s'", constraint)
			}
		}
	}

	return true
}

func (r *resolver) ResolveAssociatedTypeParams(astAssocTypes []*ast.AssociatedType, typeAssocTypes []*types.Param) bool {
	if len(astAssocTypes) == 0 {
		return false
	}

	syms := make([]symbols.Symbol, 0, len(astAssocTypes))

	for i, assocType := range astAssocTypes {
		r.nodeTypes[assocType] = typeAssocTypes[i]

		syms = append(syms, symbols.Symbol{
			Kind: symbols.TypeParam,
			Name: assocType.Name.Token.Text,
			Node: assocType.Type,
			Type: typeAssocTypes[i],
		})
	}

	r.scopes.Push(symbols.SymbolScope(syms))

	return true
}

func integerFitsInKind(value core.Integer, kind types.PrimitiveKind) bool {
	kMin, kMax := kind.IntegerRange()
	return value.GreaterThanEqual(kMin) && value.LessThanEqual(kMax)
}
