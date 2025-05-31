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

func NamedIdentifier(name string) Identifier {
	return Identifier{number: math.MaxUint32, name: name}
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

func NamedIdentifierValue(type_ Type, name string) IdentifierValue {
	return IdentifierValue{
		type_:      type_,
		identifier: Identifier{number: math.MaxUint32, name: name},
	}
}

func (i IdentifierValue) Type() Type {
	return i.type_
}

func (i IdentifierValue) String() string {
	return i.identifier.String()
}

// Zero initializer

type ZeroInitializer struct {
	type_ Type
}

func ZeroInitialize(type_ Type) ZeroInitializer {
	return ZeroInitializer{type_: type_}
}

func (z ZeroInitializer) Type() Type {
	return z.type_
}

func (z ZeroInitializer) String() string {
	return "zeroinitializer"
}

// Global

type GlobalValue struct {
	name  string
	type_ Type
}

func (g *GlobalValue) Type() Type {
	return g.type_
}

func (g *GlobalValue) String() string {
	return "@" + g.name
}

// ExternFunc

type ExternFunction struct {
	name  string
	type_ Type
}

func FakeFunctionValue(type_ Type, name string) ExternFunction {
	return ExternFunction{
		name:  name,
		type_: type_,
	}
}

func (e *ExternFunction) Type() Type {
	return e.type_
}

func (e *ExternFunction) String() string {
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

// Null

var nullType = &pointerType{
	baseType: baseType{
		size_:  64,
		align_: 64,
		dbg:    math.MaxUint32,
	},
	pointee: nil,
}

type NullValue struct {
}

func (n NullValue) Type() Type {
	return nullType
}

func (n NullValue) String() string {
	return "null"
}

func Null() NullValue {
	return NullValue{}
}

// TypedWrapper

type TypedWrapper struct {
	type_ Type
	value Value
}

func (t TypedWrapper) Type() Type {
	return t.type_
}

func (t TypedWrapper) String() string {
	return t.value.String()
}

func ChangeValueType(value Value, newType Type) TypedWrapper {
	return TypedWrapper{
		type_: newType,
		value: value,
	}
}
