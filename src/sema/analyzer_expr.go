package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"math"
	"slices"
	"strings"
)

// Visitor

func (a *analyzer) VisitBool(_ *ast.Bool) ExprInfo {
	return ExprInfo{Type: types.PrimitiveBool}
}

func (a *analyzer) VisitNumber(n *ast.Number) ExprInfo {
	switch n.Token.Kind {
	case lexer.BinaryInteger, lexer.HexInteger, lexer.UnsignedInteger:
		value := lexer.ParseInteger(n.Token).Raw()

		if value <= math.MaxUint8 {
			return ExprInfo{Type: types.PrimitiveU8}
		} else if value <= math.MaxUint16 {
			return ExprInfo{Type: types.PrimitiveU16}
		} else if value <= math.MaxUint32 {
			return ExprInfo{Type: types.PrimitiveU32}
		}
		return ExprInfo{Type: types.PrimitiveU64}

	case lexer.SignedInteger:
		value := lexer.ParseInteger(n.Token).Raw()

		if value <= math.MaxInt8 {
			return ExprInfo{Type: types.PrimitiveI8}
		} else if value <= math.MaxInt16 {
			return ExprInfo{Type: types.PrimitiveI16}
		} else if value <= math.MaxInt32 {
			return ExprInfo{Type: types.PrimitiveI32}
		}
		return ExprInfo{Type: types.PrimitiveI64}

	case lexer.Decimal:
		return ExprInfo{Type: types.PrimitiveF64}

	case lexer.Decimal32bit:
		return ExprInfo{Type: types.PrimitiveF32}

	default:
		panic("sema.analyzer.VisitNumber() - Invalid token kind")
	}
}

func (a *analyzer) VisitCharacter(c *ast.Character) ExprInfo {
	v := uint64(c.Rune)
	var typ *types.Primitive

	if v <= math.MaxUint8 {
		typ = types.PrimitiveU8
	} else if v <= math.MaxUint16 {
		typ = types.PrimitiveU16
	} else {
		typ = types.PrimitiveU32
	}

	return ExprInfo{Type: typ}
}

func (a *analyzer) VisitString(_ *ast.String) ExprInfo {
	return ExprInfo{Type: a.stringViewType}
}

var voidPtrType = &types.Pointer{Pointee: types.PrimitiveVoid}

func (a *analyzer) VisitNull(_ *ast.Null) ExprInfo {
	return ExprInfo{Type: voidPtrType}
}

func (a *analyzer) VisitStructInitializer(s *ast.StructInitializer) ExprInfo {
	typ := a.ResolveAndAnalyzeType(s.Type)
	if typ == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	t, ok := typ.(*types.Struct)
	if !ok {
		return a.Error(s.Type, "type '%s' is not a struct", typ)
	}

	for _, field := range s.Fields {
		f, i := t.Field(field.Name.Token.Text)

		if i == -1 {
			a.Error(field.Name, "field '%s' doesn't exist on struct '%s'", field.Name.Token.Text, t)
			continue
		}

		value := a.AnalyzeExpr(field.Value)
		a.ExpectType(f.Type, value, field.Value)
	}

	return ExprInfo{Type: typ}
}

func (a *analyzer) VisitArrayInitializer(ai *ast.ArrayInitializer) ExprInfo {
	typ := a.ResolveAndAnalyzeType(ai.Type)
	if typ == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	t := typ.(*types.Array)

	if t.Size != uint32(len(ai.Elements)) {
		a.Error(ai.Type, "mismatched array size, type has size of %d but got %d elements", t.Size, len(ai.Elements))
	}

	for _, element := range ai.Elements {
		expr := a.AnalyzeExpr(element)
		a.ExpectType(t.Element, expr, element)
	}

	return ExprInfo{Type: t}
}

func (a *analyzer) VisitSizeOf(s *ast.SizeOf) ExprInfo {
	typ := a.ResolveAndAnalyzeType(s.Type)
	if typ == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	return ExprInfo{Type: types.PrimitiveU32}
}

func (a *analyzer) VisitAlignOf(e *ast.AlignOf) ExprInfo {
	typ := a.ResolveAndAnalyzeType(e.Type)
	if typ == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	return ExprInfo{Type: types.PrimitiveU32}
}

