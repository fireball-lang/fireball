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

			modulePath := make([]string, 0, len(file.Mod.Path))
			for _, entry := range file.Mod.Path {
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
				Type:   &types.Struct{Name: sb.String(), ModulePath: modulePath, Layout: decl.GetLayout(), TypeParams: typeParams}, // filled in type resolver
			})

		case *ast.Enum:
			sb := strings.Builder{}

			modulePath := make([]string, 0, len(file.Mod.Path))
			for _, entry := range file.Mod.Path {
				modulePath = append(modulePath, entry.Token.Text)
				sb.WriteString(entry.Token.Text)
				sb.WriteString("::")
			}

			sb.WriteString(decl.Name().Token.Text)

			symbols = append(symbols, Symbol{
				Kind:   Enum,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   &types.Enum{Name: sb.String(), ModulePath: modulePath}, // filled in type resolver
			})

		case *ast.Interface:
			sb := strings.Builder{}

			modulePath := make([]string, 0, len(file.Mod.Path))
			for _, entry := range file.Mod.Path {
				modulePath = append(modulePath, entry.Token.Text)
				sb.WriteString(entry.Token.Text)
				sb.WriteString("::")
			}

			sb.WriteString(decl.Name().Token.Text)

			typeParams := make([]*types.Param, 0, len(decl.TypeParams))

			for _, param := range decl.TypeParams {
				typeParams = append(typeParams, &types.Param{Name: param.Name.Token.Text})
			}

			associatedTypes := make([]*types.Param, 0, len(decl.AssociatedTypes))

			for _, associatedType := range decl.AssociatedTypes {
				associatedTypes = append(associatedTypes, &types.Param{Name: associatedType.Name.Token.Text, Associated: true})
			}

			selfParam := &types.Param{Name: "Self"}

			symbols = append(symbols, Symbol{
				Kind:   Interface,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   &types.Interface{Name: sb.String(), ModulePath: modulePath, TypeParams: typeParams, SelfParam: selfParam, AssociatedTypes: associatedTypes}, // filled in type resolver
			})

		case *ast.GlobalVar:
			symbols = append(symbols, Symbol{
				Kind:   Var,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   nil, // filled in type resolver
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
