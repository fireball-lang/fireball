package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
)

func ResolveSymbols(file *ast.File, fileSymbols []symbols.Symbol, methodTable symbols.MethodTable, root symbols.Scope, path string) []core.Diagnostic {
	a := analyzer{
		path: path,
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
			typ := a.AnalyzeType(impl.Type)
			_, ok := typ.(*types.Struct)

			if typ != types.Invalid && !ok {
				a.Error(impl.Type, "implementation blocks can only be attached to structs, not '%s'", typ)
			}

			for _, f := range impl.Functions {
				// Create type
				t := &types.Func{}

				t.Params = make([]types.Type, 0, 1+len(f.Params))
				t.VarArgs = f.VarArgs

				if f.Receiver != nil {
					t.Params = append(t.Params, &types.Pointer{Pointee: typ})
				}

				for _, param := range f.Params {
					typ := a.AnalyzeType(param.Type)

					if typ == types.PrimitiveVoid {
						typ = types.Invalid
					}

					t.Params = append(t.Params, typ)
				}

				t.Returns = a.AnalyzeType(f.Returns)

				// Add method
				var okAdd bool

				if f.Receiver == nil {
					okAdd = methodTable.AddStatic(typ, f, t)
				} else {
					okAdd = methodTable.Add(typ, f, t)
				}

				if ok && !okAdd {
					a.Error(f.Name_, "method with the name '%s' already exists on type '%s'", f.Name(), typ)
				}
			}
		}
	}

	return a.diagnostics
}
