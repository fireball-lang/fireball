package ast

import (
	"iter"
)

type Decl interface {
	Node

	Name() string

	Visit(visitor DeclVisitor)
}

type DeclVisitor interface {
	VisitFunc(f *Func)
}

// Func

type Func struct {
	baseRangeNode

	NameN   *Leaf
	Params  []*Param
	Body    *Block
	Returns Type
}

func (f *Func) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if f.NameN != nil && !yield(f.NameN) {
			return
		}

		for _, param := range f.Params {
			if !yield(param) {
				return
			}
		}

		if f.Body != nil && !yield(f.Body) {
			return
		}
		if IsValid(f.Returns) && !yield(f.Returns) {
			return
		}
	}
}

func (f *Func) Name() string {
	if f.NameN != nil {
		return f.NameN.Token.Text
	}

	return ""
}

func (f *Func) Visit(visitor DeclVisitor) {
	visitor.VisitFunc(f)
}

// Param

type Param struct {
	baseRangeNode

	Name *Leaf
	Type Type
}

func (p *Param) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if p.Name != nil && !yield(p.Name) {
			return
		}
		if IsValid(p.Type) && !yield(p.Name) {
			return
		}
	}
}