func (a *analyzer) VisitOffsetOf(o *ast.OffsetOf) ExprInfo {
	typ := a.ResolveAndAnalyzeType(o.Type)
	if typ == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	if s, ok := typ.(*types.Struct); ok {
		if _, index := s.Field(o.Field.Token.Text); index == -1 {
			a.Error(o.Field, "field '%s' doesn't exist on '%s'", o.Field.Token.Text, s)
		}
	} else {
		a.Error(o.Type, "expected a struct type, not '%s'", typ)
	}

	return ExprInfo{Type: types.PrimitiveU32}
}

func (a *analyzer) VisitPrefix(p *ast.Prefix) ExprInfo {
	expr := a.AnalyzeExpr(p.Expr)
	if expr.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	switch p.Op {
	case ast.Negate:
		return ExprInfo{Type: a.ExpectPrimitiveClass(types.IsSigned, "signed numeric", expr, p.Expr)}

	case ast.Not:
		a.ExpectType(types.PrimitiveBool, expr, p.Expr)
		return ExprInfo{Type: types.PrimitiveBool}

	case ast.IncrementE:
		if !expr.Address {
			return a.Error(p.Expr, "cannot increment a temporary expression")
		}
		if !expr.Mutable {
			return a.Error(p.Expr, "cannot increment an immutable value")
		}

		return ExprInfo{Type: a.ExpectPrimitiveClass(types.IsNumeric, "numeric", expr, p.Expr)}

	case ast.DecrementE:
		if !expr.Address {
			return a.Error(p.Expr, "cannot decrement a temporary expression")
		}
		if !expr.Mutable {
			return a.Error(p.Expr, "cannot decrement an immutable value")
		}

		return ExprInfo{Type: a.ExpectPrimitiveClass(types.IsNumeric, "numeric", expr, p.Expr)}

	case ast.AddressOf:
		if !expr.Address {
			return a.Error(p.Expr, "cannot take address of a temporary expression")
		}

		return ExprInfo{Type: &types.Pointer{Mutable: expr.Mutable, Pointee: expr.Type}}

	case ast.Dereference:
		if p, ok := expr.Type.(*types.Pointer); ok {
			return ExprInfo{
				Type:    p.Pointee,
				Mutable: p.Mutable,
				Address: true,
			}
		}

		return a.Error(p.Expr, "can only dereference pointers, not '%s'", expr.Type)

	default:
		panic("sema.analyzer.VisitPrefix() - Invalid operator kind")
	}
}

func (a *analyzer) VisitPostfix(p *ast.Postfix) ExprInfo {
	expr := a.AnalyzeExpr(p.Expr)
	if expr.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	switch p.Op {
	case ast.IncrementO:
		if !expr.Address {
			return a.Error(p.Expr, "cannot increment a temporary expression")
		}
		if !expr.Mutable {
			return a.Error(p.Expr, "cannot increment an immutable value")
		}

		return ExprInfo{Type: a.ExpectPrimitiveClass(types.IsNumeric, "numeric", expr, p.Expr)}

	case ast.DecrementO:
		if !expr.Address {
			return a.Error(p.Expr, "cannot decrement a temporary expression")
		}
		if !expr.Mutable {
			return a.Error(p.Expr, "cannot decrement an immutable value")
		}

		return ExprInfo{Type: a.ExpectPrimitiveClass(types.IsNumeric, "numeric", expr, p.Expr)}

	default:
		panic("sema.analyzer.VisitPostfix() - Invalid operator kind")
	}
}

func (a *analyzer) VisitBinary(b *ast.Binary) ExprInfo {
	left := a.AnalyzeExpr(b.Left)
	right := a.AnalyzeExpr(b.Right)

	if left.Invalid() || right.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	// Compound assignment
	if b.Op.IsCompoundAssign() {
		if !left.Address {
			return a.Error(b.Left, "cannot assign to a non-addressable expression")
		}
		if !left.Mutable {
			return a.Error(b.Left, "cannot assign to an immutable value")
		}

		op := b.Op.CompoundAssignBase()
		right := a.AnalyzeBaseBinaryOp(b, left, right, op)

		a.ExpectType(left.Type, right, b.Right)

		return ExprInfo{Type: left.Type}
	}

	// Assignment
	if b.Op == ast.Assign {
		if !left.Address {
			return a.Error(b.Left, "cannot assign to a non-addressable expression")
		}
		if !left.Mutable {
			return a.Error(b.Left, "cannot assign to an immutable value")
		}

		a.ExpectType(left.Type, right, b.Right)

		return ExprInfo{Type: left.Type}
	}

	// Base
	return a.AnalyzeBaseBinaryOp(b, left, right, b.Op)
}

