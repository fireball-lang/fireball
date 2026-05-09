package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
)

func ResolveSymbols(file *ast.File, fileSymbols []symbols.Symbol, instantiations types.InstantiationCache, methodTable symbols.MethodTable, root symbols.Scope, path string) (map[ast.Node]types.Type, []core.Diagnostic) {
	defer core.Scope()()

	a := analyzer{
		path:           path,
		nodeTypes:      make(map[ast.Node]types.Type),
		instantiations: instantiations,
		methodTable:    methodTable,
	}

	a.scope = &symbols.CombinedScope{Scopes: []symbols.Scope{
		root,
		a.GetImportsScope(root, file),
		symbols.SymbolScope(fileSymbols),
	}}

	for i := range fileSymbols {
		a.ResolveSymbol(&fileSymbols[i])
	}

	for _, decl := range file.Decls {
		if impl, ok := decl.(*ast.Impl); ok {
			prevScope := a.scope

			// Temporarily push impl type params so the impl type itself can resolve
			if len(impl.TypeParams) > 0 {
				typeParams := make([]*types.Param, 0, len(impl.TypeParams))

				for _, param := range impl.TypeParams {
					typeParams = append(typeParams, &types.Param{Name: param.Token.Text})
				}

				a.scope = &symbols.ParamScope{
					Parent: a.scope,
					Params: typeParams,
					Nodes:  impl.TypeParams,
				}
			}

			typ := a.AnalyzeType(impl.Type)
			_, ok := typ.(*types.Struct)

			if typ != types.Invalid && !ok {
				a.Error(impl.Type, "implementation blocks can only be attached to structs, not '%s'", typ)
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
						for i, leaf := range impl.TypeParams {
							implNames[i] = leaf.Token.Text
						}

						a.scope = &symbols.ParamScope{
							Parent: prevScope,
							Names:  implNames,
							Params: template.TypeParams,
							Nodes:  impl.TypeParams,
						}
					}
				}
			}

			for _, f := range impl.Functions {
				prevFuncScope := a.scope
				var funcTypeParams []*types.Param

				if len(f.TypeParams) > 0 {
					funcTypeParams = make([]*types.Param, 0, len(f.TypeParams))

					for _, param := range f.TypeParams {
						funcTypeParams = append(funcTypeParams, &types.Param{Name: param.Token.Text})
					}

					a.scope = &symbols.ParamScope{
						Parent: a.scope,
						Params: funcTypeParams,
						Nodes:  f.TypeParams,
					}
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

				var okAdd bool

				if f.Receiver == nil {
					okAdd = methodTable.AddStatic(methodTyp, f, t)
				} else {
					okAdd = methodTable.Add(methodTyp, f, t)
				}

				if ok && !okAdd {
					a.Error(f.Name_, "method with the name '%s' already exists on type '%s'", f.Name().Token.Text, typ)
				}

				a.scope = prevFuncScope
			}

			a.scope = prevScope
		}
	}

	return a.nodeTypes, a.diagnostics
}
