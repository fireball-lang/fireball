package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"math"
)

// Visitor

func (a *analyzer) VisitBool(_ *ast.Bool) ExprInfo {
	return ExprInfo{Type: types.PrimitiveBool}
}

func (a *analyzer) VisitNumber(n *ast.Number) ExprInfo {
	switch n.Token.Kind {
	case lexer.BinaryInteger, lexer.HexInteger, lexer.UnsignedInteger:
		value := lexer.ParseInteger(n.Token)

		if value <= math.MaxUint8 {
			return ExprInfo{Type: types.PrimitiveU8}
		} else if value <= math.MaxUint16 {
			return ExprInfo{Type: types.PrimitiveU16}
		} else if value <= math.MaxUint32 {
			return ExprInfo{Type: types.PrimitiveU32}
		}
		return ExprInfo{Type: types.PrimitiveU64}

	case lexer.SignedInteger:
		value := lexer.ParseInteger(n.Token)

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

func (a *analyzer) VisitCharacter(_ *ast.Character) ExprInfo {
	return ExprInfo{Type: types.PrimitiveU32}
}

var stringType = &types.Pointer{Pointee: types.PrimitiveU8}

func (a *analyzer) VisitString(_ *ast.String) ExprInfo {
	return ExprInfo{Type: stringType}
}

var voidPtrType = &types.Pointer{Pointee: types.PrimitiveVoid}

func (a *analyzer) VisitNull(_ *ast.Null) ExprInfo {
	return ExprInfo{Type: voidPtrType}
}

func (a *analyzer) VisitSizeOf(s *ast.SizeOf) ExprInfo {
	typ := a.AnalyzeType(s.Type)
	if typ == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	a.nodeTypes[s.Type] = typ
	return ExprInfo{Type: types.PrimitiveU32}
}

func (a *analyzer) VisitAlignOf(e *ast.AlignOf) ExprInfo {
	typ := a.AnalyzeType(e.Type)
	if typ == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	a.nodeTypes[e.Type] = typ
	return ExprInfo{Type: types.PrimitiveU32}
}

func (a *analyzer) VisitOffsetOf(o *ast.OffsetOf) ExprInfo {
	typ := a.AnalyzeType(o.Type)
	if typ == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	a.nodeTypes[o.Type] = typ

	if s, ok := typ.(*types.Struct); ok {
		if _, index := s.Field(o.Field.Token.Text); index == -1 {
			a.Error(o.Field, "field '%s' doesn't exist on '%s'", o.Field.Token.Text, s)
		}
	} else {
		a.Error(o.Type, "expected a struct type, not '%s'", typ)
	}

	return ExprInfo{Type: types.PrimitiveU32}
}

func (a *analyzer) VisitPrefix(u *ast.Prefix) ExprInfo {
	expr := a.AnalyzeExpr(u.Expr)
	if expr.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	switch u.Op {
	case ast.Negate:
		return ExprInfo{Type: a.ExpectPrimitiveClass(types.IsSigned, "signed numeric", expr, u.Expr)}

	case ast.Not:
		a.ExpectType(types.PrimitiveBool, expr, u.Expr)
		return ExprInfo{Type: types.PrimitiveBool}

	case ast.AddressOf:
		if !expr.Address {
			return a.Error(u.Expr, "cannot take address of a temporary expression")
		}

		return ExprInfo{Type: &types.Pointer{Pointee: expr.Type}}

	case ast.Dereference:
		if p, ok := expr.Type.(*types.Pointer); ok {
			return ExprInfo{
				Type:    p.Pointee,
				Address: true,
			}
		}

		return a.Error(u.Expr, "can only dereference pointers, not '%s'", expr.Type)

	default:
		panic("sema.analyzer.VisitPrefix() - Invalid operator kind")
	}
}

func (a *analyzer) VisitBinary(b *ast.Binary) ExprInfo {
	left := a.AnalyzeExpr(b.Left)
	right := a.AnalyzeExpr(b.Right)

	if left.Invalid() || right.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	// Math
	if b.Op.IsMath() {
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
	if b.Op.IsBitwise() {
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
	if b.Op.IsBoolean() {
		a.ExpectType(types.PrimitiveBool, left, b.Left)
		a.ExpectType(types.PrimitiveBool, right, b.Right)

		return ExprInfo{Type: types.PrimitiveBool}
	}

	// Equality
	if b.Op.IsEquality() {
		if common := CommonType(left.Type, right.Type); common == nil && !left.Type.Equals(right.Type) {
			return a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left.Type, right.Type)
		}

		switch left.Type.(type) {
		case *types.Primitive, *types.Pointer:
		default:
			return a.Error(b, "equality operators only work on primitive types or pointers, not %s", left.Type)
		}

		return ExprInfo{Type: types.PrimitiveBool}
	}

	// Relational
	if b.Op.IsRelational() {
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

	// Assignment
	if b.Op == ast.Assign {
		if !left.Address {
			return a.Error(b.Left, "cannot assign to a non-addressable expression")
		}

		a.ExpectType(left.Type, right, b.Right)

		return ExprInfo{Type: left.Type}
	}

	// Invalid
	panic("sema.analyzer.VisitBinary() - Invalid operator kind")
}

func (a *analyzer) VisitIdentifier(i *ast.Identifier) ExprInfo {
	symbol, ok := a.GetSymbol(i.Path)
	if !ok {
		return ExprInfo{Type: types.Invalid}
	}

	a.nodeTypes[symbol.Node] = symbol.Type

	switch symbol.Kind {
	case symbols.Param, symbols.Var:
		return ExprInfo{
			Type:    symbol.Type,
			Node:    symbol.Node,
			Address: true,
		}

	case symbols.Func:
		return ExprInfo{
			Type: symbol.Type,
			Node: symbol.Node,
		}

	case symbols.Struct:
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
			Address: true,
		}
	}

	if t, ok := expr.Type.(*types.Array); ok {
		return ExprInfo{
			Type:    t.Element,
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

	if p, ok := typ.(*types.Pointer); ok {
		address = true
		typ = p.Pointee
	}

	if t, ok := typ.(*types.Struct); ok {
		if field, index := t.Field(m.Name.Token.Text); index != -1 {
			return ExprInfo{
				Type:    field.Type,
				Address: address,
			}
		}

		if f, typ := a.methodTable.Get(typ, m.Name.Token.Text); !core.IsNil(typ) {
			a.nodeTypes[f] = typ

			return ExprInfo{
				Type: typ,
				Node: f,
			}
		}

		return a.Error(m.Name, "member '%s' doesn't exist on type '%s'", m.Name.Token.Text, t)
	}

	return a.Error(m.Expr, "expected a struct or a pointer to a struct, got '%s'", expr.Type)
}

func (a *analyzer) VisitCall(c *ast.Call) ExprInfo {
	expr := a.AnalyzeExpr(c.Callee)
	if expr.Invalid() {
		return ExprInfo{Type: types.Invalid}
	}

	if f, ok := expr.Type.(*types.Func); ok {
		params := f.Params
		if expr.Node.(*ast.Func).IsMethod() && expr.Node.(*ast.Func).Receiver != nil {
			params = params[1:]
		}

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

	to := a.AnalyzeType(c.Type)
	if to == types.Invalid {
		return ExprInfo{Type: types.Invalid}
	}

	if _, ok := GetExplicitCast(expr.Type, to); ok {
		return ExprInfo{Type: to}
	}

	return a.Error(c, "'%s' cannot be cast to '%s'", expr.Type, to)
}

func (a *analyzer) VisitBadExpr(_ *ast.BadExpr) ExprInfo {
	return ExprInfo{Type: types.Invalid}
}

// Utils

func (a *analyzer) AnalyzeExpr(expr ast.Expr) ExprInfo {
	if core.IsNil(expr) {
		return ExprInfo{}
	}

	info := ast.VisitExpr(a, expr)
	a.exprInfos[expr] = info

	return info
}
