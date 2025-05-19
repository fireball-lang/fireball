package llvm

import (
	"math"
	"strconv"
)

type Value interface {
	Type() Type
	String() string
}

// Identifier

type Identifier struct {
	number uint32
	name   string
}

func (i Identifier) String() string {
	if i.number != math.MaxUint32 {
		return "%" + strconv.FormatUint(uint64(i.number), 10)
	}

	return "%" + i.name
}

type IdentifierValue struct {
	type_      Type
	identifier Identifier
}

func (i IdentifierValue) Type() Type {
	return i.type_
}

func (i IdentifierValue) String() string {
	return i.identifier.String()
}

// Global

type GlobalValue struct {
	name  string
	type_ Type
}

func (e *GlobalValue) Type() Type {
	return e.type_
}

func (e *GlobalValue) String() string {
	return "@" + e.name
}

// Int

type IntValue struct {
	type_ Type

	negative bool
	value    uint64
}

func Int(type_ Type, value int64) IntValue {
	if t, ok := type_.(*integerType); !ok || !t.signed {
		panic("llvm.Int() - Invalid type")
	}

	return IntValue{
		type_:    type_,
		negative: value < 0,
		value:    uint64(value),
	}
}

func Uint(type_ Type, value uint64) IntValue {
	if t, ok := type_.(*integerType); !ok || t.signed {
		panic("llvm.Uint() - Invalid type")
	}

	return IntValue{
		type_:    type_,
		negative: false,
		value:    value,
	}
}

func (i IntValue) Type() Type {
	return i.type_
}

func (i IntValue) String() string {
	if i.negative {
		return "-" + strconv.FormatUint(i.value, 10)
	}

	return strconv.FormatUint(i.value, 10)
}

// Float

type FloatingValue struct {
	type_ Type
	value float64
}

func Float(value float32) FloatingValue {
	return FloatingValue{type_: F32, value: float64(value)}
}

func Double(value float64) FloatingValue {
	return FloatingValue{type_: F64, value: value}
}

func (f FloatingValue) Type() Type {
	return f.type_
}

func (f FloatingValue) String() string {
	bits := math.Float64bits(f.value)
	return "0x" + strconv.FormatUint(bits, 16)
}

// Bool

func True() IntValue {
	return IntValue{type_: I1, value: 1}
}

func False() IntValue {
	return IntValue{type_: I1, value: 0}
}
