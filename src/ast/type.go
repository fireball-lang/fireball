package ast

import (
	"fireball/lexer"
	"fireball/types"
	"fmt"
	"iter"
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

// Identifier

type IdentifierType struct {
	baseNode

	Mutable  bool
	Path     *IdentifierPath
	TypeArgs []Type
}

func (i *IdentifierType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(i.Path) {
			return
		}
		for _, arg := range i.TypeArgs {
			if !yield(arg) {
				return
			}
		}
	}
}

func (i *IdentifierType) String() string {
	if i.Mutable {
		return "mut " + i.Path.String()
	}

	return i.Path.String()
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
