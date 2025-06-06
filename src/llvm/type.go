package llvm

import (
	"fmt"
	"strconv"
	"strings"
)

type Type interface {
	write(sb *strings.Builder)
	String() string

	Size() uint32
	Align() uint32

	debugIndex() uint32
}

type baseType struct {
	size_  uint32
	align_ uint32

	dbg uint32
}

func (b *baseType) Size() uint32 {
	return b.size_
}

func (b *baseType) Align() uint32 {
	return b.align_
}

func (b *baseType) debugIndex() uint32 {
	return b.dbg
}

// Simple

type simpleType struct {
	baseType
	text string
}

var F32 = &simpleType{baseType: baseType{size_: 32, align_: 32}, text: "float"}
var F64 = &simpleType{baseType: baseType{size_: 64, align_: 64}, text: "double"}

func (s *simpleType) write(sb *strings.Builder) {
	sb.WriteString(s.text)
}

func (s *simpleType) String() string {
	return s.text
}

// Integer

type integerType struct {
	baseType
	signed   bool
	bitCount uint32
}

var I1 = &integerType{baseType: baseType{size_: 1, align_: 1}, signed: true, bitCount: 1}
var I8 = &integerType{baseType: baseType{size_: 8, align_: 8}, signed: true, bitCount: 8}
var I16 = &integerType{baseType: baseType{size_: 16, align_: 16}, signed: true, bitCount: 16}
var I32 = &integerType{baseType: baseType{size_: 32, align_: 32}, signed: true, bitCount: 32}
var I64 = &integerType{baseType: baseType{size_: 64, align_: 64}, signed: true, bitCount: 64}

var U1 = &integerType{baseType: baseType{size_: 1, align_: 1}, signed: false, bitCount: 1}
var U8 = &integerType{baseType: baseType{size_: 8, align_: 8}, signed: false, bitCount: 8}
var U16 = &integerType{baseType: baseType{size_: 16, align_: 16}, signed: false, bitCount: 16}
var U32 = &integerType{baseType: baseType{size_: 32, align_: 32}, signed: false, bitCount: 32}
var U64 = &integerType{baseType: baseType{size_: 64, align_: 64}, signed: false, bitCount: 64}

func (s *integerType) write(sb *strings.Builder) {
	var buffer [8]byte
	bitCount := strconv.AppendUint(buffer[0:0], uint64(s.bitCount), 10)

	sb.WriteRune('i')
	sb.Write(bitCount)
}

func (s *integerType) String() string {
	return fmt.Sprintf("i%d", s.bitCount)
}

// Array

type arrayType struct {
	baseType
	count   uint32
	element Type
}

func (a *arrayType) write(sb *strings.Builder) {
	var buffer [8]byte
	count := strconv.AppendUint(buffer[0:0], uint64(a.count), 10)

	sb.WriteRune('[')
	sb.Write(count)
	sb.WriteString(" x ")
	a.element.write(sb)
	sb.WriteRune(']')
}

func (a *arrayType) String() string {
	sb := strings.Builder{}
	a.write(&sb)

	return sb.String()
}

// Pointer

type pointerType struct {
	baseType
	pointee Type
}

func (p *pointerType) write(sb *strings.Builder) {
	sb.WriteString("ptr")
}

func (p *pointerType) String() string {
	return "ptr"
}

// Struct

type Field struct {
	Name   string
	Type   Type
	Offset uint32
}

type structType struct {
	baseType
	name   string
	fields []Field
}

func (s *structType) write(sb *strings.Builder) {
	sb.WriteString("%struct.")
	sb.WriteString(s.name)
}

func (s *structType) String() string {
	return "%struct." + s.name
}

// Function

type functionType struct {
	baseType
	returns Type
	params  []Type
	vararg  bool
}

func (f *functionType) write(sb *strings.Builder) {
	f.returns.write(sb)
	sb.WriteString(" (")

	for i, param := range f.params {
		if i > 0 {
			sb.WriteString(", ")
		}

		param.write(sb)
	}

	if f.vararg {
		if len(f.params) > 0 {
			sb.WriteString(", ...")
		} else {
			sb.WriteString("...")
		}
	}

	sb.WriteRune(')')
}

func (f *functionType) String() string {
	sb := strings.Builder{}
	f.write(&sb)

	return sb.String()
}

// Utils

func getFunctionType(t Type) *functionType {
	if p, ok := t.(*pointerType); ok {
		t = p.pointee
	}

	if f, ok := t.(*functionType); ok {
		return f
	}

	panic("llvm.getFunctionType() - Type is not a function or a pointer to a function")
}
