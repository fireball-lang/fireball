package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
)

// Visitor

func (a *analyzer) VisitStruct(s *ast.Struct) {
	symbol, _ := a.scope.Get(s.Name())
	typ := symbol.Type.(*types.Struct)

	names := make(map[string]any)

	for i, field := range s.Fields {
		name := field.Name.Token.Text
		if name == "" {
			continue
		}

		if _, ok := names[name]; ok {
			a.Error(field.Name, "field with the name '%s' already exists", name)
		}

		if typ.Fields[i].Type == types.PrimitiveVoid {
			a.Error(field.Type, "field cannot be of type 'void'")
		}

		names[name] = nil
	}
}

func (a *analyzer) VisitFunc(f *ast.Func) {
	symbol, _ := a.scope.Get(f.Name())
	typ := symbol.Type.(*types.Func)

	// Body
	a.locals.Push()

	for i, param := range f.Params {
		typ := typ.Params[i]

		if typ == types.PrimitiveVoid {
			a.Error(param.Type, "parameter cannot be of type 'void'")
			typ = types.Invalid
		}

		symbol := symbols.Symbol{
			Kind: symbols.Param,
			Name: param.Name.Token.Text,
			Node: param,
			Type: typ,
		}

		if !a.locals.Add(symbol) {
			a.Error(param.Name, "parameter with the name '%s' already exists", symbol.Name)
		}
	}

	a.funcType = typ
	a.AnalyzeStmt(f.Body)
	a.funcType = nil

	a.locals.Pop()

	// Return
	if !core.IsNil(f.Body) && !typ.Returns.Equals(types.PrimitiveVoid) {
		switch stmt := f.Body.(type) {
		case *ast.Return:

		case *ast.Block:
			if len(stmt.Stmts) == 0 {
				a.Error(f.Name_, "missing return statement")
			} else if _, ok := stmt.Stmts[len(stmt.Stmts)-1].(*ast.Return); !ok {
				a.Error(f.Name_, "missing return statement")
			}

		default:
			a.Error(f.Name_, "missing return statement")
		}
	}
}

func (a *analyzer) VisitBadDecl(_ *ast.BadDecl) {}
