package ir

import (
	"iter"
	"slices"
)

type TypeInfo struct {
	Size  uint32
	Align uint32
}

type Type interface {
	isIrType()

	Info() TypeInfo
	Equals(other Type) bool
}

type StructLikeType interface {
	Type

	IsPacked() bool

	AllFields() iter.Seq2[int, Field]
	Field(name string) (Type, int)
}

// Simple

type SimpleKind uint8

const (
	VoidKind SimpleKind = iota
	FloatKind
	DoubleKind
	PointerKind
)

func (s SimpleKind) String() string {
	switch s {
	case VoidKind:
		return "void"
	case FloatKind:
		return "float"
	case DoubleKind:
		return "double"
	case PointerKind:
		return "pointer"

	default:
		panic("ir.SimpleKind.String() - Invalid kind")
	}
}

type SimpleType struct {
	Kind SimpleKind
}

func (s SimpleType) isIrType() {}

func (s SimpleType) Info() TypeInfo {
	switch s.Kind {
	case VoidKind:
		return TypeInfo{}
	case FloatKind:
		return TypeInfo{Size: 4, Align: 4}
	case DoubleKind:
		return TypeInfo{Size: 8, Align: 8}
	case PointerKind:
		return TypeInfo{Size: 8, Align: 8}

	default:
		panic("ir.SimpleType.Info() - Invalid kind")
	}
}

func (s SimpleType) Equals(other Type) bool {
	if other, ok := other.(*SimpleType); ok {
		return s.Kind == other.Kind
	}

	return false
}

var Void = &SimpleType{Kind: VoidKind}
var Float = &SimpleType{Kind: FloatKind}
var Double = &SimpleType{Kind: DoubleKind}
var Pointer = &SimpleType{Kind: PointerKind}

// Integer

type IntegerType struct {
	Bits uint8
}

func (i IntegerType) isIrType() {}

func (i IntegerType) Info() TypeInfo {
	bytes := 1 + (uint32(i.Bits)-1)/8
	return TypeInfo{Size: bytes, Align: max(bytes, 1)}
}

func (i IntegerType) Equals(other Type) bool {
	if other, ok := other.(*IntegerType); ok {
		return i.Bits == other.Bits
	}

	return false
}

var I1 = &IntegerType{Bits: 1}
var I8 = &IntegerType{Bits: 8}
var I16 = &IntegerType{Bits: 16}
var I32 = &IntegerType{Bits: 32}
var I64 = &IntegerType{Bits: 64}

// Vector

type VectorType struct {
	Length  uint32
	Element Type
}

func (v VectorType) isIrType() {}

func (v VectorType) Info() TypeInfo {
	info := v.Element.Info()
	return TypeInfo{Size: info.Size * v.Length, Align: info.Align}
}

func (v VectorType) Equals(other Type) bool {
	if other, ok := other.(*VectorType); ok {
		return v.Length == other.Length && v.Element.Equals(other.Element)
	}

	return false
}

// Array

type ArrayType struct {
	Length  uint32
	Element Type
}

func (a ArrayType) isIrType() {}

func (a ArrayType) Info() TypeInfo {
	info := a.Element.Info()
	return TypeInfo{Size: info.Size * a.Length, Align: info.Align}
}

func (a ArrayType) Equals(other Type) bool {
	if other, ok := other.(*ArrayType); ok {
		return a.Length == other.Length && a.Element.Equals(other.Element)
	}

	return false
}

// Struct

type Field struct {
	Name string
	Type Type
}

type StructType struct {
	Packed bool
	Fields []Field
}

func (s StructType) isIrType() {}

func (s StructType) Info() TypeInfo {
	info := TypeInfo{Align: 1}

	if s.Packed {
		for _, field := range s.Fields {
			info.Size += field.Type.Info().Size
		}

		return info
	}

	for _, field := range s.Fields {
		fieldInfo := field.Type.Info()

		info.Size = alignTo(info.Size, fieldInfo.Align)
		info.Size += fieldInfo.Size
		info.Align = max(info.Align, fieldInfo.Align)
	}

	info.Size = alignTo(info.Size, info.Align)

	return info
}

func (s StructType) Equals(other Type) bool {
	switch other := other.(type) {
	case *StructType:
		return structsEquals(&s, other)
	case *RefStructType:
		return structsEquals(&s, &other.Struct)

	default:
		return false
	}
}

func (s StructType) IsPacked() bool {
	return s.Packed
}

func (s StructType) AllFields() iter.Seq2[int, Field] {
	return slices.All(s.Fields)
}

func (s StructType) Field(name string) (Type, int) {
	for i, field := range s.Fields {
		if field.Name == name {
			return field.Type, i
		}
	}

	return nil, -1
}

// Ref

type RefStructType struct {
	Name   string
	Struct StructType
}

func (r RefStructType) isIrType() {}

func (r RefStructType) Info() TypeInfo {
	return r.Struct.Info()
}

func (r RefStructType) Equals(other Type) bool {
	switch other := other.(type) {
	case *StructType:
		return structsEquals(&r.Struct, other)
	case *RefStructType:
		return structsEquals(&r.Struct, &other.Struct)

	default:
		return false
	}
}

func (r RefStructType) IsPacked() bool {
	return r.Struct.Packed
}

func (r RefStructType) AllFields() iter.Seq2[int, Field] {
	return r.Struct.AllFields()
}

func (r RefStructType) Field(name string) (Type, int) {
	return r.Struct.Field(name)
}

// Utils

func structsEquals(s1, s2 *StructType) bool {
	if s1.Packed != s2.Packed || len(s1.Fields) != len(s2.Fields) {
		return false
	}

	for i, field := range s1.Fields {
		if !field.Type.Equals(s2.Fields[i].Type) {
			return false
		}
	}

	return true
}

func alignTo(num, align uint32) uint32 {
	if num%align != 0 {
		num += align - (num % align)
	}

	return num
}

func IsAggregate(typ Type) bool {
	switch typ.(type) {
	case *VectorType, *ArrayType, *StructType, *RefStructType:
		return true

	default:
		return false
	}
}