func (a *analyzer) AnalyzeBaseBinaryOp(b *ast.Binary, left, right ExprInfo, op ast.BinaryOp) ExprInfo {
	// Math
	if op.IsMath() {
		left := a.ExpectPrimitiveClass(types.IsNumeric, "numeric", left, b.Left)
		right := a.ExpectPrimitiveClass(types.IsNumeric, "numeric", right, b.Right)

		if left == types.Invalid || right == types.Invalid {
			return ExprInfo{Type: types.Invalid}
		}

		typ := CommonType(left, right)
		if typ == nil {
			return a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left, right)
		}

		return ExprInfo{Type: typ}
	}

	// Bitwise
	if op.IsBitwise() {
		left := a.ExpectPrimitiveClass(types.IsInteger, "integer", left, b.Left)
		right := a.ExpectPrimitiveClass(types.IsInteger, "integer", right, b.Right)

		if left == types.Invalid || right == types.Invalid {
			return ExprInfo{Type: types.Invalid}
		}

		typ := CommonType(left, right)
		if typ == nil {
			return a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left, right)
		}

		return ExprInfo{Type: typ}
	}

	// Boolean
	if op.IsBoolean() {
		a.ExpectType(types.PrimitiveBool, left, b.Left)
		a.ExpectType(types.PrimitiveBool, right, b.Right)

		return ExprInfo{Type: types.PrimitiveBool}
	}

	// Equality
	if op.IsEquality() {
		if common := CommonType(left.Type, right.Type); common == nil && !left.Type.Equals(right.Type) {
			return a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left.Type, right.Type)
		}

		switch left.Type.(type) {
		case *types.Primitive, *types.Pointer, *types.Func, *types.Enum:
		default:
			return a.Error(b, "equality operators only work on primitive types, pointers, function pointers or enums, not %s", left.Type)
		}

		return ExprInfo{Type: types.PrimitiveBool}
	}

	// Relational
	if op.IsRelational() {
		left := a.ExpectPrimitiveClass(types.IsNumeric, "numeric", left, b.Left)
		right := a.ExpectPrimitiveClass(types.IsNumeric, "numeric", right, b.Right)

		if left == types.Invalid || right == types.Invalid {
			return ExprInfo{Type: types.Invalid}
		}

		if common := CommonType(left, right); common == nil {
			return a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left, right)
		}

		return ExprInfo{Type: types.PrimitiveBool}
	}

	panic("sema.analyzer.AnalyzeBaseBinaryOp() - Invalid base operator")
}

func (a *analyzer) VisitIdentifier(i *ast.Identifier) ExprInfo {
	symbol, ok := a.GetSymbol(i.Path)
	if !ok {
		return ExprInfo{Type: types.Invalid}
	}

	a.nodeTypes[symbol.Node] = symbol.Type

	switch symbol.Kind {
	case symbols.Case:
		return ExprInfo{
			Type:    symbol.Type,
			Node:    symbol.Node,
			Symbol:  symbol.Kind,
			Mutable: false,
			Address: false,
		}

	case symbols.Param, symbols.Var:
		return ExprInfo{
			Type:    symbol.Type,
			Node:    symbol.Node,
			Symbol:  symbol.Kind,
			Mutable: true,
			Address: true,
		}

	case symbols.Func:
		return ExprInfo{
			Type:   symbol.Type,
			Node:   symbol.Node,
			Symbol: symbol.Kind,
		}

	case symbols.Struct, symbols.Enum, symbols.Interface:
		return a.Error(i, "symbol '%s' is a type and cannot be used as an expression", i.Path.LastName())

	default:
		panic("sema.analyzer.VisitIdentifier() - Invalid symbol kind")
	}
}

