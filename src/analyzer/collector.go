package analyzer

import "fireball/ast"

func CollectTypeDecls(f *ast.File) []ast.Decl {
	var decls []ast.Decl

	for _, decl := range f.Decls {
		if decl.Name() == "" {
			continue
		}

		switch decl.(type) {
		case *ast.Struct:
			decls = append(decls, decl)
		}
	}

	return decls
}
