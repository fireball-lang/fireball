package ast

import (
	"fireball/lexer"
	"fireball/types"
	"iter"
)

type Type interface {
	Node

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

func (a *ArrayType) _isType() {}

// Pointer

type PointerType struct {
	baseNode

	Pointee Type
}

func (p *PointerType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		yield(p.Pointee)
	}
}

func (p *PointerType) _isType() {}

// Identifier

type IdentifierType struct {
	baseNode

	Path *IdentifierPath
}

func (i *IdentifierType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(i.Path) {
			return
		}
	}
}

func (i *IdentifierType) _isType() {}

// Bad

type BadType struct {
	baseNode
}

func (b *BadType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadType) _isType() {}
