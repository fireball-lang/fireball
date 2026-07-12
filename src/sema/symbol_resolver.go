package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
	"slices"
)

func ResolveSymbols(file *ast.File, fileSymbols []symbols.Symbol, instantiations types.InstantiationCache, typeEnv *TypeEnvironment, root symbols.Scope, path string) (map[ast.Node]types.Type, []core.Diagnostic) {
	defer core.Scope()()

	a := analyzer{
		path:           path,
		nodeTypes:      make(map[ast.Node]types.Type),
		instantiations: instantiations,
		typeEnv:        typeEnv,
	}

	a.scopes.Push(root)
	a.scopes.Push(a.GetImportsScope(root, file))
	a.scopes.Push(symbols.SymbolScope(fileSymbols))

	for i := range fileSymbols {
		a.ResolveSymbol(&fileSymbols[i])
	}

	for _, decl := range file.Decls {
		if impl, ok := decl.(*ast.Impl); ok {
			a.resolveImpl(impl)
		}
	}

	// Cleanup

	for i := 0; i < 3; i++ {
		a.scopes.Pop()
	}

	a.scopes.ValidateEmpty()

	return a.nodeTypes, a.diagnostics
}

func (a *analyzer) resolveImpl(impl *ast.Impl) {
	// Temporarily push impl type params so the impl type itself can resolve
	if len(impl.TypeParams) > 0 {
		typeParams := make([]*types.Param, 0, len(impl.TypeParams))

		for _, param := range impl.TypeParams {
			typeParams = append(typeParams, &types.Param{Name: param.Name.Token.Text})
		}

		a.resolveTypeParams(impl.TypeParams, typeParams)
		defer a.scopes.Pop()
	}

	typ := a.AnalyzeType(impl.Type)
	okType := true

	if typ != types.Invalid {
		if _, ok := typ.(*types.Primitive); !ok {
			if _, ok := typ.(*types.Enum); !ok {
				if _, ok := typ.(*types.Struct); !ok {
					a.Error(impl.Type, "implementation blocks can only be attached to primitives, enums and structs, not '%s'", typ)
					okType = false
				}
			}
		}
	}

	// Rebuild the scope using the canonical struct's TypeParams
	methodTyp := typ

	if s, ok := typ.(*types.Struct); ok {
		template := s
		if s.Generic != nil {
			template = s.Generic
		}

		if len(template.TypeParams) > 0 && len(impl.TypeParams) > 0 {
			if len(impl.TypeParams) != len(template.TypeParams) {
				a.Error(impl.Type, "implementation of generic struct '%s' must declare %d type parameter(s), got %d", template.Name, len(template.TypeParams), len(impl.TypeParams))
			} else {
				methodTyp = s.Generic

				implNames := make([]string, len(impl.TypeParams))
				for i, param := range impl.TypeParams {
					implNames[i] = param.Name.Token.Text
				}

				a.scopes.Push(&symbols.ParamScope{
					Names:  implNames,
					Params: template.TypeParams,
					Nodes:  impl.TypeParams,
				})

				defer a.scopes.Pop()
			}
		}
	}

	// Interface
	if impl.Interface != nil {
		inRaw := a.AnalyzeType(impl.Interface)
		in, ok := inRaw.(*types.Interface)

		if inRaw != types.Invalid && !ok {
			a.Error(impl.Interface, "'%s' is not an interface", impl.Interface)
		} else if ok {
			if !a.typeEnv.AddConformance(methodTyp, in) {
				a.Error(impl.Type, "type '%s' already implements interface '%s'", typ, in)
			}

			// Associated types
			matchedNodes := make([]*ast.AssociatedType, 0, len(impl.AssociatedTypes))
			aliasTypes := make([]types.Type, 0, len(impl.AssociatedTypes))

			for _, associatedType := range impl.AssociatedTypes {
				i := slices.IndexFunc(in.AssociatedTypes, func(param *types.Param) bool {
					return param.Name == associatedType.Name.Token.Text
				})

				if i == -1 {
					a.Error(associatedType, "interface '%s' does not have an associated type '%s'", in.String(), associatedType.Name.Token.Text)
					continue
				}

				alias := a.AnalyzeType(associatedType.Type)

				matchedNodes = append(matchedNodes, associatedType)
				aliasTypes = append(aliasTypes, alias)
			}

			if len(aliasTypes) < len(in.AssociatedTypes) {
				a.Error(impl.Type, "implementation of interface '%s' for '%s' is missing some associated types", in.String(), typ.String())
			}

			if a.pushAssociatedTypes(matchedNodes, aliasTypes) {
				defer a.scopes.Pop()
			}
		}
	} else {
		// Associated types
		for _, associatedType := range impl.AssociatedTypes {
			a.Error(associatedType, "associated types can only be used with interfaces")
		}
	}

	// Methods
	prevSelf := a.selfType
	a.selfType = methodTyp
	defer func() { a.selfType = prevSelf }()

	for _, f := range impl.Methods {
		a.resolveMethod(f, okType, typ, methodTyp)
	}
}

func (a *analyzer) resolveMethod(f *ast.Func, okType bool, typ, methodTyp types.Type) {
	var funcTypeParams []*types.Param

	if len(f.TypeParams) > 0 {
		funcTypeParams = make([]*types.Param, 0, len(f.TypeParams))

		for _, param := range f.TypeParams {
			funcTypeParams = append(funcTypeParams, &types.Param{Name: param.Name.Token.Text})
		}

		a.resolveTypeParams(f.TypeParams, funcTypeParams)
		defer a.scopes.Pop()
	}

	// Create type
	t := &types.Func{}
	if len(f.TypeParams) > 0 {
		t.TypeParams = funcTypeParams
	}

	t.Params = make([]types.Type, 0, 1+len(f.Params))
	t.VarArgs = f.VarArgs

	if f.Receiver != nil {
		t.Params = append(t.Params, &types.Pointer{Mutable: f.Receiver.Mutable, Pointee: methodTyp})
	}

	for _, param := range f.Params {
		typ := a.AnalyzeType(param.Type)

		if typ == types.PrimitiveVoid {
			typ = types.Invalid
		}

		t.Params = append(t.Params, typ)
	}

	t.Returns = a.AnalyzeType(f.Returns)

	symbol := symbols.Symbol{
		Kind:   symbols.Func,
		Public: f.Public,
		Name:   f.Name().Token.Text,
		Node:   f,
		Type:   t,
	}

	var okAdd bool

	if f.Receiver == nil {
		okAdd = a.typeEnv.AddStaticMethod(methodTyp, symbol)
	} else {
		okAdd = a.typeEnv.AddInstanceMethod(methodTyp, symbol)
	}

	if okType && !okAdd {
		a.Error(f.Name_, "method with the name '%s' already exists on type '%s'", f.Name().Token.Text, typ)
	}
}

func (a *analyzer) pushAssociatedTypes(astAssocTypes []*ast.AssociatedType, aliasTypes []types.Type) bool {
	if len(astAssocTypes) == 0 {
		return false
	}

	syms := make([]symbols.Symbol, 0, len(astAssocTypes))

	for i, assocType := range astAssocTypes {
		if i >= len(aliasTypes) {
			continue
		}

		a.nodeTypes[assocType] = aliasTypes[i]

		syms = append(syms, symbols.Symbol{
			Kind: symbols.TypeParam,
			Name: assocType.Name.Token.Text,
			Node: assocType.Type,
			Type: aliasTypes[i],
		})
	}

	a.scopes.Push(symbols.SymbolScope(syms))

	return true
}
