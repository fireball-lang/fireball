package ast

import (
	"fireball/lexer"
	"iter"
)

type Type interface {
	Node

	Equals(other Type) bool
	String() string
}

// PrimitiveType

type PrimitiveKind uint8

const (
	Void PrimitiveKind = iota
	Bool
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

func (p PrimitiveKind) IsInteger() bool {
	return p >= U8 && p <= I64
}

func (p PrimitiveKind) IsFloating() bool {
	return p >= F32 && p <= F64
}

func (p PrimitiveKind) IsNumeric() bool {
	return p >= U8 && p <= F64
}

func (p PrimitiveKind) String() string {
	switch p {
	case Void:
		return "void"
	case Bool:
		return "bool"
	case U8:
		return "u8"
	case U16:
		return "u16"
	case U32:
		return "u32"
	case U64:
		return "u64"
	case I8:
		return "i8"
	case I16:
		return "i16"
	case I32:
		return "i32"
	case I64:
		return "i64"
	case F32:
		return "f32"
	case F64:
		return "f64"
	default:
		panic("ast.PrimitiveKind.String() - Invalid")
	}
}

type PrimitiveType struct {
	baseRangeNode

	Kind PrimitiveKind
}

func (p *PrimitiveType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (p *PrimitiveType) Equals(other Type) bool {
	if other, ok := other.(*PrimitiveType); ok {
		return p.Kind == other.Kind
	}

	return false
}

func (p *PrimitiveType) String() string {
	return p.Kind.String()
}

var VoidType = &PrimitiveType{Kind: Void}
var BoolType = &PrimitiveType{Kind: Bool}
var U8Type = &PrimitiveType{Kind: U8}
var U16Type = &PrimitiveType{Kind: U16}
var U32Type = &PrimitiveType{Kind: U32}
var U64Type = &PrimitiveType{Kind: U64}
var I8Type = &PrimitiveType{Kind: I8}
var I16Type = &PrimitiveType{Kind: I16}
var I32Type = &PrimitiveType{Kind: I32}
var I64Type = &PrimitiveType{Kind: I64}
var F32Type = &PrimitiveType{Kind: F32}
var F64Type = &PrimitiveType{Kind: F64}

// DeclType

type DeclType struct {
	baseNode

	Name *Leaf
	Decl Decl
}

func (d *DeclType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if d.Name != nil && !yield(d.Name) {
			return
		}
	}
}

func (d *DeclType) Range() lexer.Range {
	return d.Name.Range()
}

func (d *DeclType) Equals(other Type) bool {
	if other, ok := other.(*DeclType); ok {
		return d.Name == other.Name
	}

	return false
}

func (d *DeclType) String() string {
	return d.Name.Token.Text
}

// PointerType

type PointerType struct {
	baseRangeNode

	Pointee Type
}

func (p *PointerType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(p.Pointee) && !yield(p.Pointee) {
			return
		}
	}
}

func (p *PointerType) Equals(other Type) bool {
	if other, ok := other.(*PointerType); ok {
		return p.Pointee.Equals(other.Pointee)
	}

	return false
}

func (p *PointerType) String() string {
	return "*" + p.Pointee.String()
}
