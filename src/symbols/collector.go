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
		case *ast.TypeAlias:
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
				Kind:   TypeAlias,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   &types.Alias{Name: sb.String(), ModulePath: modulePath, TypeParams: typeParams}, // filled in type resolver
			})

		case *ast.Struct:
			var typ *types.Struct

			if decl.Name().Token.Text == "Interface" && len(file.Mod.Path) == 1 && file.Mod.Path[0].Token.Text == "core" {
				typ = types.InterfaceUnderlying
			} else {
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

				typ = &types.Struct{Name: sb.String(), ModulePath: modulePath, Layout: decl.GetLayout(), TypeParams: typeParams} // filled in type resolver
			}

			symbols = append(symbols, Symbol{
				Kind:   Struct,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   typ,
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

			typeParams := make([]*types.Param, len(decl.TypeParams))

			for i, param := range decl.TypeParams {
				typeParams[i] = &types.Param{Name: param.Name.Token.Text}
			}

			associatedTypes := make([]*types.Param, len(decl.AssociatedTypes))

			for i, associatedType := range decl.AssociatedTypes {
				associatedTypes[i] = &types.Param{Name: associatedType.Name.Token.Text, Associated: true}
			}

			associatedConsts := make([]types.AssociatedConst, len(decl.AssociatedConsts))

			for i, assocConst := range decl.AssociatedConsts {
				associatedConsts[i] = types.AssociatedConst{
					Name: assocConst.Name.Token.Text,
					Type: nil, // filled in resolver
				}
			}

			selfParam := &types.Param{Name: "Self"}

			symbols = append(symbols, Symbol{
				Kind:   Interface,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   &types.Interface{Name: sb.String(), ModulePath: modulePath, TypeParams: typeParams, SelfParam: selfParam, AssociatedTypes: associatedTypes, AssociatedConsts: associatedConsts}, // filled in type resolver
			})

		case *ast.Const:
			symbols = append(symbols, Symbol{
				Kind:   Const,
				Public: decl.Public,
				Name:   decl.Name().Token.Text,
				Node:   decl,
				Type:   nil, // filled in type resolver
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
