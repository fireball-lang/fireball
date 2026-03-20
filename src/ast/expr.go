package ast

import (
	"fireball/lexer"
	"iter"
)

type ExprVisitor interface {
	VisitUnary(u *Unary)
	VisitBinary(b *Binary)

	VisitBool(b *Bool)
	VisitNumber(n *Number)
	VisitCharacter(c *Character)
	VisitString(s *String)

	VisitIdentifier(i *Identifier)
	VisitIndex(i *Index)
	VisitMember(m *Member)
	VisitCall(c *Call)

	VisitBadExpr(p *BadExpr)
}

type Expr interface {
	Node

	VisitExpr(visitor ExprVisitor)
}

// Unary

type UnaryOp uint8

const (
	Negate UnaryOp = iota
	Not
)

type Unary struct {
	baseNode

	Op   UnaryOp
	Expr Expr
}

func (u *Unary) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		yield(u.Expr)
	}
}

func (u *Unary) VisitExpr(visitor ExprVisitor) {
	visitor.VisitUnary(u)
}

// Binary

type BinaryOp uint8

const (
	Add BinaryOp = iota
	Subtract
	Multiply
	Divide
	Modulo

	BoolAnd
	BoolOr

	BitAnd
	BitOr
	BitXor

	Equal
	Less
	LessEqual
	Greater
	GreaterEqual

	Assign
)

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
	baseLeafNode

	Token lexer.Token
}

func (i *Identifier) Range() lexer.Range {
	return i.Token.Range
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

func (n *Number) Range() lexer.Range {
	return n.Token.Range
}

func (n *Number) VisitExpr(visitor ExprVisitor) {
	visitor.VisitNumber(n)
}

// Character

type Character struct {
	baseLeafNode

	Token lexer.Token
}

func (c *Character) Range() lexer.Range {
	return c.Token.Range
}

func (c *Character) VisitExpr(visitor ExprVisitor) {
	visitor.VisitCharacter(c)
}

// String

type String struct {
	baseLeafNode

	Token lexer.Token
}

func (s *String) Range() lexer.Range {
	return s.Token.Range
}

func (s *String) VisitExpr(visitor ExprVisitor) {
	visitor.VisitString(s)
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