func (a *analyzer) VisitIndex(i *ast.Index) ExprInfo {
	// Index
	index := a.AnalyzeExpr(i.Index)
	a.ExpectPrimitiveClass(types.IsInteger, "integer", index, i.Index)

	// Expression
	expr := a.AnalyzeExpr(i.Expr)
	if expr.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	if p, ok := expr.Type.(*types.Pointer); ok {
		return ExprInfo{
			Type:    p.Pointee,
			Mutable: p.Mutable,
			Address: true,
		}
	}

	if t, ok := expr.Type.(*types.Array); ok {
		return ExprInfo{
			Type:    t.Element,
			Mutable: true,
			Address: expr.Address,
		}
	}

	return a.Error(i.Expr, "expected an array or a pointer, got '%s'", expr.Type)
}

func (a *analyzer) VisitMember(m *ast.Member) ExprInfo {
	expr := a.AnalyzeExpr(m.Expr)
	if expr.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	typ := expr.Type
	address := expr.Address
	mutable := expr.Mutable

	// Interface
	if t, ok := typ.(*types.Interface); ok {
		for _, method := range t.InstanceMethods {
			if method.Name == m.Name.Token.Text {
				in := a.typeEnv.GetInterfaceNode(t)
				if in == nil {
					return ExprInfo{Type: types.Invalid}
				}

				var f *ast.Func

				for _, m := range in.Methods {
					if m.Name().Token.Text == method.Name {
						f = m
						break
					}
				}

				if f == nil {
					panic("sema.analyzer.VisitMember() - Failed to find interface method node")
				}

				a.nodeTypes[f] = method.Type
				return ExprInfo{Type: method.Type, Node: f}
			}
		}

		return a.Error(m.Name, "method '%s' doesn't exist on interface '%s'", m.Name.Token.Text, t)
	}

	// Constrained type parameter
	if t, ok := typ.(*types.Param); ok {
		if len(t.Constraints) == 0 {
			return a.Error(m.Expr, "cannot call method '%s' on unconstrained type parameter '%s'", m.Name.Token.Text, t.Name)
		}

		for _, constraint := range t.Constraints {
			for _, method := range constraint.InstanceMethods {
				if method.Name != m.Name.Token.Text {
					continue
				}

				in := a.typeEnv.GetInterfaceNode(constraint)
				if in == nil {
					return ExprInfo{Type: types.Invalid}
				}

				var f *ast.Func

				for _, mf := range in.Methods {
					if mf.Name().Token.Text == method.Name {
						f = mf
						break
					}
				}

				if f == nil {
					panic("sema.analyzer.VisitMember() - missing interface method AST node")
				}

				methodType := method.Type
				subs := []types.Substitution{{Param: constraint.SelfParam, Type: t}}
				methodType = a.instantiations.Substitute(methodType, subs).(*types.Func)

				a.nodeTypes[f] = methodType
				return ExprInfo{Type: methodType, Node: f}
			}
		}

		// Build constraint list for error message
		sb := strings.Builder{}

		for i, c := range t.Constraints {
			if i > 0 {
				sb.WriteString(" + ")
			}
			sb.WriteString(c.String())
		}

		return a.Error(m.Name, "method '%s' does not exist on constraint '%s'", m.Name.Token.Text, sb.String())
	}

	// Pointer
	if p, ok := typ.(*types.Pointer); ok {
		address = true
		mutable = p.Mutable
		typ = p.Pointee
	}

	// Struct
	if t, ok := typ.(*types.Struct); ok {
		if field, index := t.Field(m.Name.Token.Text); index != -1 {
			_, fieldIsFunc := field.Type.(*types.Func)

			if !a.WantsFunction(m) || fieldIsFunc {
				if a.checkVisibility && !field.Public && !slices.Equal(t.ModulePath, a.fileModPath) {
					a.Error(m.Name, "field '%s' is private", m.Name.Token.Text)
				}

				var fieldNode ast.Node
				lookupStruct := t

				if t.Generic != nil {
					lookupStruct = t.Generic
				}

				if structNode := a.typeEnv.GetStructNode(lookupStruct); structNode != nil {
					for _, f := range structNode.Fields {
						if f.Name.Token.Text == m.Name.Token.Text {
							fieldNode = f
							break
						}
					}
				}

				return ExprInfo{
					Type:    field.Type,
					Node:    fieldNode,
					Mutable: mutable,
					Address: address,
				}
			}
		}

		// Instance method
		if sym, ok := a.typeEnv.GetInstanceMethod(t, m.Name.Token.Text); ok {
			if a.checkVisibility && !sym.Public && !slices.Equal(t.ModulePath, a.fileModPath) {
				a.Error(m.Name, "method '%s' is private", m.Name.Token.Text)
			}

			methodType := sym.Type
			if t.Generic != nil {
				methodType = a.instantiations.Get(methodType, t.Substitutions).(*types.Func)
			}

			a.nodeTypes[sym.Node] = methodType
			return ExprInfo{Type: methodType, Node: sym.Node}
		}

		// Static method
		if t.Generic != nil {
			if sym, ok := a.typeEnv.GetInstanceMethod(t.Generic, m.Name.Token.Text); ok {
				if a.checkVisibility && !sym.Public && !slices.Equal(t.Generic.ModulePath, a.fileModPath) {
					a.Error(m.Name, "method '%s' is private", m.Name.Token.Text)
				}

				methodType := sym.Type
				if t.Generic != nil {
					methodType = a.instantiations.Get(methodType, t.Substitutions).(*types.Func)
				}

				a.nodeTypes[sym.Node] = methodType
				return ExprInfo{Type: methodType, Node: sym.Node}
			}
		}

		return a.Error(m.Name, "member '%s' doesn't exist on type '%s'", m.Name.Token.Text, t)
	}

	if t, ok := typ.(*types.Enum); ok {
		// Instance method
		if sym, ok := a.typeEnv.GetInstanceMethod(t, m.Name.Token.Text); ok {
			if a.checkVisibility && !sym.Public && !slices.Equal(t.ModulePath, a.fileModPath) {
				a.Error(m.Name, "method '%s' is private", m.Name.Token.Text)
			}

			a.nodeTypes[sym.Node] = sym.Type
			return ExprInfo{Type: sym.Type, Node: sym.Node}
		}

		return a.Error(m.Name, "member '%s' doesn't exist on type '%s'", m.Name.Token.Text, t)
	}

	return a.Error(m.Expr, "expected a struct, enum or a pointer to a struct, got '%s'", expr.Type)
}

