package ast

import (
	"fireball/core"
	"iter"
)

type Stmt interface {
	Node

	_isStmt()
}

// Block

type Block struct {
	baseNode

	Stmts []Stmt
}

func (b *Block) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, stmt := range b.Stmts {
			if !yield(stmt) {
				return
			}
		}
	}
}

func (b *Block) _isStmt() {}

// Expression

type Expression struct {
	baseNode

	Expr Expr
}

func (e *Expression) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		yield(e.Expr)
	}
}

func (e *Expression) _isStmt() {}

// Var

type Var struct {
	baseNode

	Name        *Leaf
	Type        Type // optional
	Initializer Expr // optional
}

func (v *Var) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(v.Name) {
			return
		}
		if !core.IsNil(v.Type) && !yield(v.Type) {
			return
		}
		if !core.IsNil(v.Initializer) {
			yield(v.Initializer)
		}
	}
}

func (v *Var) _isStmt() {}

// If

type If struct {
	baseNode

	Condition   Expr
	BranchTrue  Stmt
	BranchFalse Stmt // optional
}

func (i *If) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(i.Condition) {
			return
		}
		if !yield(i.BranchTrue) {
			return
		}
		if !core.IsNil(i.BranchFalse) {
			yield(i.BranchFalse)
		}
	}
}

func (i *If) _isStmt() {}

// While

type While struct {
	baseNode

	Condition Expr
	Body      Stmt
}

func (w *While) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(w.Condition) {
			return
		}
		yield(w.Body)
	}
}

func (w *While) _isStmt() {}

// For

type For struct {
	baseNode

	Initializer Stmt // optional
	Condition   Expr // optional
	Increment   Expr // optional

	Body Stmt
}

func (f *For) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !core.IsNil(f.Initializer) && !yield(f.Initializer) {
			return
		}
		if !core.IsNil(f.Condition) && !yield(f.Condition) {
			return
		}
		if !core.IsNil(f.Increment) && !yield(f.Increment) {
			return
		}
		yield(f.Body)
	}
}

func (f *For) _isStmt() {}

// Return

type Return struct {
	baseNode

	Value Expr // optional
}

func (r *Return) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !core.IsNil(r.Value) {
			yield(r.Value)
		}
	}
}

func (r *Return) _isStmt() {}

// Break

type Break struct {
	baseNode
}

func (b *Break) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *Break) _isStmt() {}

// Continue

type Continue struct {
	baseNode
}

func (c *Continue) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (c *Continue) _isStmt() {}

// Bad

type BadStmt struct {
	baseNode
}

func (b *BadStmt) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadStmt) _isStmt() {}
