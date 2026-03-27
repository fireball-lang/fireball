package sema

import (
	"fireball/ast"
	"fireball/symbols"
	"fireball/types"
)

func (a *analyzer) ResolveSymbol(symbol *symbols.Symbol) {
	switch symbol.Kind {
	case symbols.Struct:
		s := symbol.Node.(*ast.Struct)
		t := symbol.Type.(*types.Struct)

		t.Fields = make([]types.Field, len(s.Fields))

		for i := 0; i < len(s.Fields); i++ {
			typ := a.AnalyzeType(s.Fields[i].Type)

			if typ == types.PrimitiveVoid {
				typ = types.Invalid
			}

			t.Fields[i] = types.Field{
				Name: s.Fields[i].Name.Token.Text,
				Type: typ,
			}
		}

	case symbols.Func:
		f := symbol.Node.(*ast.Func)
		t := symbol.Type.(*types.Func)

		t.Params = make([]types.Type, len(f.Params))
		t.VarArgs = f.VarArgs

		for i := 0; i < len(f.Params); i++ {
			typ := a.AnalyzeType(f.Params[i].Type)

			if typ == types.PrimitiveVoid {
				typ = types.Invalid
			}

			t.Params[i] = typ
		}

		t.Returns = a.AnalyzeType(f.Returns)

	default:
		panic("sema.analyzer.ResolveSymbol() - Invalid symbol kind")
	}
}
