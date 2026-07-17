package cfg

import (
	"fireball/ast"
	"fireball/core"
)

func (e *Env) Strip(f *ast.File) {
	defer core.Scope()()

	// File

	if !shouldKeep(e, f) {
		end := f.Mod.Range_.End
		if len(f.Decls) > 0 {
			end = f.Decls[len(f.Decls)-1].Range().End
		}

		f.Stripped = []core.Range{
			{
				Start: f.Mod.Range_.Start,
				End:   end,
			},
		}

		f.Imports = nil
		f.Decls = nil
	}

	// Imports

	var imports []*ast.Import

	for _, import_ := range f.Imports {
		if shouldKeep(e, import_) {
			imports = append(imports, import_)
		} else {
			f.Stripped = append(f.Stripped, import_.Range_)
		}
	}

	f.Imports = imports

	// Decls

	var decls []ast.Decl

	for _, decl := range f.Decls {
		if shouldKeep(e, decl) {
			if impl, ok := decl.(*ast.Impl); ok {
				e.stripImpl(f, impl)
			}

			decls = append(decls, decl)
		} else {
			f.Stripped = append(f.Stripped, decl.Range())
		}
	}

	f.Decls = decls
}

func (e *Env) stripImpl(f *ast.File, impl *ast.Impl) {
	var methods []*ast.Func

	for _, method := range impl.Methods {
		if shouldKeep(e, method) {
			methods = append(methods, method)
		} else {
			f.Stripped = append(f.Stripped, method.Range())
		}
	}

	impl.Methods = methods
}

func shouldKeep[N interface{ Attributes() []ast.Attribute }](e *Env, node N) bool {
	if cfg := ast.GetAttribute[*ast.Cfg](node); cfg != nil {
		return e.Visit(cfg.Predicate)
	}

	return true
}
