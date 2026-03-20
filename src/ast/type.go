package ast

import (
	"fireball/lexer"
	"iter"
)

type TypeVisitor interface {
	VisitPrimitiveType(p *PrimitiveType)
	VisitArrayType(a *ArrayType)
	VisitIdentifierType(i *IdentifierType)

	VisitBadType(b *BadType)
}

type Type interface {
	Node

	VisitType(visitor TypeVisitor)
}

// Primitive

type PrimitiveKind uint8

const (
	Void PrimitiveKind = iota
	Boolean

	U8
	U16
	U32
	U64

	I8
	I16
	I32
	I64

	F32
	F64
)

type PrimitiveType struct {
	baseNode

	Kind PrimitiveKind
}

func (p *PrimitiveType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (p *PrimitiveType) VisitType(visitor TypeVisitor) {
	visitor.VisitPrimitiveType(p)
}

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

func (a *ArrayType) VisitType(visitor TypeVisitor) {
	visitor.VisitArrayType(a)
}

// Identifier

type IdentifierType struct {
	baseLeafNode

	Token lexer.Token
}

func (i *IdentifierType) Range() lexer.Range {
	return i.Token.Range
}

func (i *IdentifierType) VisitType(visitor TypeVisitor) {
	visitor.VisitIdentifierType(i)
}

// Bad

type BadType struct {
	baseNode
}

func (b *BadType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadType) VisitType(visitor TypeVisitor) {
	visitor.VisitBadType(b)
}
