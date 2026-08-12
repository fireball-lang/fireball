package types

import (
	"fireball/core"
	"math"
)

type PrimitiveKind uint8

// values need to match with `core::PrimitiveKind`
const (
	Void PrimitiveKind = 0
	Bool               = 1

	U8  = 2
	U16 = 3
	U32 = 4
	U64 = 5

	I8  = 6
	I16 = 7
	I32 = 8
	I64 = 9

	F32 = 10
	F64 = 11
)

func IsUnsignedInteger(p PrimitiveKind) bool {
	return p >= U8 && p <= U64
}

func IsSignedInteger(p PrimitiveKind) bool {
	return p >= I8 && p <= I64
}

func IsInteger(p PrimitiveKind) bool {
	return p >= U8 && p <= I64
}

func IsFloating(p PrimitiveKind) bool {
	return p == F32 || p == F64
}

func IsSigned(p PrimitiveKind) bool {
	return p >= I8 && p <= F64
}

func IsNumeric(p PrimitiveKind) bool {
	return p >= U8 && p <= F64
}

func (p PrimitiveKind) Size() uint32 {
	switch p {
	case Void:
		return 0
	case Bool:
		return 1

	case U8:
		return 1
	case U16:
		return 2
	case U32:
		return 4
	case U64:
		return 8

	case I8:
		return 1
	case I16:
		return 2
	case I32:
		return 4
	case I64:
		return 8

	case F32:
		return 4
	case F64:
		return 8

	default:
		panic("types.PrimitiveKind.Size() - Invalid kind")
	}
}

func (p PrimitiveKind) IntegerRange() (core.Integer, core.Integer) {
	switch p {
	case U8:
		return core.Unsigned(false, 0), core.Unsigned(false, math.MaxUint8)
	case U16:
		return core.Unsigned(false, 0), core.Unsigned(false, math.MaxUint16)
	case U32:
		return core.Unsigned(false, 0), core.Unsigned(false, math.MaxUint32)
	case U64:
		return core.Unsigned(false, 0), core.Unsigned(false, math.MaxUint64)

	case I8:
		return core.Signed(math.MinInt8), core.Signed(math.MaxInt8)
	case I16:
		return core.Signed(math.MinInt16), core.Signed(math.MaxInt16)
	case I32:
		return core.Signed(math.MinInt32), core.Signed(math.MaxInt32)
	case I64:
		return core.Signed(math.MinInt64), core.Signed(math.MaxInt64)

	default:
		panic("types.PrimitiveKind.Size() - Invalid kind")
	}
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
		panic("types.PrimitiveKind.String() - Invalid kind")
	}
}

type Primitive struct {
	Kind PrimitiveKind
}

func (p *Primitive) Equals(other Type) bool {
	if other, ok := other.(*Primitive); ok {
		return p.Kind == other.Kind
	}

	return false
}

func (p *Primitive) String() string {
	return p.Kind.String()
}

var PrimitiveVoid = &Primitive{Void}
var PrimitiveBool = &Primitive{Bool}

var PrimitiveU8 = &Primitive{U8}
var PrimitiveU16 = &Primitive{U16}
var PrimitiveU32 = &Primitive{U32}
var PrimitiveU64 = &Primitive{U64}

var PrimitiveI8 = &Primitive{I8}
var PrimitiveI16 = &Primitive{I16}
var PrimitiveI32 = &Primitive{I32}
var PrimitiveI64 = &Primitive{I64}

var PrimitiveF32 = &Primitive{F32}
var PrimitiveF64 = &Primitive{F64}

func GetPrimitive(kind PrimitiveKind) *Primitive {
	switch kind {
	case Void:
		return PrimitiveVoid
	case Bool:
		return PrimitiveBool

	case U8:
		return PrimitiveU8
	case U16:
		return PrimitiveU16
	case U32:
		return PrimitiveU32
	case U64:
		return PrimitiveU64
	case I8:
		return PrimitiveI8
	case I16:
		return PrimitiveI16
	case I32:
		return PrimitiveI32
	case I64:
		return PrimitiveI64

	case F32:
		return PrimitiveF32
	case F64:
		return PrimitiveF64

	default:
		panic("types.GetPrimitive() - Invalid kind")
	}
}
