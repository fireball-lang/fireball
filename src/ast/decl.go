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

type Attribute struct {
	baseNode

	Name      *Leaf
	Arguments []Expr
}

func (a *Attribute) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(a.Name) {
			return
		}
		for _, argument := range a.Arguments {
			if !yield(argument) {
				return
			}
		}
	}
}

// Struct

type Struct struct {
	baseNode

	Attributes []*Attribute

	Name_  *Leaf
	Fields []*NameType
}

func (s *Struct) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range s.Attributes {
			if !yield(attribute) {
				return
			}
		}
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

	Attributes []*Attribute

	Name_ *Leaf

	Returns Type
	Params  []*NameType
	VarArgs bool

	Body Stmt // optional
}

func (f *Func) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range f.Attributes {
			if !yield(attribute) {
				return
			}
		}
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

func (f *Func) GetTestName() string {
	for _, attribute := range f.Attributes {
		if attribute.Name.Token.Text == "test" {
			name := ""

			if len(attribute.Arguments) > 0 {
				if s, ok := attribute.Arguments[0].(*String); ok {
					name = string(s.Runes)
				}
			}

			if name == "" {
				name = f.Name()
			}

			return name
		}
	}

	return ""
}

func (f *Func) IsExtern() bool {
	for _, attribute := range f.Attributes {
		if attribute.Name.Token.Text == "extern" {
			return true
		}
	}

	return false
}

func (f *Func) GetLinkName() string {
	for _, attribute := range f.Attributes {
		if attribute.Name.Token.Text == "link_name" {
			name := ""

			if len(attribute.Arguments) > 0 {
				if s, ok := attribute.Arguments[0].(*String); ok {
					name = string(s.Runes)
				}
			}

			return name
		}
	}

	return ""
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
