package ast

import (
	"fireball/lexer"
	"iter"
)

type Expr interface {
	Node

	Result() *ExprResult

	Visit(visitor ExprVisitor)
}

type ExprVisitor interface {
	VisitBlock(b *Block)
	VisitVar(v *Var)
	VisitIf(i *If)
	VisitWhile(w *While)
	VisitBreak(b *Break)
	VisitContinue(c *Continue)
	VisitReturn(r *Return)

	VisitLiteral(l *Literal)
	VisitStructInitializer(s *StructInitializer)
	VisitParen(p *Paren)
	VisitIdentifier(i *Identifier)
	VisitCall(c *Call)
	VisitTypeCall(t *TypeCall)
	VisitIndex(i *Index)
	VisitMember(m *Member)
	VisitUnary(u *Unary)
	VisitBinary(b *Binary)
	VisitIs(i *Is)
	VisitCast(c *Cast)
}

// baseExpr

type baseExpr struct {
	baseRangeNode

	result ExprResult
}

func (b *baseExpr) Result() *ExprResult {
	return &b.result
}

// Block

type Block struct {
	baseExpr

	Exprs []Expr
}

func (b *Block) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, expr := range b.Exprs {
			if !yield(expr) {
				return
			}
		}
	}
}

func (b *Block) Visit(visitor ExprVisitor) {
	visitor.VisitBlock(b)
}

// Var

type Var struct {
	baseExpr

	Name  *Leaf
	Type  Type
	Value Expr
}

func (v *Var) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if v.Name != nil && !yield(v.Name) {
			return
		}
		if IsValid(v.Type) && !yield(v.Type) {
			return
		}
		if IsValid(v.Value) && !yield(v.Value) {
			return
		}
	}
}

func (v *Var) Visit(visitor ExprVisitor) {
	visitor.VisitVar(v)
}

func (v *Var) ActualType() Type {
	if IsValid(v.Type) {
		return v.Type
	}

	if IsValid(v.Value) && !v.Value.Result().Flags.IsInvalid() {
		return v.Value.Result().Type
	}

	return nil
}

// If

type If struct {
	baseExpr

	Condition Expr
	Then      Expr
	Else      Expr
}

func (i *If) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(i.Condition) && !yield(i.Condition) {
			return
		}
		if IsValid(i.Then) && !yield(i.Then) {
			return
		}
		if IsValid(i.Else) && !yield(i.Else) {
			return
		}
	}
}

func (i *If) Visit(visitor ExprVisitor) {
	visitor.VisitIf(i)
}

// While

type While struct {
	baseExpr

	Condition Expr
	Body      Expr
}

func (w *While) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(w.Condition) && !yield(w.Condition) {
			return
		}
		if IsValid(w.Body) && !yield(w.Body) {
			return
		}
	}
}

func (w *While) Visit(visitor ExprVisitor) {
	visitor.VisitWhile(w)
}

// Break

type Break struct {
	baseExpr
}

func (b *Break) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *Break) Visit(visitor ExprVisitor) {
	visitor.VisitBreak(b)
}

// Continue

type Continue struct {
	baseExpr
}

func (c *Continue) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (c *Continue) Visit(visitor ExprVisitor) {
	visitor.VisitContinue(c)
}

// Return

type Return struct {
	baseExpr

	Value Expr
}

func (r *Return) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(r.Value) && !yield(r.Value) {
			return
		}
	}
}

func (r *Return) Visit(visitor ExprVisitor) {
	visitor.VisitReturn(r)
}

// Literal

type Literal struct {
	baseNode

	Value  *Leaf
	result ExprResult
}

func (l *Literal) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(l.Value) && !yield(l.Value) {
			return
		}
	}
}

func (l *Literal) Range() lexer.Range {
	return l.Value.Range()
}

func (l *Literal) Result() *ExprResult {
	return &l.result
}

func (l *Literal) Visit(visitor ExprVisitor) {
	visitor.VisitLiteral(l)
}

// StructInitializer

type StructInitializer struct {
	baseExpr

	Path   *Path
	Struct *Struct

	Fields []*StructInitializerField
}

func (s *StructInitializer) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(s.Path) && !yield(s.Path) {
			return
		}

		for _, field := range s.Fields {
			if !yield(field) {
				return
			}
		}
	}
}

