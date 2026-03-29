package ir

type Type interface {
	isIrType()
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

type StructType struct {
	Packed bool
	Fields []Type
}

func (s StructType) isIrType() {}

// Ref

type RefStructType struct {
	Name   string
	Struct StructType
}

func (r RefStructType) isIrType() {}

// Utils

func IsAggregate(typ Type) bool {
	switch typ.(type) {
	case *ArrayType, *StructType, *RefStructType:
		return true

	default:
		return false
	}
}
