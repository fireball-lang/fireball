package ast

import (
	"fireball/core"
	"fireball/lexer"
	"fireball/types"
	"iter"
)

type TypeVisitor interface {
	VisitPrimitiveType(p *PrimitiveType)
	VisitArrayType(a *ArrayType)
	VisitPointerType(p *PointerType)
	VisitIdentifierType(i *IdentifierType)

	VisitBadType(b *BadType)
}

type Type interface {
	Node

	VisitType(visitor TypeVisitor)
}

// Primitive

type PrimitiveType struct {
	baseNode

	Kind types.PrimitiveKind
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

func (p *PointerType) VisitType(visitor TypeVisitor) {
	visitor.VisitPointerType(p)
}

// Identifier

type IdentifierType struct {
	baseLeafNode

	Token lexer.Token
}

func (i *IdentifierType) Range() core.Range {
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
