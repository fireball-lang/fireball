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
	symbol, _ := a.scopes.GetSymbol(s.Name().Token.Text)

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

func (a *analyzer) VisitEnum(e *ast.Enum) {
	// Type
	symbol, _ := a.scopes.GetSymbol(e.Name().Token.Text)

	typ := symbol.Type.(*types.Enum)
	a.nodeTypes[e] = typ

	// Duplicate names and values
	names := make(map[string]any)
	values := make(map[core.Integer]any)

	for i, c := range typ.Cases {
		node := e.Cases[i].Value
		if node == nil {
			node = e.Cases[i].Name
		}

		if _, ok := names[c.Name]; ok {
			a.Error(node, "case with name '%s' already exists", c.Name)
		}
		names[c.Name] = nil

		if _, ok := values[c.Value]; ok {
			a.Error(node, "case with value '%s' already exists", c.Value)
		}
		values[c.Value] = nil
	}
}

func (a *analyzer) VisitInterface(i *ast.Interface) {
	// Type
	symbol, _ := a.scopes.GetSymbol(i.Name().Token.Text)

	typ := symbol.Type.(*types.Interface)
	a.nodeTypes[i] = typ

	// Methods
	for _, method := range i.Methods {
		if !core.IsNil(method.Body) {
			a.Error(method.Name_, "interface methods cannot have default implementations")
		}

		if len(method.TypeParams) > 0 {
			a.Error(method.Name_, "interface methods cannot have type parameters")
		}
	}
}

func (a *analyzer) VisitImpl(i *ast.Impl) {
	if prev, ok := a.nodeTypes[i.Type]; ok {
		if s, ok := prev.(*types.Struct); ok {
			template := s
			if s.Generic != nil {
				template = s.Generic
			}

			if len(template.TypeParams) > 0 && len(i.TypeParams) == len(template.TypeParams) {
				implNames := make([]string, len(i.TypeParams))

				for j, param := range i.TypeParams {
					implNames[j] = param.Name.Token.Text
				}

				a.scopes.Push(&symbols.ParamScope{
					Names:  implNames,
					Params: template.TypeParams,
					Nodes:  i.TypeParams,
				})

				defer a.scopes.Pop()
			}
		}
	}

	// Type
	typ := a.ResolveAndAnalyzeType(i.Type)
	if typ == types.Invalid {
		return
	}

	a.nodeTypes[i] = typ
	lookupTyp := typ

	if s, ok := typ.(*types.Struct); ok && s.Generic != nil && len(i.TypeParams) > 0 {
		lookupTyp = s.Generic
	}

	// Self
	prevSelf := a.selfType
	a.selfType = lookupTyp
	defer func() { a.selfType = prevSelf }()

	// Methods
	for _, f := range i.Methods {
		if core.IsNil(f.Body) {
			a.Error(f.Name_, "methods need to have a body")
		}

		var fTyp *types.Func
		var receiverTyp types.Type

		if f.Receiver == nil {
			if sym, ok := a.typeEnv.GetStaticMethod(lookupTyp, f.Name().Token.Text); ok {
				fTyp = sym.Type.(*types.Func)
			}
			receiverTyp = nil
		} else {
			if sym, ok := a.typeEnv.GetInstanceMethod(lookupTyp, f.Name().Token.Text); ok {
				fTyp = sym.Type.(*types.Func)
				receiverTyp = fTyp.Params[0]
			}
		}

		if fTyp == nil {
			panic("sema.analyzer.VisitImpl() - Method typ not found")
		}

		if len(fTyp.TypeParams) > 0 {
			a.scopes.Push(&symbols.ParamScope{
				Params: fTyp.TypeParams,
				Nodes:  f.TypeParams,
			})
		}

		a.nodeTypes[f] = fTyp
		a.VisitFuncInner(f, fTyp, receiverTyp)

		if len(fTyp.TypeParams) > 0 {
			a.scopes.Pop()
		}
	}

	// Interface
	if i.Interface != nil {
		inType, ok := a.nodeTypes[i.Interface]
		if !ok || inType == types.Invalid {
			return
		}

		in := inType.(*types.Interface)

		inGeneric := in.Generic
		if inGeneric == nil {
			inGeneric = in
		}

		var methodSubs []types.Substitution
		methodSubs = append(methodSubs, in.Substitutions...)

		if inGeneric.SelfParam != nil {
			methodSubs = append(methodSubs, types.Substitution{Param: inGeneric.SelfParam, Type: lookupTyp})
		}

		for _, assocParam := range inGeneric.AssociatedTypes {
			for _, implAssoc := range i.AssociatedTypes {
				if implAssoc.Name.Token.Text != assocParam.Name {
					continue
				}

				if alias, ok := a.nodeTypes[implAssoc]; ok {
					methodSubs = append(methodSubs, types.Substitution{Param: assocParam, Type: alias})
				}

				break
			}
		}

		substituteInMethod := func(t *types.Func) *types.Func {
			if len(methodSubs) == 0 {
				return t
			}

			return a.instantiations.Substitute(t, methodSubs).(*types.Func)
		}

		for _, method := range inGeneric.InstanceMethods {
			sym, ok := a.typeEnv.GetInstanceMethod(lookupTyp, method.Name)
			if !ok {
				a.Error(i.Type, "type '%s' does not implement interface '%s': missing instance method '%s'", typ, in, method.Name)
				continue
			}

			concrete := sym.Type.(*types.Func)
			substituted := substituteInMethod(method.Type)

			// Check receiver mutability using the substituted interface method type.
			if len(substituted.Params) > 0 && len(concrete.Params) > 0 {
				inRecv, inOk := substituted.Params[0].(*types.Pointer)
				conRecv, conOk := concrete.Params[0].(*types.Pointer)

				if inOk && conOk && inRecv.Mutable != conRecv.Mutable {
					a.Error(i.Type, "method '%s' receiver mutability does not match interface '%s'", method.Name, in)
				}
			}

			if !instanceSignatureMatches(substituted, concrete) {
				a.Error(i.Type, "method '%s' has wrong signature for interface '%s'", method.Name, in)
			}
		}

		for _, method := range inGeneric.StaticMethods {
			sym, ok := a.typeEnv.GetStaticMethod(lookupTyp, method.Name)
			if !ok {
				a.Error(i.Type, "type '%s' does not implement interface '%s': missing static method '%s'", typ, in, method.Name)
				continue
			}

			concrete := sym.Type.(*types.Func)
			if !substituteInMethod(method.Type).Equals(concrete) {
				a.Error(i.Type, "static method '%s' has wrong signature for interface '%s'", method.Name, in)
			}
		}

		for _, f := range i.Methods {
			var methods []types.Method

			if f.Receiver != nil {
				methods = in.InstanceMethods
			} else {
				methods = in.StaticMethods
			}

			name := f.Name().Token.Text
			found := false

			for _, m := range methods {
				if m.Name == name {
					found = true
					break
				}
			}

			if !found {
				a.Error(f.Name_, "method '%s' is not part of interface '%s'", name, in)
			}
		}

		a.typeEnv.RegisterImplNode(lookupTyp, inGeneric, i)
	}
}

