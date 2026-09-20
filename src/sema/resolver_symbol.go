package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
	"slices"
)

func (r *resolver) ResolveSymbol(symbol *symbols.Symbol) {
	switch symbol.Kind {
	case symbols.TypeAlias:
		n := symbol.Node.(*ast.TypeAlias)
		t := symbol.Type.(*types.Alias)

		r.typeEnv.RegisterTypeDeclNode(t, n)
		r.nodeTypes[n] = t

		if r.ResolveTypeParams(n.TypeParams, t.TypeParams) {
			defer r.scopes.Pop()
		}

		t.Type = r.ResolveAndAnalyzeType(n.Type)

	case symbols.Struct:
		s := symbol.Node.(*ast.Struct)
		t := symbol.Type.(*types.Struct)

		r.typeEnv.RegisterTypeDeclNode(t, s)
		r.nodeTypes[s] = t

		if r.ResolveTypeParams(s.TypeParams, t.TypeParams) {
			defer r.scopes.Pop()
		}

		if t.Name != "core::Interface" {
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
					Name:     s.Fields[i].Name.Token.Text,
					Type:     typ,
					Public:   s.Fields[i].Public,
					Required: ast.GetAttribute[*ast.Required](s.Fields[i]) != nil,
				}
			}

			t.Optimize()
		}

	case symbols.Enum:
		e := symbol.Node.(*ast.Enum)
		t := symbol.Type.(*types.Enum)

		// Custom case type
		if !core.IsNil(e.Type) {
			typ := r.ResolveAndAnalyzeType(e.Type)

			if typ != types.Invalid {
				if p, ok := typ.(*types.Primitive); !ok || !types.IsInteger(p.Kind) {
					r.Error(e.Type, "the underlying type of an enum can only be an integer type, not '%s'", typ)
					typ = types.Invalid
				}
			}

			t.CaseType = typ
		}

		// Cases
		t.Cases = make([]types.Case, len(e.Cases))

		for i, cas := range e.Cases {
			t.Cases[i] = types.Case{
				Name: cas.Name.Token.Text,
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

		for _, assocConst := range in.AssociatedConsts {
			i := slices.IndexFunc(inType.AssociatedConsts, func(assoc types.AssociatedConst) bool {
				return assoc.Name == assocConst.Name.Token.Text
			})

			if i == -1 {
				panic("sema.resolver.ResolveSymbol() - Failed to find associated constant on types.Interface")
			}

			typ := r.ResolveAndAnalyzeType(assocConst.Type)

			if typ == types.PrimitiveVoid {
				r.Error(assocConst.Type, "associated constant cannot be of type 'void'")
				typ = types.Invalid
			}

			inType.AssociatedConsts[i].Type = typ
		}

		for _, f := range in.Methods {
			m := types.Method{
				Name: f.Name().Token.Text,
				Type: &types.Func{},
			}

			r.ResolveFunc(f, m.Type)

			if f.Receiver != nil {
				selfRef := &types.Reference{Mutable: f.Receiver.Mutable, Pointee: inType.SelfParam}

				m.Type.HasReceiver = true
				m.Type.Params = append([]types.Type{selfRef}, m.Type.Params...)

				inType.InstanceMethods = append(inType.InstanceMethods, m)
			} else {
				inType.StaticMethods = append(inType.StaticMethods, m)
			}
		}

		inType.CopyMethodsToOppositeMutabilityVariant()

	case symbols.Const:
		c := symbol.Node.(*ast.Const)

		typ := r.ResolveAndAnalyzeType(c.Type)

		if typ == types.PrimitiveVoid {
			r.Error(c.Name(), "constant cannot have a void type")
			typ = types.Invalid
		}

		r.nodeTypes[c] = typ
		symbol.Type = typ

	case symbols.Var:
		g := symbol.Node.(*ast.GlobalVar)

		typ := r.ResolveAndAnalyzeType(g.Type)

		if typ == types.PrimitiveVoid {
			r.Error(g.Name(), "global variable cannot have a void type")
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
