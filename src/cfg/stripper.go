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

	f.Imports = getStrippedSlice(e, f, f.Imports)

	// Decls

	var decls []ast.Decl

	for _, decl := range f.Decls {
		if shouldKeep(e, decl) {
			switch decl := decl.(type) {
			case *ast.Struct:
				decl.Fields = getStrippedSlice(e, f, decl.Fields)

			case *ast.Interface:
				decl.AssociatedTypes = getStrippedSlice(e, f, decl.AssociatedTypes)
				decl.Methods = getStrippedSlice(e, f, decl.Methods)

			case *ast.Impl:
				decl.AssociatedTypes = getStrippedSlice(e, f, decl.AssociatedTypes)
				decl.Methods = getStrippedSlice(e, f, decl.Methods)
			}

			decls = append(decls, decl)
		} else {
			f.Stripped = append(f.Stripped, decl.Range())
		}
	}

	f.Decls = decls
}

func getStrippedSlice[T ast.AttributeHolder](e *Env, f *ast.File, original []T) []T {
	var nodes []T

	for _, node := range original {
		if shouldKeep(e, node) {
			nodes = append(nodes, node)
		} else {
			f.Stripped = append(f.Stripped, node.Range())
		}
	}

	return nodes
}

func shouldKeep[T ast.AttributeHolder](e *Env, node T) bool {
	if cfg := ast.GetAttribute[*ast.Cfg](node); cfg != nil {
		return e.Visit(cfg.Predicate)
	}

	return true
}