func instanceSignatureMatches(in *types.Func, concrete *types.Func) bool {
	inParams := in.Params
	if len(inParams) > 0 {
		inParams = inParams[1:]
	}

	concreteParams := concrete.Params
	if len(concreteParams) > 0 {
		concreteParams = concreteParams[1:]
	}

	if len(inParams) != len(concreteParams) || in.VarArgs != concrete.VarArgs {
		return false
	}

	for i, p := range inParams {
		if !p.Equals(concreteParams[i]) {
			return false
		}
	}

	return in.Returns.Equals(concrete.Returns)
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
	symbol, _ := a.scopes.GetSymbol(f.Name().Token.Text)

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
	if len(typ.TypeParams) > 0 {
		a.scopes.Push(&symbols.ParamScope{
			Params: typ.TypeParams,
			Nodes:  f.TypeParams,
		})

		defer a.scopes.Pop()
	}

	a.VisitFuncInner(f, typ, nil)
}

func (a *analyzer) VisitFuncInner(f *ast.Func, typ *types.Func, receiverTyp types.Type) {
	// Params
	a.locals.Push()

	paramI := 0

	if !core.IsNil(receiverTyp) {
		symbol := symbols.Symbol{
			Kind:   symbols.Param,
			Public: true,
			Name:   "self",
			Node:   f.Receiver,
			Type:   receiverTyp,
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
			Kind:   symbols.Param,
			Public: true,
			Name:   param.Name.Token.Text,
			Node:   param,
			Type:   typ,
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
