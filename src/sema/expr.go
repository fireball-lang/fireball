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

func (a *analyzer) VisitBool(_ *ast.Bool) {
	a.typ = types.PrimitiveBool
}

func (a *analyzer) VisitNumber(n *ast.Number) {
	switch n.Token.Kind {
	case lexer.BinaryInteger, lexer.HexInteger, lexer.UnsignedInteger:
		value := lexer.ParseInteger(n.Token)

		if value <= math.MaxUint8 {
			a.typ = types.PrimitiveU8
		} else if value <= math.MaxUint16 {
			a.typ = types.PrimitiveU16
		} else if value <= math.MaxUint32 {
			a.typ = types.PrimitiveU32
		} else {
			a.typ = types.PrimitiveU64
		}

	case lexer.SignedInteger:
		value := lexer.ParseInteger(n.Token)

		if value <= math.MaxInt8 {
			a.typ = types.PrimitiveI8
		} else if value <= math.MaxInt16 {
			a.typ = types.PrimitiveI16
		} else if value <= math.MaxInt32 {
			a.typ = types.PrimitiveI32
		} else {
			a.typ = types.PrimitiveI64
		}

	case lexer.Decimal:
		a.typ = types.PrimitiveF64

	case lexer.Decimal32bit:
		a.typ = types.PrimitiveF32

	default:
		panic("sema.analyzer.VisitNumber() - Invalid token kind")
	}
}

func (a *analyzer) VisitCharacter(_ *ast.Character) {
	a.typ = types.PrimitiveU32
}

var stringType = &types.Pointer{Pointee: types.PrimitiveU8}

func (a *analyzer) VisitString(_ *ast.String) {
	a.typ = stringType
}

func (a *analyzer) VisitSizeOf(s *ast.SizeOf) {
	typ := a.AnalyzeType(s.Type)
	a.typ = types.PrimitiveU32

	if typ == types.Invalid {
		return
	}

	a.nodeTypes[s.Type] = typ
}

func (a *analyzer) VisitAlignOf(e *ast.AlignOf) {
	typ := a.AnalyzeType(e.Type)
	a.typ = types.PrimitiveU32

	if typ == types.Invalid {
		return
	}

	a.nodeTypes[e.Type] = typ
}

func (a *analyzer) VisitOffsetOf(o *ast.OffsetOf) {
	typ := a.AnalyzeType(o.Type)
	a.typ = types.PrimitiveU32

	if typ == types.Invalid {
		return
	}

	a.nodeTypes[o.Type] = typ

	if s, ok := typ.(*types.Struct); ok {
		if _, index := s.Field(o.Field.Token.Text); index == -1 {
			a.Error(o.Field, "field '%s' doesn't exist on '%s'", o.Field.Token.Text, s)
		}
	} else {
		a.Error(o.Type, "expected a struct type, not '%s'", typ)
	}
}

func (a *analyzer) VisitPrefix(u *ast.Prefix) {
	expr := a.AnalyzeExpr(u.Expr)
	if expr.Invalid() {
		a.typ = types.Invalid
		return
	}

	switch u.Op {
	case ast.Negate:
		a.typ = a.ExpectPrimitiveClass(types.IsSigned, "signed numeric", expr, u.Expr)

	case ast.Not:
		a.ExpectType(types.PrimitiveBool, expr, u.Expr)
		a.typ = types.PrimitiveBool

	case ast.AddressOf:
		if !expr.Address {
			a.Error(u.Expr, "cannot take address of a temporary expression")
			a.typ = types.Invalid
			return
		}

		a.typ = &types.Pointer{Pointee: expr.Type}

	case ast.Dereference:
		if p, ok := expr.Type.(*types.Pointer); ok {
			a.typ = p.Pointee
			a.address = true
		} else {
			a.Error(u.Expr, "can only dereference pointers, not '%s'", expr.Type)
			a.typ = types.Invalid
		}

	default:
		panic("sema.analyzer.VisitPrefix() - Invalid operator kind")
	}
}

func (a *analyzer) VisitBinary(b *ast.Binary) {
	left := a.AnalyzeExpr(b.Left)
	right := a.AnalyzeExpr(b.Right)

	if left.Invalid() || right.Invalid() {
		a.typ = types.Invalid
		return
	}

	// Math
	if b.Op.IsMath() {
		left := a.ExpectPrimitiveClass(types.IsNumeric, "numeric", left, b.Left)
		right := a.ExpectPrimitiveClass(types.IsNumeric, "numeric", right, b.Right)

		if left == types.Invalid || right == types.Invalid {
			a.typ = types.Invalid
			return
		}

		a.typ = CommonType(left, right)
		if a.typ == nil {
			a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left, right)
			a.typ = types.Invalid
			return
		}

		return
	}

	// Bitwise
	if b.Op.IsBitwise() {
		left := a.ExpectPrimitiveClass(types.IsInteger, "integer", left, b.Left)
		right := a.ExpectPrimitiveClass(types.IsInteger, "integer", right, b.Right)

		if left == types.Invalid || right == types.Invalid {
			a.typ = types.Invalid
			return
		}

		a.typ = CommonType(left, right)
		if a.typ == nil {
			a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left, right)
			a.typ = types.Invalid
			return
		}

		return
	}

	// Boolean
	if b.Op.IsBoolean() {
		a.ExpectType(types.PrimitiveBool, left, b.Left)
		a.ExpectType(types.PrimitiveBool, right, b.Right)

		a.typ = types.PrimitiveBool
		return
	}

	// Equality
	if b.Op.IsEquality() {
		if common := CommonType(left.Type, right.Type); common == nil && !left.Type.Equals(right.Type) {
			a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left.Type, right.Type)
			a.typ = types.Invalid
			return
		}

		switch left.Type.(type) {
		case *types.Primitive, *types.Pointer:
		default:
			a.Error(b, "equality operators only work on primitive types or pointers, not %s", left.Type)
			a.typ = types.Invalid
			return
		}

		a.typ = types.PrimitiveBool
		return
	}

	// Relational
	if b.Op.IsRelational() {
		left := a.ExpectPrimitiveClass(types.IsNumeric, "numeric", left, b.Left)
		right := a.ExpectPrimitiveClass(types.IsNumeric, "numeric", right, b.Right)

		if left == types.Invalid || right == types.Invalid {
			a.typ = types.Invalid
			return
		}

		if common := CommonType(left, right); common == nil {
			a.Error(b, "binary operator needs compatible types, got '%s' and '%s'", left, right)
			a.typ = types.Invalid
			return
		}

		a.typ = types.PrimitiveBool
		return
	}

	// Assignment
	if b.Op == ast.Assign {
		if !left.Address {
			a.Error(b.Left, "cannot assign to a non-addressable expression")
			a.typ = types.Invalid
			return
		}

		a.ExpectType(left.Type, right, b.Right)

		a.typ = left.Type
		return
	}

	// Invalid
	panic("sema.analyzer.VisitBinary() - Invalid operator kind")
}