func (a *analyzer) VisitCall(c *ast.Call) ExprInfo {
	expr := a.AnalyzeExpr(c.Callee)
	if expr.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	if f, ok := expr.Type.(*types.Func); ok {
		params := f.Params

		if funcNode, ok := expr.Node.(*ast.Func); ok {
			// Build substitutions
			var subs []types.Substitution

			// Generic receiver
			if funcNode.IsMethod() {
				if member, ok := c.Callee.(*ast.Member); ok {
					receiverType := a.exprInfos[member.Expr].Type
					if p, ok := receiverType.(*types.Pointer); ok {
						receiverType = p.Pointee
					}

					implNode, isImplMethod := funcNode.Parent().(*ast.Impl)
					if s, ok := receiverType.(*types.Struct); ok && s.Generic != nil &&
						isImplMethod && len(implNode.TypeParams) > 0 {
						subs = append(subs, s.Substitutions...)
					}
				}
			}

			// Generic method
			if len(f.TypeParams) > 0 {
				if len(c.TypeArgs) == 0 {
					a.Error(c.Callee, "generic function '%s' requires explicit type arguments", funcNode.Name().Token.Text)
					return ExprInfo{Type: types.Invalid}
				}

				if len(c.TypeArgs) != len(f.TypeParams) {
					a.ErrorRange(ast.SliceRange(c.TypeArgs), "expected %d type argument(s), got %d", len(f.TypeParams), len(c.TypeArgs))
					return ExprInfo{Type: types.Invalid}
				}

				funcSubs := make([]types.Substitution, len(c.TypeArgs))

				for i, typeArg := range c.TypeArgs {
					argType := a.ResolveAndAnalyzeType(typeArg)
					if argType == types.Invalid {
						return ExprInfo{Type: types.Invalid}
					}

					funcSubs[i] = types.Substitution{Param: f.TypeParams[i], Type: argType}
				}

				allSubs := append(subs, funcSubs...)

				for i, typeArg := range c.TypeArgs {
					param := f.TypeParams[i]

					for _, constraint := range param.Constraints {
						if in, ok := a.instantiations.Substitute(constraint, allSubs).(*types.Interface); ok {
							a.CheckConstraint(funcSubs[i].Type, in, typeArg)
						}
					}
				}

				subs = allSubs
			} else if len(c.TypeArgs) > 0 {
				a.Error(c.Callee, "function '%s' is not generic", funcNode.Name().Token.Text)
			}

			// Instantiate
			if len(subs) > 0 {
				f = a.instantiations.Get(f, subs).(*types.Func)
				a.nodeTypes[funcNode] = f
				a.nodeTypes[c] = f
			}

			params = f.Params

			if funcNode.Receiver != nil {
				params = params[1:]

				if funcNode.Receiver.Mutable {
					if member, ok := c.Callee.(*ast.Member); ok {
						if p, ok := a.exprInfos[member.Expr].Type.(*types.Pointer); ok && !p.Mutable {
							a.Error(member.Expr, "cannot call mutable method '%s' on an immutable pointer", funcNode.Name().Token.Text)
						}
						if in, ok := a.exprInfos[member.Expr].Type.(*types.Interface); ok && !in.Mutable {
							a.Error(member.Expr, "cannot call mutable method '%s' on an immutable interface", funcNode.Name().Token.Text)
						}
						if tp, ok := a.exprInfos[member.Expr].Type.(*types.Param); ok {
							// Find which constraint owns this method and check its mutability
							if parentIface, ok := funcNode.Parent().(*ast.Interface); ok {
								for _, constraint := range tp.Constraints {
									if a.typeEnv.GetInterfaceNode(constraint) == parentIface && !constraint.Mutable {
										a.Error(member.Expr, "cannot call mutable method '%s' on type parameter '%s' with immutable constraint '%s'", funcNode.Name().Token.Text, tp.Name, constraint)
										break
									}
								}
							}
						}
					}
				}
			}
		}

		// Return value
		if p, ok := f.Returns.(*types.Param); ok && p.Associated {
			return a.Error(c.Callee, "cannot call a function which return type is not fully defined")
		}

		// Arguments
		if len(c.Args) != len(params) && (!f.VarArgs || len(c.Args) < len(params)) {
			a.Error(c.Callee, "expected %d arguments, got %d", len(params), len(c.Args))
		}

		for i := 0; i < min(len(c.Args), len(params)); i++ {
			arg := a.AnalyzeExpr(c.Args[i])
			if arg.Invalid() {
				continue
			}

			a.ExpectType(params[i], arg, c.Args[i])
		}

		for i := len(params); i < len(c.Args); i++ {
			a.AnalyzeExpr(c.Args[i])
		}

		return ExprInfo{Type: f.Returns}
	}

	return a.Error(c.Callee, "expected a function, got '%s'", expr.Type)
}

