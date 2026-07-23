package ir

import (
	"iter"
	"slices"
)

type Type interface {
	isIrType()
}

type StructLikeType interface {
	Type

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

type SimpleType struct {
	Kind SimpleKind
}

func (s SimpleType) isIrType() {}

var Void = &SimpleType{Kind: VoidKind}
var Float = &SimpleType{Kind: FloatKind}
var Double = &SimpleType{Kind: DoubleKind}
var Pointer = &SimpleType{Kind: PointerKind}

// Integer

type IntegerType struct {
	Bits uint8
}

func (i IntegerType) isIrType() {}

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

// Array

type ArrayType struct {
	Length  uint32
	Element Type
}

func (a ArrayType) isIrType() {}

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

func (r RefStructType) AllFields() iter.Seq2[int, Field] {
	return r.Struct.AllFields()
}

func (r RefStructType) Field(name string) (Type, int) {
	return r.Struct.Field(name)
}

// Utils

func IsAggregate(typ Type) bool {
	switch typ.(type) {
	case *ArrayType, *StructType, *RefStructType:
		return true

	default:
		return false
	}
}
