package ast

import (
	"fireball/core"
	"fireball/lexer"
	"iter"
)

type Expr interface {
	Node

	_isExpr()
}

// Bool

type Bool struct {
	baseNode

	Value bool
}

func (b *Bool) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *Bool) _isExpr() {}

// Number

type Number struct {
	baseLeafNode

	Token lexer.Token
}

func (n *Number) Range() core.Range {
	return n.Token.Range
}

func (n *Number) _isExpr() {}

// Character

type Character struct {
	baseNode

	Rune rune
}

func (c *Character) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (c *Character) _isExpr() {}

// String

type String struct {
	baseNode

	Runes []rune
}

func (s *String) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (s *String) _isExpr() {}

// Null

type Null struct {
	baseNode
}

func (n *Null) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (n *Null) _isExpr() {}

// SizeOf

type SizeOf struct {
	baseNode

	Type Type
}

func (s *SizeOf) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(s.Type) {
			return
		}
	}
}

func (s *SizeOf) _isExpr() {}

// AlignOf

type AlignOf struct {
	baseNode

	Type Type
}

func (a *AlignOf) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(a.Type) {
			return
		}
	}
}

func (a *AlignOf) _isExpr() {}

// OffsetOf

type OffsetOf struct {
	baseNode

	Type  Type
	Field *Leaf
}

func (o *OffsetOf) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(o.Type) {
			return
		}
		if !yield(o.Field) {
			return
		}
	}
}

func (o *OffsetOf) _isExpr() {}

// Prefix

type PrefixOp uint8

const (
	Negate PrefixOp = iota
	Not

	IncrementE
	DecrementE

	AddressOf
	Dereference
)

type Prefix struct {
	baseNode

	Op   PrefixOp
	Expr Expr
}

func (u *Prefix) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		yield(u.Expr)
	}
}

func (u *Prefix) _isExpr() {}

// Postfix

type PostfixOp uint8

const (
	IncrementO PostfixOp = iota
	DecrementO
)

type Postfix struct {
	baseNode

	Expr Expr
	Op   PostfixOp
}

func (p *Postfix) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		yield(p.Expr)
	}
}

func (p *Postfix) _isExpr() {}

// Binary

type BinaryOp uint8

const (
	Add BinaryOp = iota
	Subtract
	Multiply
	Divide
	Modulo

	ShiftLeft
	ShiftRightSignExt
	ShiftRightZeroExt

	BitOr
	BitXor
	BitAnd

	BoolAnd
	BoolOr

	Equal
	NotEqual

	Less
	LessEqual
	Greater
	GreaterEqual

	Assign

	AddAssign
	SubtractAssign
	MultiplyAssign
	DivideAssign
	ModuloAssign

	ShiftLeftAssign
	ShiftRightSignExtAssign
	ShiftRightZeroExtAssign

	BitOrAssign
	BitXorAssign
	BitAndAssign
)

func (b BinaryOp) IsMath() bool {
	return b <= Modulo
}

func (b BinaryOp) IsBitwise() bool {
	return b >= ShiftLeft && b <= BitAnd
}

func (b BinaryOp) IsBoolean() bool {
	return b == BoolAnd || b == BoolOr
}

func (b BinaryOp) IsEquality() bool {
	return b == Equal || b == NotEqual
}

func (b BinaryOp) IsRelational() bool {
	return b >= Less && b <= GreaterEqual
}

func (b BinaryOp) IsCompoundAssign() bool {
	return b >= AddAssign && b <= BitAndAssign
}

func (b BinaryOp) CompoundAssignBase() BinaryOp {
	switch b {
	case AddAssign:
		return Add
	case SubtractAssign:
		return Subtract
	case MultiplyAssign:
		return Multiply
	case DivideAssign:
		return Divide
	case ModuloAssign:
		return Modulo

	case ShiftLeftAssign:
		return ShiftLeft
	case ShiftRightSignExtAssign:
		return ShiftRightSignExt
	case ShiftRightZeroExtAssign:
		return ShiftRightZeroExt

	case BitOrAssign:
		return BitOr
	case BitXorAssign:
		return BitXor
	case BitAndAssign:
		return BitAnd

	default:
		panic("ast.BinaryOp.CompoundBase() - Not compound operator")
	}
}

type Binary struct {
	baseNode

	Left  Expr
	Op    BinaryOp
	Right Expr
}

func (b *Binary) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(b.Left) {
			return
		}
		yield(b.Right)
	}
}

func (b *Binary) _isExpr() {}

// Identifier

type Identifier struct {
	baseNode

	Path *IdentifierPath
}

func (i *Identifier) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(i.Path) {
			return
		}
	}
}

func (i *Identifier) _isExpr() {}

// Index

type Index struct {
	baseNode

	Expr  Expr
	Index Expr
}

func (i *Index) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(i.Expr) {
			return
		}
		yield(i.Index)
	}
}

func (i *Index) _isExpr() {}

// Member

type Member struct {
	baseNode

	Expr Expr
	Name *Leaf
}

func (m *Member) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(m.Expr) {
			return
		}
		yield(m.Name)
	}
}

func (m *Member) _isExpr() {}

// Call

type Call struct {
	baseNode

	Callee Expr
	Args   []Expr
}

func (c *Call) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(c.Callee) {
			return
		}
		for _, arg := range c.Args {
			if !yield(arg) {
				return
			}
		}
	}
}

func (c *Call) _isExpr() {}

// Cast

type Cast struct {
	baseNode

	Expr Expr
	Type Type
}

func (c *Cast) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(c.Expr) {
			return
		}
		if !yield(c.Type) {
			return
		}
	}
}

func (c *Cast) _isExpr() {}

// Bad

type BadExpr struct {
	baseNode
}

func (b *BadExpr) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadExpr) _isExpr() {}