func (a *analyzer) VisitCast(c *ast.Cast) ExprInfo {
	expr := a.AnalyzeExpr(c.Expr)
	if expr.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	to := a.ResolveAndAnalyzeType(c.Type)
	if to == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	if _, ok := GetExplicitCast(a.typeEnv, expr.Type, to); ok {
		return ExprInfo{Type: to}
	}

	return a.Error(c, "'%s' cannot be cast to '%s'", expr.Type, to)
}

func (a *analyzer) VisitBadExpr(_ *ast.BadExpr) ExprInfo {
	return ExprInfo{Type: types.Invalid}
}

// Utils

func (a *analyzer) WantsFunction(node ast.Node) bool {
	switch parent := node.Parent().(type) {
	case *ast.Call:
		if parent.Callee == node {
			return true
		}

		for i, arg := range parent.Args {
			if arg == node {
				if f, ok := a.AnalyzeExpr(parent.Callee).Type.(*types.Func); ok && i < len(f.Params) {
					_, ok := f.Params[i].(*types.Func)
					return ok
				}
			}
		}

	case *ast.Binary:
		if parent.Op == ast.Equal && parent.Right == node {
			if _, ok := a.AnalyzeExpr(parent.Left).Type.(*types.Func); ok {
				return true
			}
		}
	}

	return false
}

func (a *analyzer) AnalyzeExpr(expr ast.Expr) ExprInfo {
	if core.IsNil(expr) {
		return ExprInfo{}
	}

	info := ast.VisitExpr(a, expr)
	a.exprInfos[expr] = info

	return info
}