func (s *StructInitializer) Visit(visitor ExprVisitor) {
	visitor.VisitStructInitializer(s)
}

// StructInitializerField

type StructInitializerField struct {
	baseRangeNode

	Name  *Leaf
	Value Expr
}

func (s *StructInitializerField) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(s.Name) && !yield(s.Name) {
			return
		}
		if IsValid(s.Value) && !yield(s.Value) {
			return
		}
	}
}

// Paren

type Paren struct {
	baseExpr

	Expr Expr
}

func (p *Paren) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(p.Expr) && !yield(p.Expr) {
			return
		}
	}
}

func (p *Paren) Visit(visitor ExprVisitor) {
	visitor.VisitParen(p)
}

// Identifier

type Identifier struct {
	baseNode

	Path     *Path
	Resolved Node

	result ExprResult
}

func (i *Identifier) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if i.Path != nil && !yield(i.Path) {
			return
		}
	}
}

func (i *Identifier) Range() lexer.Range {
	return i.Path.Range()
}

func (i *Identifier) Result() *ExprResult {
	return &i.result
}

func (i *Identifier) Visit(visitor ExprVisitor) {
	visitor.VisitIdentifier(i)
}

// Call

type Call struct {
	baseExpr

	Callee Expr
	Args   []Expr
}

func (c *Call) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(c.Callee) && !yield(c.Callee) {
			return
		}

		for _, arg := range c.Args {
			if !yield(arg) {
				return
			}
		}
	}
}

func (c *Call) Visit(visitor ExprVisitor) {
	visitor.VisitCall(c)
}

// TypeCall

type TypeCallKind uint8

const (
	Sizeof TypeCallKind = iota
	Alignof
)

type TypeCall struct {
	baseExpr

	Kind TypeCallKind
	Arg  Type
}

func (t *TypeCall) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(t.Arg) && !yield(t.Arg) {
			return
		}
	}
}

func (t *TypeCall) Visit(visitor ExprVisitor) {
	visitor.VisitTypeCall(t)
}

// Index

type Index struct {
	baseExpr

	Value Expr
	Index Expr
}

func (i *Index) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(i.Value) && !yield(i.Value) {
			return
		}
		if IsValid(i.Index) && !yield(i.Index) {
			return
		}
	}
}

func (i *Index) Visit(visitor ExprVisitor) {
	visitor.VisitIndex(i)
}

// Member

type Member struct {
	baseExpr

	Value Expr
	Name  *Leaf

	Resolved Node
}

func (m *Member) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(m.Value) && !yield(m.Value) {
			return
		}
		if m.Name != nil && !yield(m.Name) {
			return
		}
	}
}

func (m *Member) Visit(visitor ExprVisitor) {
	visitor.VisitMember(m)
}

// Unary

type Unary struct {
	baseExpr

	Expr    Expr
	Op      lexer.TokenKind
	Postfix bool
}

func (u *Unary) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(u.Expr) && !yield(u.Expr) {
			return
		}
	}
}

func (u *Unary) Visit(visitor ExprVisitor) {
	visitor.VisitUnary(u)
}

// Binary

type Binary struct {
	baseExpr

	Left  Expr
	Op    lexer.TokenKind
	Right Expr
}

func (b *Binary) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(b.Left) && !yield(b.Left) {
			return
		}
		if IsValid(b.Right) && !yield(b.Right) {
			return
		}
	}
}

func (b *Binary) Visit(visitor ExprVisitor) {
	visitor.VisitBinary(b)
}

// Is

type Is struct {
	baseExpr

	Value Expr
	Type  Type
}

func (i *Is) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(i.Value) && !yield(i.Value) {
			return
		}
		if IsValid(i.Type) && !yield(i.Type) {
			return
		}
	}
}

func (i *Is) Visit(visitor ExprVisitor) {
	visitor.VisitIs(i)
}

// Cast

type Cast struct {
	baseExpr

	Value Expr
	Type  Type
}

func (c *Cast) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(c.Value) && !yield(c.Value) {
			return
		}
		if IsValid(c.Type) && !yield(c.Type) {
			return
		}
	}
}

func (c *Cast) Visit(visitor ExprVisitor) {
	visitor.VisitCast(c)
}
