package ast

import (
	"fireball/core"
	"fireball/lexer"
	"iter"
)

type ExprVisitor interface {
	VisitBool(b *Bool)
	VisitNumber(n *Number)
	VisitCharacter(c *Character)
	VisitString(s *String)

	VisitSizeOf(s *SizeOf)
	VisitAlignOf(a *AlignOf)
	VisitOffsetOf(o *OffsetOf)

	VisitPrefix(u *Prefix)
	VisitBinary(b *Binary)

	VisitIdentifier(i *Identifier)
	VisitIndex(i *Index)
	VisitMember(m *Member)
	VisitCall(c *Call)
	VisitCast(c *Cast)

	VisitBadExpr(p *BadExpr)
}

type Expr interface {
	Node

	VisitExpr(visitor ExprVisitor)
}

// Bool

type Bool struct {
	baseNode

	Value bool
}

func (b *Bool) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *Bool) VisitExpr(visitor ExprVisitor) {
	visitor.VisitBool(b)
}

// Number

type Number struct {
	baseLeafNode

	Token lexer.Token
}

func (n *Number) Range() core.Range {
	return n.Token.Range
}

func (n *Number) VisitExpr(visitor ExprVisitor) {
	visitor.VisitNumber(n)
}

// Character

type Character struct {
	baseNode

	Rune rune
}

func (c *Character) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (c *Character) VisitExpr(visitor ExprVisitor) {
	visitor.VisitCharacter(c)
}

// String

type String struct {
	baseNode

	Runes []rune
}

func (s *String) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (s *String) VisitExpr(visitor ExprVisitor) {
	visitor.VisitString(s)
}

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

func (s *SizeOf) VisitExpr(visitor ExprVisitor) {
	visitor.VisitSizeOf(s)
}

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

func (a *AlignOf) VisitExpr(visitor ExprVisitor) {
	visitor.VisitAlignOf(a)
}

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

func (o *OffsetOf) VisitExpr(visitor ExprVisitor) {
	visitor.VisitOffsetOf(o)
}

// Prefix

type PrefixOp uint8

const (
	Negate PrefixOp = iota
	Not

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

func (u *Prefix) VisitExpr(visitor ExprVisitor) {
	visitor.VisitPrefix(u)
}

// Binary

type BinaryOp uint8

const (
	Add BinaryOp = iota
	Subtract
	Multiply
	Divide
	Modulo

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
)

func (b BinaryOp) IsMath() bool {
	return b <= Modulo
}

func (b BinaryOp) IsBitwise() bool {
	return b >= BitOr && b <= BitAnd
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

func (b *Binary) VisitExpr(visitor ExprVisitor) {
	visitor.VisitBinary(b)
}

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

func (i *Identifier) VisitExpr(visitor ExprVisitor) {
	visitor.VisitIdentifier(i)
}

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

func (i *Index) VisitExpr(visitor ExprVisitor) {
	visitor.VisitIndex(i)
}

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

func (m *Member) VisitExpr(visitor ExprVisitor) {
	visitor.VisitMember(m)
}

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

func (c *Call) VisitExpr(visitor ExprVisitor) {
	visitor.VisitCall(c)
}

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

func (c *Cast) VisitExpr(visitor ExprVisitor) {
	visitor.VisitCast(c)
}

// Bad

type BadExpr struct {
	baseNode
}

func (b *BadExpr) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadExpr) VisitExpr(visitor ExprVisitor) {
	visitor.VisitBadExpr(b)
}
