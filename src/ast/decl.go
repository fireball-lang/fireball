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
	VisitStruct(f *Struct)
	VisitFunc(f *Func)
}

// Attribute

type Attribute struct {
	baseRangeNode

	Name  *Leaf
	Param string
}

func (a *Attribute) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(a.Name) && !yield(a.Name) {
			return
		}
	}
}

type attributeHolder struct {
	Attributes []*Attribute
}

func (a *attributeHolder) GetAttribute(name string) *Attribute {
	for _, attribute := range a.Attributes {
		if attribute.Name != nil && attribute.Name.Token.Text == name {
			return attribute
		}
	}

	return nil
}

// Struct

type Struct struct {
	baseRangeNode
	attributeHolder

	NameN  *Leaf
	Fields []*Field
}

func (s *Struct) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range s.Attributes {
			if !yield(attribute) {
				return
			}
		}

		if s.NameN != nil && !yield(s.NameN) {
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
	if s.NameN != nil {
		return s.NameN.Token.Text
	}

	return ""
}

func (s *Struct) Visit(visitor DeclVisitor) {
	visitor.VisitStruct(s)
}

func (s *Struct) GetField(name string) (*Field, int) {
	for i, field := range s.Fields {
		if field.Name != nil && field.Name.Token.Text == name {
			return field, i
		}
	}

	return nil, -1
}

// Field

type Field struct {
	baseRangeNode

	Name *Leaf
	Type Type
}

func (f *Field) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if f.Name != nil && !yield(f.Name) {
			return
		}
		if IsValid(f.Type) && !yield(f.Type) {
			return
		}
	}
}

// Func

type Func struct {
	baseRangeNode
	attributeHolder

	NameN   *Leaf
	Params  []*Param
	varArgs bool
	Body    *Block
	Returns Type
}

func (f *Func) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range f.Attributes {
			if !yield(attribute) {
				return
			}
		}

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

func (f *Func) Equals(other Type) bool {
	return funcTypeEquals(f, other)
}

func (f *Func) String() string {
	return funcTypeString(f)
}

func (f *Func) ParamTypes() iter.Seq[Type] {
	return func(yield func(Type) bool) {
		for _, param := range f.Params {
			if IsValid(param.Type) && !yield(param.Type) {
				return
			}
		}
	}
}

func (f *Func) VarArgs() bool {
	return f.varArgs
}

func (f *Func) ReturnType() Type {
	if IsValid(f.Returns) {
		return f.Returns
	}

	return VoidType
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
		if IsValid(p.Type) && !yield(p.Type) {
			return
		}
	}
}
