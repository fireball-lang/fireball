package symbols

import (
	"fireball/ast"
	"fireball/types"
	"strings"
)

func Collect(file *ast.File) []Symbol {
	var symbols []Symbol

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Struct:
			sb := strings.Builder{}

			for _, entry := range file.Mod.Path.Entries {
				sb.WriteString(entry.Token.Text)
				sb.WriteString("::")
			}

			sb.WriteString(decl.Name())

			symbols = append(symbols, Symbol{
				Kind: Struct,
				Name: decl.Name(),
				Node: decl,
				Type: &types.Struct{Name: sb.String()}, // filled in type resolver
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
