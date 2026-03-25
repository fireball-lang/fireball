package types

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

type Primitive struct {
	Kind PrimitiveKind
}

func (p *Primitive) Equals(other Type) bool {
	if other, ok := other.(*Primitive); ok {
		return p.Kind == other.Kind
	}

	return false
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