func (a *analyzer) VisitIdentifier(i *ast.Identifier) {
	symbol, ok := a.GetSymbol(i.Path)
	if !ok {
		a.typ = types.Invalid
		return
	}

	a.nodeTypes[symbol.Node] = symbol.Type
	a.node = symbol.Node

	switch symbol.Kind {
	case symbols.Param, symbols.Var:
		a.typ = symbol.Type
		a.address = true

	case symbols.Func:
		a.typ = symbol.Type

	case symbols.Struct:
		a.Error(i, "symbol '%s' is a type and cannot be used as an expression", i.Path.LastName())
		a.typ = types.Invalid

	default:
		panic("sema.analyzer.VisitIdentifier() - Invalid symbol kind")
	}
}

func (a *analyzer) VisitIndex(i *ast.Index) {
	// Index
	index := a.AnalyzeExpr(i.Index)
	a.ExpectPrimitiveClass(types.IsInteger, "integer", index, i.Index)

	// Expression
	expr := a.AnalyzeExpr(i.Expr)
	if expr.Invalid() {
		a.typ = types.Invalid
		return
	}

	if p, ok := expr.Type.(*types.Pointer); ok {
		a.typ = p.Pointee
		a.address = true
		return
	}

	if t, ok := expr.Type.(*types.Array); ok {
		a.typ = t.Element
		a.address = expr.Address
		return
	}

	a.Error(i.Expr, "expected an array or a pointer, got '%s'", expr.Type)
	a.typ = types.Invalid
}

func (a *analyzer) VisitMember(m *ast.Member) {
	expr := a.AnalyzeExpr(m.Expr)
	if expr.Invalid() {
		a.typ = types.Invalid
		return
	}

	a.address = expr.Address
	typ := expr.Type

	if p, ok := typ.(*types.Pointer); ok {
		a.address = true
		typ = p.Pointee
	}

	if t, ok := typ.(*types.Struct); ok {
		if field, index := t.Field(m.Name.Token.Text); index != -1 {
			a.typ = field.Type
			return
		}

		if f, typ := a.methodTable.Get(typ, m.Name.Token.Text); !core.IsNil(typ) {
			a.nodeTypes[f] = typ

			a.typ = typ
			a.node = f
			a.address = false

			return
		}

		a.Error(m.Name, "member '%s' doesn't exist on type '%s'", m.Name.Token.Text, t)
		a.typ = types.Invalid

		return
	}

	a.Error(m.Expr, "expected a struct or a pointer to a struct, got '%s'", expr.Type)
	a.typ = types.Invalid
}

func (a *analyzer) VisitCall(c *ast.Call) {
	expr := a.AnalyzeExpr(c.Callee)
	if expr.Invalid() {
		a.typ = types.Invalid
		return
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

		a.typ = f.Returns
		return
	}

	a.Error(c.Callee, "expected a function, got '%s'", expr.Type)
	a.typ = types.Invalid
}

func (a *analyzer) VisitCast(c *ast.Cast) {
	expr := a.AnalyzeExpr(c.Expr)
	if expr.Invalid() {
		a.typ = types.Invalid
		return
	}

	to := a.AnalyzeType(c.Type)
	if to == types.Invalid {
		a.typ = types.Invalid
		return
	}

	if _, ok := GetExplicitCast(expr.Type, to); ok {
		a.typ = to
		return
	}

	a.Error(c, "'%s' cannot be cast to '%s'", expr.Type, to)
	a.typ = types.Invalid
}

func (a *analyzer) VisitBadExpr(_ *ast.BadExpr) {
	a.typ = types.Invalid
}

// Utils

func (a *analyzer) AnalyzeExpr(expr ast.Expr) ExprInfo {
	if core.IsNil(expr) {
		return ExprInfo{}
	}

	expr.VisitExpr(a)

	if core.IsNil(a.typ) {
		panic("sema.analyzer.AnalyzeExpr() - Expression type is nil")
	}

	info := ExprInfo{a.typ, a.node, a.address}
	a.exprInfos[expr] = info

	a.typ = nil
	a.node = nil
	a.address = false

	return info
}
