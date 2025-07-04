package ast

import (
	"fireball/lexer"
	"fmt"
	"iter"
	"slices"
	"strings"
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

func (p PrimitiveKind) BitCount() uint32 {
	switch p {
	case Void:
		return 0
	case Bool:
		return 1
	case U8, I8:
		return 8
	case U16, I16:
		return 16
	case U32, I32, F32:
		return 32
	case U64, I64, F64:
		return 64
	default:
		panic("ast.PrimitiveKind.BitCount() - Invalid primitive kind")
	}
}

func (p PrimitiveKind) IsInteger() bool {
	return p >= U8 && p <= I64
}

func (p PrimitiveKind) IsUnsignedInteger() bool {
	return p >= U8 && p <= U64
}

func (p PrimitiveKind) IsSignedInteger() bool {
	return p >= I8 && p <= I64
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

	Path *Path
	Decl Decl
}

func (d *DeclType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if d.Path != nil && !yield(d.Path) {
			return
		}
	}
}

func (d *DeclType) Range() lexer.Range {
	return d.Path.Range()
}

func (d *DeclType) Equals(other Type) bool {
	if other, ok := other.(*DeclType); ok {
		return IsValid(d.Decl) && d.Decl == other.Decl
	}

	return false
}

func (d *DeclType) String() string {
	var sb strings.Builder

	if d.Path != nil {
		for i, segment := range d.Path.Segments {
			if i > 0 {
				sb.WriteRune(':')
			}

			sb.WriteString(segment.Token.Text)
		}
	}

	return sb.String()
}

// ArrayType

type ArrayType struct {
	baseRangeNode

	Count   uint32
	Element Type
}

func (a *ArrayType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(a.Element) && !yield(a.Element) {
			return
		}
	}
}

func (a *ArrayType) Equals(other Type) bool {
	if other, ok := other.(*ArrayType); ok {
		return a.Count == other.Count && a.Element.Equals(other.Element)
	}

	return false
}

func (a *ArrayType) String() string {
	if IsValid(a.Element) {
		return fmt.Sprintf("[%d]%s", a.Count, a.Element.String())
	}

	return fmt.Sprintf("[%d]", a.Count)
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
	if IsValid(p.Pointee) {
		return "*" + p.Pointee.String()
	}

	return "*"
}

// FuncType

type FuncType interface {
	Type

	ParamTypes() iter.Seq[Type]
	VarArgs() bool

	ReturnType() Type
}

type SimpleFuncType struct {
	baseRangeNode

	Params   []Type
	VarArgs_ bool
	Returns  Type
}

func (s *SimpleFuncType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, param := range s.Params {
			if !yield(param) {
				return
			}
		}

		if IsValid(s.Returns) && !yield(s.Returns) {
			return
		}
	}
}

func (s *SimpleFuncType) Equals(other Type) bool {
	return funcTypeEquals(s, other)
}

func (s *SimpleFuncType) String() string {
	return funcTypeString(s, false)
}

func (s *SimpleFuncType) ParamTypes() iter.Seq[Type] {
	return slices.Values(s.Params)
}

func (s *SimpleFuncType) VarArgs() bool {
	return s.VarArgs_
}

func (s *SimpleFuncType) ReturnType() Type {
	if IsValid(s.Returns) {
		return s.Returns
	}

	return VoidType
}

func funcTypeEquals(f FuncType, other Type) bool {
	if other, ok := other.(FuncType); ok {
		aNext, aStop := iter.Pull(f.ParamTypes())
		bNext, bStop := iter.Pull(other.ParamTypes())

		defer aStop()
		defer bStop()

		for {
			aType, aValid := aNext()
			bType, bValid := bNext()

			if !aValid && !bValid {
				break
			}

			if !aValid || !bValid {
				return false
			}

			if !aType.Equals(bType) {
				return false
			}
		}

		if !f.ReturnType().Equals(other.ReturnType()) {
			return false
		}

		return f.VarArgs() == other.VarArgs()
	}

	return false
}

func funcTypeString(f FuncType, paramNames bool) string {
	var sb strings.Builder

	var params []*Param

	if fu, ok := f.(*Func); ok && paramNames {
		params = fu.Params
	}

	sb.WriteString("fn (")
	i := 0

	for param := range f.ParamTypes() {
		if i > 0 {
			sb.WriteString(", ")
		}

		if params != nil {
			name := params[i].Name

			if name != nil {
				sb.WriteString(name.Token.Text)
				sb.WriteRune(' ')
			}
		}

		sb.WriteString(param.String())
		i++
	}

	if f.VarArgs() {
		if i > 0 {
			sb.WriteString(", ...")
		} else {
			sb.WriteString("...")
		}
	}

	sb.WriteRune(')')

	if IsValid(f.ReturnType()) {
		sb.WriteRune(' ')
		sb.WriteString(f.ReturnType().String())
	}

	return sb.String()
}
