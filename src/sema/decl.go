package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
)

// Visitor

func (a *analyzer) VisitStruct(s *ast.Struct) {
	// Attributes
	attributes := make(map[string]any)

	for _, attribute := range s.Attributes {
		name := attribute.Name.Token.Text
		if name == "" {
			continue
		}

		if _, ok := attributes[name]; ok {
			a.Error(attribute.Name, "attribute with the name '%s' already exists", name)
			continue
		}
		attributes[name] = nil

		switch name {
		default:
			a.Error(attribute.Name, "unknown struct attribute '%s'", name)
		}
	}

	// Type
	symbol, _ := a.scope.GetSymbol(s.Name().Token.Text)

	typ := symbol.Type.(*types.Struct)
	a.nodeTypes[s] = typ

	// Fields
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

func (a *analyzer) VisitImpl(i *ast.Impl) {
	prevScope := a.scope

	if prev, ok := a.nodeTypes[i.Type]; ok {
		if s, ok := prev.(*types.Struct); ok {
			template := s
			if s.Generic != nil {
				template = s.Generic
			}

			if len(template.TypeParams) > 0 && len(i.TypeParams) == len(template.TypeParams) {
				implNames := make([]string, len(i.TypeParams))

				for j, leaf := range i.TypeParams {
					implNames[j] = leaf.Token.Text
				}

				a.scope = &symbols.ParamScope{
					Parent: a.scope,
					Names:  implNames,
					Params: template.TypeParams,
					Nodes:  i.TypeParams,
				}
			}
		}
	}

	// Type
	typ := a.AnalyzeType(i.Type)
	if typ == types.Invalid {
		a.scope = prevScope
		return
	}

	a.nodeTypes[i] = typ

	if _, ok := typ.(*types.Struct); !ok {
		a.Error(i.Type, "implementation blocks can only be attached to struct types, not '%s'", typ)
	}

	lookupTyp := typ

	if s, ok := typ.(*types.Struct); ok && s.Generic != nil && len(i.TypeParams) > 0 {
		lookupTyp = s.Generic
	}

	// Methods
	for _, f := range i.Functions {
		if core.IsNil(f.Body) {
			a.Error(f.Name_, "methods need to have a body")
		}

		prevFuncScope := a.scope

		if len(f.TypeParams) > 0 {
			funcTypeParams := make([]*types.Param, 0, len(f.TypeParams))

			for _, param := range f.TypeParams {
				funcTypeParams = append(funcTypeParams, &types.Param{Name: param.Token.Text})
			}

			a.scope = &symbols.ParamScope{
				Parent: a.scope,
				Params: funcTypeParams,
				Nodes:  f.TypeParams,
			}
		}

		var fTyp *types.Func
		var receiverTyp types.Type

		if f.Receiver == nil {
			_, fTyp = a.methodTable.GetStatic(lookupTyp, f.Name().Token.Text)
			receiverTyp = nil
		} else {
			_, fTyp = a.methodTable.Get(lookupTyp, f.Name().Token.Text)
			if fTyp != nil {
				receiverTyp = fTyp.Params[0]
			}
		}

		a.nodeTypes[f] = fTyp
		a.VisitFuncInner(f, fTyp, receiverTyp)

		a.scope = prevFuncScope
	}

	a.scope = prevScope
}

func (a *analyzer) VisitFunc(f *ast.Func) {
	// Attributes
	attributes := make(map[string]any)

	test := false
	extern := false

	for _, attribute := range f.Attributes {
		name := attribute.Name.Token.Text
		if name == "" {
			continue
		}

		if _, ok := attributes[name]; ok {
			a.Error(attribute.Name, "attribute with the name '%s' already exists", name)
			continue
		}
		attributes[name] = nil

		switch name {
		case "test":
			test = true

			if len(attribute.Arguments) > 1 {
				a.Error(attribute.Name, "too many attribute arguments for 'test' attribute")
			}

			if len(attribute.Arguments) == 1 {
				arg := attribute.Arguments[0]

				if _, ok := arg.(*ast.String); !ok {
					if _, ok := arg.(*ast.BadExpr); !ok {
						a.Error(arg, "expected a string")
					}
				}
			}

		case "extern":
			extern = true

			if len(attribute.Arguments) > 0 {
				a.Error(attribute.Name, "too many attribute arguments for 'extern' attribute")
			}

		case "link_name":
			if len(attribute.Arguments) != 1 {
				a.Error(attribute.Name, "'link_name' attribute needs 1 argument, got %d", len(attribute.Arguments))
			}

			if len(attribute.Arguments) == 1 {
				arg := attribute.Arguments[0]

				if _, ok := arg.(*ast.String); !ok {
					if _, ok := arg.(*ast.BadExpr); !ok {
						a.Error(arg, "expected a string")
					}
				}
			}

		default:
			a.Error(attribute.Name, "unknown function attribute '%s'", name)
		}
	}

	// Type
	symbol, _ := a.scope.GetSymbol(f.Name().Token.Text)

	typ := symbol.Type.(*types.Func)
	a.nodeTypes[f] = typ

	// Test
	if test {
		if len(f.Params) != 0 || f.VarArgs {
			a.Error(f.Name_, "test functions cannot have any parameters")
		}

		if typ.Returns != types.PrimitiveBool {
			var node ast.Node = f.Returns
			if node.Range().Start == node.Range().End {
				node = f.Name_
			}

			a.Error(node, "test functions need to return a boolean")
		}

		if core.IsNil(f.Body) {
			a.Error(f.Name_, "test functions need to have a body")
		}
	}

	// Body
	if extern {
		if !core.IsNil(f.Body) {
			a.Error(f.Body, "extern functions cannot have a body")
		}
	} else {
		if core.IsNil(f.Body) {
			a.Error(f.Name_, "non-extern functions need to have a body")
		}
	}

	// Inner
	prevScope := a.scope
	if len(typ.TypeParams) > 0 {
		a.scope = &symbols.ParamScope{Parent: a.scope, Params: typ.TypeParams, Nodes: f.TypeParams}
	}

	a.VisitFuncInner(f, typ, nil)

	a.scope = prevScope
}

func (a *analyzer) VisitFuncInner(f *ast.Func, typ *types.Func, receiverTyp types.Type) {
	// Params
	a.locals.Push()

	paramI := 0

	if !core.IsNil(receiverTyp) {
		symbol := symbols.Symbol{
			Kind: symbols.Param,
			Name: "self",
			Node: f.Receiver,
			Type: receiverTyp,
		}

		if !a.locals.Add(symbol) {
			panic("analyzer.analyzer.VisitFuncInner() - Failed to add 'self' receiver to locals")
		}

		paramI++
	}

	for _, param := range f.Params {
		typ := typ.Params[paramI]
		paramI++

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

	// Body
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
