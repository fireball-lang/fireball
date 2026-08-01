package ir

import (
	"fireball/core"
	"unicode/utf8"
)

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
	Value core.Integer
}

func (i *Integer) Type() Type {
	return i.Typ
}

var False = &Integer{Typ: I1, Value: core.Signed(0)}
var True = &Integer{Typ: I1, Value: core.Signed(1)}

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
	Size  uint32
	Runes []rune

	NullTerminated bool
}

func NewString(runes []rune, nullTerminated bool) *String {
	size := uint32(0)

	for _, ch := range runes {
		chSize := utf8.RuneLen(ch)
		if chSize == -1 {
			chSize = utf8.RuneLen(utf8.RuneError)
		}

		size += uint32(chSize)
	}

	return &String{
		Size:           size,
		Runes:          runes,
		NullTerminated: nullTerminated,
	}
}

func (s *String) Type() Type {
	size := s.Size

	if s.NullTerminated {
		size++
	}

	return &ArrayType{
		Length:  size,
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

// Utils

func IsConstant(value Value) bool {
	switch value.(type) {
	case *ZeroInitializer, *Null, *Integer, *FloatV, *DoubleV, *String, *Vector, *Array, *Struct:
		return true
	case *GlobalVar, *Function:
		return true
	default:
		return false
	}
}
