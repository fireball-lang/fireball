package ir

import "fireball/utils"

type Value interface {
	Type() Type
}

type RuntimeValue interface {
	Value

	Meta() MetaRef
	SetMeta(ref MetaRef)
}

type baseRuntimeValue struct {
	meta MetaRef
}

func (b *baseRuntimeValue) Meta() MetaRef {
	return b.meta
}

func (b *baseRuntimeValue) SetMeta(ref MetaRef) {
	b.meta = ref
}

// Zero initializer

type ZeroInitializer struct {
	Typ Type
}

func (z *ZeroInitializer) Type() Type {
	return z.Typ
}

// Null

type Null struct{}

func (n *Null) Type() Type {
	return Pointer
}

// Integer

type Integer struct {
	Typ   Type
	Value utils.Integer
}

func (i *Integer) Type() Type {
	return i.Typ
}

var False = &Integer{Typ: I1, Value: utils.Signed(0)}
var True = &Integer{Typ: I1, Value: utils.Signed(1)}

// Float

type FloatV struct {
	Value float32
}

func (f *FloatV) Type() Type {
	return Float
}

// Double

type DoubleV struct {
	Value float64
}

func (d *DoubleV) Type() Type {
	return Double
}

// String

type String struct {
	Length uint32
	Value  string
}

func (s *String) Type() Type {
	return &ArrayType{
		Length:  s.Length,
		Element: I8,
	}
}

// Vector

type Vector struct {
	Elements []Value
}

func (v *Vector) Type() Type {
	return &VectorType{
		Length:  uint32(len(v.Elements)),
		Element: v.Elements[0].Type(),
	}
}

// Array

type Array struct {
	Elements []Value
}

func (a *Array) Type() Type {
	return &ArrayType{
		Length:  uint32(len(a.Elements)),
		Element: a.Elements[0].Type(),
	}
}

// Struct

type Struct struct {
	Typ    Type
	Fields []Value
}

func (s *Struct) Type() Type {
	return s.Typ
}
