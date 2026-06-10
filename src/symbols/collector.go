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

			modulePath := make([]string, 0, len(file.Mod.Path.Entries))
			for _, entry := range file.Mod.Path.Entries {
				modulePath = append(modulePath, entry.Token.Text)
				sb.WriteString(entry.Token.Text)
				sb.WriteString("::")
			}

			sb.WriteString(decl.Name().Token.Text)

			typeParams := make([]*types.Param, 0, len(decl.TypeParams))

			for _, param := range decl.TypeParams {
				typeParams = append(typeParams, &types.Param{Name: param.Name.Token.Text})
			}

			symbols = append(symbols, Symbol{
				Kind:   Struct,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   &types.Struct{Name: sb.String(), ModulePath: modulePath, TypeParams: typeParams}, // filled in type resolver
			})

		case *ast.Interface:
			sb := strings.Builder{}

			modulePath := make([]string, 0, len(file.Mod.Path.Entries))
			for _, entry := range file.Mod.Path.Entries {
				modulePath = append(modulePath, entry.Token.Text)
				sb.WriteString(entry.Token.Text)
				sb.WriteString("::")
			}

			sb.WriteString(decl.Name().Token.Text)

			typeParams := make([]*types.Param, 0, len(decl.TypeParams))

			for _, param := range decl.TypeParams {
				typeParams = append(typeParams, &types.Param{Name: param.Name.Token.Text})
			}

			selfParam := &types.Param{Name: "Self"}

			symbols = append(symbols, Symbol{
				Kind:   Interface,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   &types.Interface{Name: sb.String(), ModulePath: modulePath, TypeParams: typeParams, SelfParam: selfParam}, // filled in type resolver
			})

		case *ast.Func:
			typeParams := make([]*types.Param, 0, len(decl.TypeParams))

			for _, param := range decl.TypeParams {
				typeParams = append(typeParams, &types.Param{Name: param.Name.Token.Text})
			}

			symbols = append(symbols, Symbol{
				Kind:   Func,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   &types.Func{TypeParams: typeParams}, // filled in type resolver
			})
		}
	}

	return symbols
}
