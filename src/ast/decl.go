package ast

import (
	"fireball/core"
	"iter"
	"strings"
)

type Decl interface {
	Node

	Name() *Leaf

	_isDecl()
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
	Public     bool

	Name_      *Leaf
	TypeParams []*TypeParam
	Fields     []*Field
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
		for _, param := range s.TypeParams {
			if !yield(param) {
				return
			}
		}
		for _, field := range s.Fields {
			if !yield(field) {
				return
			}
		}
	}
}

func (s *Struct) Name() *Leaf {
	return s.Name_
}

func (s *Struct) _isDecl() {}

// Field

type Field struct {
	baseNode

	Public bool

	Name *Leaf
	Type Type
}

func (f *Field) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(f.Name) {
			return
		}
		yield(f.Type)
	}
}

// Interface

type Interface struct {
	baseNode

	Public bool

	Name_      *Leaf
	TypeParams []*TypeParam

	Methods []*Func
}

func (i *Interface) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(i.Name_) {
			return
		}
		for _, param := range i.TypeParams {
			if !yield(param) {
				return
			}
		}
		for _, method := range i.Methods {
			if !yield(method) {
				return
			}
		}
	}
}

func (i *Interface) Name() *Leaf {
	return i.Name_
}

func (i *Interface) _isDecl() {}

// Impl

type Impl struct {
	baseNode

	TypeParams []*TypeParam

	Type      Type
	Interface *IdentifierType // optional

	Methods []*Func
}

func (i *Impl) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, param := range i.TypeParams {
			if !yield(param) {
				return
			}
		}
		if !yield(i.Type) {
			return
		}
		if i.Interface != nil && !yield(i.Interface) {
			return
		}
		for _, function := range i.Methods {
			if !yield(function) {
				return
			}
		}
	}
}

func (i *Impl) Name() *Leaf {
	return nil
}

func (i *Impl) _isDecl() {}

// Func

type Func struct {
	baseNode

	Attributes []*Attribute
	Public     bool

	Name_      *Leaf
	TypeParams []*TypeParam

	Receiver *Receiver // optional
	Params   []*Param
	VarArgs  bool

	Returns Type

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
		for _, param := range f.TypeParams {
			if !yield(param) {
				return
			}
		}
		if f.Receiver != nil && !yield(f.Receiver) {
			return
		}
		for _, param := range f.Params {
			if !yield(param) {
				return
			}
		}
		if !yield(f.Returns) {
			return
		}
		if !core.IsNil(f.Body) {
			yield(f.Body)
		}
	}
}

func (f *Func) Name() *Leaf {
	return f.Name_
}

func (f *Func) _isDecl() {}

func (f *Func) IsMethod() bool {
	_, ok := f.Parent().(*Impl)
	return ok
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
				name = f.Name_.Token.Text
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

func (f *Func) String(paramNames bool) string {
	var sb strings.Builder

	sb.WriteString("func ")
	sb.WriteString(f.Name_.Token.Text)
	sb.WriteString("(")

	hasParams := false

	if f.Receiver != nil {
		if f.Receiver.Mutable {
			sb.WriteString("mut self")
		} else {
			sb.WriteString("self")
		}

		hasParams = true
	}

	for _, param := range f.Params {
		if hasParams {
			sb.WriteString(", ")
		}

		if paramNames {
			sb.WriteString(param.Name.Token.Text)
			sb.WriteString(": ")
		}

		sb.WriteString(param.Type.String())

		hasParams = true
	}

	sb.WriteString(") ")
	sb.WriteString(f.Returns.String())

	return sb.String()
}

// Receiver

type Receiver struct {
	baseNode

	Mutable bool
}

func (r *Receiver) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

// Param

type Param struct {
	baseNode

	Name *Leaf
	Type Type
}

func (p *Param) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(p.Name) {
			return
		}
		yield(p.Type)
	}
}

// TypeParam

type TypeParam struct {
	baseNode

	Name        *Leaf
	Constraints []Type // optional
}

func (t *TypeParam) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(t.Name) {
			return
		}
		for _, c := range t.Constraints {
			if !yield(c) {
				return
			}
		}
	}
}

// Bad

type BadDecl struct {
	baseNode
}

func (b *BadDecl) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadDecl) Name() *Leaf {
	return nil
}

func (b *BadDecl) _isDecl() {}
