package symbols

import (
	"fireball/ast"
	"fireball/types"
)

func Collect(decls []ast.Decl) SimpleScope {
	var symbols []Symbol

	for _, decl := range decls {
		switch decl := decl.(type) {
		case *ast.Struct:
			symbols = append(symbols, Symbol{
				Kind: Struct,
				Decl: decl,
				Type: &types.Struct{}, // filled in type resolver
			})

		case *ast.Func:
			symbols = append(symbols, Symbol{
				Kind: Func,
				Decl: decl,
				Type: &types.Func{}, // filled in type resolver
			})
		}
	}

	return symbols
}
