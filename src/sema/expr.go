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

		if value <= math.MaxUint32 {
			a.typ = types.PrimitiveU32
		} else {
			a.typ = types.PrimitiveU64
		}

	case lexer.SignedInteger:
		value := lexer.ParseInteger(n.Token)

		if value <= math.MaxInt32 {
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

		if !left.Equals(right) {
			a.Error(b, "binary operator needs to have the same types, got '%s' and '%s'", left, right)
			a.typ = types.Invalid
			return
		}

		a.typ = left
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

		if !left.Equals(right) {
			a.Error(b, "binary operator needs to have the same types, got '%s' and '%s'", left, right)
			a.typ = types.Invalid
			return
		}

		a.typ = left
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
		if !left.Type.Equals(right.Type) {
			a.Error(b, "binary operator needs to have the same types, got '%s' and '%s'", left.Type, right.Type)
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

		if !left.Equals(right) {
			a.Error(b, "binary operator needs to have the same types, got '%s' and '%s'", left, right)
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

		if !left.Type.Equals(right.Type) {
			a.Error(b, "binary operator needs to have the same types, got '%s' and '%s'", left.Type, right.Type)
			a.typ = types.Invalid
			return
		}

		a.typ = left.Type
		return
	}

	// Invalid
	panic("sema.analyzer.VisitBinary() - Invalid operator kind")
}

func (a *analyzer) VisitIdentifier(i *ast.Identifier) {
	symbol, ok := a.scope.Get(i.Token.Text)
	if !ok {
		a.Error(i, "symbol '%s' not found", i.Token.Text)
		a.typ = types.Invalid
		return
	}

	switch symbol.Kind {
	case symbols.Param, symbols.Var:
		a.typ = symbol.Type
		a.address = true

	case symbols.Func:
		a.typ = symbol.Type

	case symbols.Struct:
		a.Error(i, "symbol '%s' is a type and cannot be used as an expression", i.Token.Text)
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

	if t, ok := expr.Type.(*types.Array); ok {
		a.typ = t.Element
		a.address = expr.Address
		return
	}

	a.Error(i.Expr, "expected an array, got '%s'", expr.Type)
	a.typ = types.Invalid
}

func (a *analyzer) VisitMember(m *ast.Member) {
	expr := a.AnalyzeExpr(m.Expr)
	if expr.Invalid() {
		a.typ = types.Invalid
		return
	}

	if t, ok := expr.Type.(*types.Struct); ok {
		a.address = expr.Address

		if field, index := t.Field(m.Name.Token.Text); index != -1 {
			a.typ = field.Type
		} else {
			a.Error(m.Name, "member '%s' doesn't exist on type '%s'", m.Name.Token.Text, t)
			a.typ = types.Invalid
		}

		return
	}

	a.Error(m.Expr, "expected a struct, got '%s'", expr.Type)
	a.typ = types.Invalid
}

func (a *analyzer) VisitCall(c *ast.Call) {
	expr := a.AnalyzeExpr(c.Callee)
	if expr.Invalid() {
		a.typ = types.Invalid
		return
	}

	if f, ok := expr.Type.(*types.Func); ok {
		a.typ = f.Returns

		if len(c.Args) != len(f.Params) && (!f.VarArgs || len(c.Args) < len(f.Params)) {
			a.Error(c.Callee, "expected %d arguments, got %d", len(f.Params), len(c.Args))
		}

		for i := 0; i < min(len(c.Args), len(f.Params)); i++ {
			arg := a.AnalyzeExpr(c.Args[i])
			if arg.Invalid() {
				continue
			}

			a.ExpectType(f.Params[i], arg, c.Args[i])
		}

		for i := len(f.Params); i < len(c.Args); i++ {
			a.AnalyzeExpr(c.Args[i])
		}

		return
	}

	a.Error(c.Callee, "expected a function, got '%s'", expr.Type)
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

	info := ExprInfo{a.typ, a.address}
	a.exprInfos[expr] = info

	a.typ = nil
	a.address = false

	return info
}
