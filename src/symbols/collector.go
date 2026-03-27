package symbols

import (
	"fireball/ast"
	"fireball/types"
)

func Collect(decls []ast.Decl) []Symbol {
	var symbols []Symbol

	for _, decl := range decls {
		switch decl := decl.(type) {
		case *ast.Struct:
			symbols = append(symbols, Symbol{
				Kind: Struct,
				Name: decl.Name(),
				Node: decl,
				Type: &types.Struct{Name: decl.Name()}, // filled in type resolver
			})

		case *ast.Func:
			symbols = append(symbols, Symbol{
				Kind: Func,
				Name: decl.Name(),
				Node: decl,
				Type: &types.Func{}, // filled in type resolver
			})
		}
	}

	return symbols
}
