package ast

import (
	"fireball/lexer"
	"fireball/types"
	"fmt"
	"iter"
	"strings"
)

type Type interface {
	Node

	String() string

	_isType()
}

// Primitive

type PrimitiveType struct {
	baseNode

	Kind types.PrimitiveKind
}

func (p *PrimitiveType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (p *PrimitiveType) String() string {
	return p.Kind.String()
}

func (p *PrimitiveType) _isType() {}

// Array

type ArrayType struct {
	baseNode

	Size lexer.Token
	Type Type
}

func (a *ArrayType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		yield(a.Type)
	}
}

func (a *ArrayType) String() string {
	return fmt.Sprintf("[%s]%s", a.Size.Text, a.Type)
}

func (a *ArrayType) _isType() {}

// Pointer

type PointerType struct {
	baseNode

	Mutable bool
	Pointee Type
}

func (p *PointerType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		yield(p.Pointee)
	}
}

func (p *PointerType) String() string {
	if p.Mutable {
		return "mut *" + p.Pointee.String()
	}

	return "*" + p.Pointee.String()
}

func (p *PointerType) _isType() {}

// Func

type FuncType struct {
	baseNode

	Params  []*Param
	VarArgs bool

	Returns Type
}

func (f *FuncType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, param := range f.Params {
			if !yield(param) {
				return
			}
		}
		if !yield(f.Returns) {
			return
		}
	}
}

func (f *FuncType) String() string {
	var sb strings.Builder

	sb.WriteString("func(")

	for i, param := range f.Params {
		if i > 0 {
			sb.WriteString(", ")
		}

		if param.Name != nil {
			sb.WriteString(param.Name.Token.Text)
			sb.WriteRune(' ')
		}

		sb.WriteString(param.Type.String())
	}

	sb.WriteString(") ")
	sb.WriteString(f.Returns.String())

	return sb.String()
}

func (f *FuncType) _isType() {}

// Identifier

type IdentifierType struct {
	baseNode

	Mutable bool
	Path    []*IdentifierEntry
}

func (i *IdentifierType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, entry := range i.Path {
			if !yield(entry) {
				return
			}
		}
	}
}

func (i *IdentifierType) String() string {
	var sb strings.Builder

	if i.Mutable {
		sb.WriteString("mut ")
	}

	for index, entry := range i.Path {
		if index > 0 {
			sb.WriteString("::")
		}

		sb.WriteString(entry.Name.Token.Text)

		if len(entry.TypeArgs) > 0 {
			sb.WriteString(":[")

			for _, arg := range entry.TypeArgs {
				sb.WriteString(arg.String())
			}

			sb.WriteRune(']')
		}
	}

	return sb.String()
}

func (i *IdentifierType) _isType() {}

// Self

type SelfType struct {
	baseNode
}

func (s *SelfType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (s *SelfType) String() string {
	return "Self"
}

func (s *SelfType) _isType() {}

// Option

type OptionType struct {
	baseNode

	Type Type
}

func (o *OptionType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(o.Type) {
			return
		}
	}
}

func (o *OptionType) String() string {
	return "?" + o.Type.String()
}

func (o *OptionType) _isType() {}

// Bad

type BadType struct {
	baseNode
}

func (b *BadType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadType) String() string {
	return "<invalid>"
}

func (b *BadType) _isType() {}
