package ast

import (
	"fireball/core"
	"iter"
)

type DeclVisitor interface {
	VisitStruct(s *Struct)
	VisitFunc(f *Func)

	VisitBadDecl(b *BadDecl)
}

type Decl interface {
	Node

	Name() string

	VisitDecl(visitor DeclVisitor)
}

// Struct

type Struct struct {
	baseNode

	Name_  *Leaf
	Fields []*NameType
}

func (s *Struct) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(s.Name_) {
			return
		}
		for _, field := range s.Fields {
			if !yield(field) {
				return
			}
		}
	}
}

func (s *Struct) Name() string {
	return s.Name_.Token.Text
}

func (s *Struct) VisitDecl(visitor DeclVisitor) {
	visitor.VisitStruct(s)
}

// Func

type Func struct {
	baseNode

	Name_ *Leaf

	Returns Type
	Params  []*NameType
	VarArgs bool

	Body Stmt // optional
}

func (f *Func) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(f.Name_) {
			return
		}
		if !yield(f.Returns) {
			return
		}
		for _, param := range f.Params {
			if !yield(param) {
				return
			}
		}
		if !core.IsNil(f.Body) {
			yield(f.Body)
		}
	}
}

func (f *Func) Name() string {
	return f.Name_.Token.Text
}

func (f *Func) VisitDecl(visitor DeclVisitor) {
	visitor.VisitFunc(f)
}

// Bad

type BadDecl struct {
	baseNode
}

func (b *BadDecl) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadDecl) Name() string {
	return ""
}

func (b *BadDecl) VisitDecl(visitor DeclVisitor) {
	visitor.VisitBadDecl(b)
}
