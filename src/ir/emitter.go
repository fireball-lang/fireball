package ir

import (
	"fireball/core"
	"fmt"
	"iter"
	"math"
	"reflect"
	"slices"
	"strings"
)

type Emitter struct {
	Module *Module

	scope []MetaRef

	loc    core.Pos
	locRef MetaRef

	block *Block
	skip  bool
}

func (e *Emitter) PushScope(ref MetaRef) {
	e.scope = append(e.scope, ref)
}

func (e *Emitter) PeekScope() MetaRef {
	return e.scope[len(e.scope)-1]
}

func (e *Emitter) PopScope() {
	e.scope = e.scope[:len(e.scope)-1]
}

func (e *Emitter) SetDebugLocation(loc core.Pos) {
	if loc.Line != 0 && loc.Column != 0 {
		e.loc = loc
		e.locRef = MetaRef(0)
	}
}

func (e *Emitter) GetLocMetaRef() MetaRef {
	if e.loc.Line == 0 && e.loc.Column == 0 {
		return MetaRef(0)
	}

	if !e.locRef.Valid() {
		e.locRef = e.Module.AddMeta(&LocationMeta{
			Scope:  e.PeekScope(),
			Line:   e.loc.Line,
			Column: e.loc.Column,
		})
	}

	return e.locRef
}

func (e *Emitter) Begin(block *Block) {
	e.block = block
	e.skip = false

	for inst := range block.Instructions() {
		switch inst.(type) {
		case *Ret, *Br, *BrCond:
			e.skip = true
			return
		}
	}
}

func (e *Emitter) Block() *Block {
	return e.block
}

func (e *Emitter) emit[T Instruction](in T) T {
	in.SetMeta(e.GetLocMetaRef())
	e.block.AddLast(in)

	return in
}

type dummyInstruction struct {
	baseVoidInstruction
}

func (d *dummyInstruction) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {}
}

var dummy = &dummyInstruction{}

// Terminator instructions

func (e *Emitter) Ret(value Value) Instruction {
	if e.skip {
		return dummy
	}
	e.skip = true

	return e.emit(&Ret{
		Value: value,
	})
}

func (e *Emitter) Br(label *Block) Instruction {
	if e.skip {
		return dummy
	}
	e.skip = true

	return e.emit(&Br{
		Label: label,
	})
}

func (e *Emitter) BrCond(condition Value, ifTrue, ifFalse *Block) Instruction {
	if e.skip {
		return dummy
	}
	e.skip = true

	assertIntegerType(condition.Type(), 1, 1)

	return e.emit(&BrCond{
		Condition: condition,
		IfTrue:    ifTrue,
		IfFalse:   ifFalse,
	})
}

// Unary instructions

func (e *Emitter) Fneg(value Value) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(value.Type(), FloatKind, DoubleKind)

	return e.emit(&FNeg{
		Value: value,
	})
}

// Binary instructions

func (e *Emitter) Add(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertExactType(left.Type(), right.Type())

	if isFloating(left.Type()) {
		return e.emit(&FAdd{
			Left:  left,
			Right: right,
		})
	}

	return e.emit(&IAdd{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Sub(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertExactType(left.Type(), right.Type())

	if isFloating(left.Type()) {
		return e.emit(&FSub{
			Left:  left,
			Right: right,
		})
	}

	return e.emit(&ISub{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Mul(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertExactType(left.Type(), right.Type())

	if isFloating(left.Type()) {
		return e.emit(&FMul{
			Left:  left,
			Right: right,
		})
	}

	return e.emit(&IMul{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Div(kind DivKind, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertExactType(left.Type(), right.Type())
	assertIntegerType(left.Type(), 0, 255)

	return e.emit(&IDiv{
		Kind:  kind,
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) FDiv(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertExactType(left.Type(), right.Type())
	assertSimpleType(left.Type(), FloatKind, DoubleKind)

	return e.emit(&FDiv{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Rem(kind DivKind, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertExactType(left.Type(), right.Type())
	assertIntegerType(left.Type(), 0, 255)

	return e.emit(&IRem{
		Kind:  kind,
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) FRem(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertExactType(left.Type(), right.Type())
	assertSimpleType(left.Type(), FloatKind, DoubleKind)

	return e.emit(&FRem{
		Left:  left,
		Right: right,
	})
}

// Bitwise binary instructions

func (e *Emitter) Shl(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(left.Type(), 0, 255)
	assertExactType(left.Type(), right.Type())

	return e.emit(&Shl{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Shr(signExt bool, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(left.Type(), 0, 255)
	assertExactType(left.Type(), right.Type())

	return e.emit(&Shr{
		SignExt: signExt,
		Left:    left,
		Right:   right,
	})
}

func (e *Emitter) And(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(left.Type(), 0, 255)
	assertExactType(left.Type(), right.Type())

	return e.emit(&And{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Or(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(left.Type(), 0, 255)
	assertExactType(left.Type(), right.Type())

	return e.emit(&Or{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Xor(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(left.Type(), 0, 255)
	assertExactType(left.Type(), right.Type())

	return e.emit(&Xor{
		Left:  left,
		Right: right,
	})
}

// Vector instructions

func (e *Emitter) ExtractElement(value, index Value) Instruction {
	if e.skip {
		return dummy
	}

	assertVectorType(value.Type())
	assertIntegerType(index.Type(), 0, 255)

	return e.emit(&ExtractElement{
		Value: value,
		Index: index,
	})
}

func (e *Emitter) InsertElement(value, element, index Value) Instruction {
	if e.skip {
		return dummy
	}

	assertVectorType(value.Type())
	assertExactType(value.Type().(*VectorType).Element, element.Type())
	assertIntegerType(index.Type(), 0, 255)

	return e.emit(&InsertElement{
		Value:   value,
		Element: element,
		Index:   index,
	})
}

func (e *Emitter) ShuffleVector(value1, value2, mask Value) Instruction {
	if e.skip {
		return dummy
	}

	assertVectorType(value1.Type())
	assertVectorType(value2.Type())
	assertVectorType(mask.Type())
	assertIntegerType(mask.Type().(*VectorType).Element, 32, 32)

	return e.emit(&ShuffleVector{
		Value1: value1,
		Value2: value2,
		Mask:   mask,
	})
}

// Aggregate instructions

func (e *Emitter) ExtractValue(value Value, indices ...uint32) Instruction {
	if e.skip {
		return dummy
	}

	assertAggregateType(value.Type(), false)

	return e.emit(&ExtractValue{
		Value:   value,
		Indices: indices,
	})
}

func (e *Emitter) InsertValue(value, element Value, indices ...uint32) Instruction {
	if e.skip {
		return dummy
	}

	assertAggregateType(value.Type(), false)

	return e.emit(&InsertValue{
		Value:   value,
		Element: element,
		Indices: indices,
	})
}

// Memory access and addressing instructions

func (e *Emitter) Alloca(typ Type, count uint32) Instruction {
	if e.skip {
		return dummy
	}

	return e.emit(&Alloca{
		Typ:   typ,
		Count: count,
	})
}

func (e *Emitter) Load(typ Type, pointer Value) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(pointer.Type(), PointerKind)

	return e.emit(&Load{
		Typ:     typ,
		Pointer: pointer,
	})
}

func (e *Emitter) Store(value, pointer Value) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(pointer.Type(), PointerKind)

	return e.emit(&Store{
		Value:   value,
		Pointer: pointer,
	})
}

func (e *Emitter) GetElementPtrConst(typ Type, pointer Value, indices ...uint32) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(pointer.Type(), PointerKind)
	if len(indices) > 4 {
		panic("ir.Emitter.GetElementPtrConst() - Can only have at most 4 indices")
	}

	gep := &GetElementPtrConst{
		Typ:     typ,
		Pointer: pointer,
	}

	copy(gep.Indices[:len(indices)], indices)

	for i := len(indices); i < 4; i++ {
		gep.Indices[i] = math.MaxUint32
	}

	return e.emit(gep)
}

func (e *Emitter) GetElementPtrDyn(typ Type, pointer Value, indices ...Value) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(pointer.Type(), PointerKind)
	if len(indices) > 4 {
		panic("ir.Emitter.GetElementPtrDyn() - Can only have at most 4 indices")
	}

	for _, index := range indices {
		assertIntegerType(index.Type(), 0, 255)
	}

	gep := GetElementPtrDyn{
		Typ:     typ,
		Pointer: pointer,
	}

	copy(gep.Indices[:len(indices)], indices)

	return e.emit(&gep)
}

// Conversion instructions

func (e *Emitter) Trunc(value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	if _, ok := typ.(*SimpleType); ok {
		assertSimpleType(value.Type(), FloatKind, DoubleKind)
		assertSimpleType(typ, FloatKind, DoubleKind)

		return e.emit(&FTrunc{
			Value: value,
			Typ:   typ,
		})
	}

	assertIntegerType(value.Type(), 0, 255)
	assertIntegerType(typ, 0, 255)

	return e.emit(&ITrunc{
		Value: value,
		Typ:   typ,
	})
}

func (e *Emitter) Ext(kind DivKind, value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(value.Type(), 0, 255)
	assertIntegerType(typ, 0, 255)

	return e.emit(&IExt{
		Kind:  kind,
		Value: value,
		Typ:   typ,
	})
}

func (e *Emitter) FExt(value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(value.Type(), FloatKind, DoubleKind)
	assertSimpleType(typ, FloatKind, DoubleKind)

	return e.emit(&FExt{
		Value: value,
		Typ:   typ,
	})
}

func (e *Emitter) FpToInt(signed bool, value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(value.Type(), FloatKind, DoubleKind)
	assertIntegerType(typ, 0, 255)

	return e.emit(&FpToInt{
		Signed: signed,
		Value:  value,
		Typ:    typ,
	})
}

func (e *Emitter) IntToFp(signed bool, value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(value.Type(), 0, 255)
	assertSimpleType(typ, FloatKind, DoubleKind)

	return e.emit(&IntToFp{
		Signed: signed,
		Value:  value,
		Typ:    typ,
	})
}

func (e *Emitter) PtrToInt(value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(value.Type(), PointerKind)
	assertIntegerType(typ, 64, 64)

	return e.emit(&PtrToInt{
		Value: value,
		Typ:   typ,
	})
}

func (e *Emitter) IntToPtr(value Value) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(value.Type(), 64, 64)

	return e.emit(&IntToPtr{
		Value: value,
	})
}

func (e *Emitter) BitCast(value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	assertAggregateType(value.Type(), true)
	assertAggregateType(typ, true)

	if value.Type().Info().Size != typ.Info().Size {
		panic("ir.Emitter.BitCast() - BitCast needs types with the same size, got " + reflect.TypeOf(value.Type()).String() + " and " + reflect.TypeOf(typ).String())
	}

	return e.emit(&BitCast{
		Value: value,
		Typ:   typ,
	})
}

// Other instructions

func (e *Emitter) ICmp(op CmpOp, signed bool, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerOrPointerType(left.Type(), 0, 255)
	assertIntegerOrPointerType(right.Type(), 0, 255)

	return e.emit(&ICmp{
		Op:     op,
		Signed: signed,
		Left:   left,
		Right:  right,
	})
}

func (e *Emitter) FCmp(op CmpOp, ordered bool, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(left.Type(), FloatKind, DoubleKind)
	assertExactType(left.Type(), right.Type())

	return e.emit(&FCmp{
		Op:      op,
		Ordered: ordered,
		Left:    left,
		Right:   right,
	})
}

func (e *Emitter) Phi(pairs ...PhiPair) Instruction {
	if e.skip {
		return dummy
	}

	typ := pairs[0].Value.Type()

	for _, pair := range pairs[1:] {
		assertExactType(typ, pair.Value.Type())
	}

	return e.emit(&Phi{
		Pairs: pairs,
	})
}

func (e *Emitter) Select(condition, ifTrue, ifFalse Value) Instruction {
	if e.skip {
		return dummy
	}

	assertIntegerType(condition.Type(), 1, 1)
	assertExactType(ifTrue.Type(), ifFalse.Type())

	return e.emit(&Select{
		Condition: condition,
		IfTrue:    ifTrue,
		IfFalse:   ifFalse,
	})
}

func (e *Emitter) Call(sig *Signature, callee Value, args []Value) Instruction {
	if e.skip {
		return dummy
	}

	if _, ok := callee.(*Assembly); !ok {
		assertSimpleType(callee.Type(), PointerKind)
	}

	return e.emit(&Call{
		Signature: sig,
		Callee:    callee,
		Args:      args,
	})
}

// Debug instructions

func (e *Emitter) DbgDeclare(pointer Value, variableRef, locationRef MetaRef) Instruction {
	if e.skip {
		return dummy
	}

	assertSimpleType(pointer.Type(), PointerKind)

	return e.block.AddLast(&DbgDeclare{
		Pointer:     pointer,
		VariableRef: variableRef,
		LocationRef: locationRef,
	})
}

// Utils

func isFloating(typ Type) bool {
	if typ, ok := typ.(*SimpleType); ok {
		return typ.Kind == FloatKind || typ.Kind == DoubleKind
	}

	return false
}

func assertSimpleType(typ Type, kinds ...SimpleKind) {
	if typ, ok := typ.(*SimpleType); !ok || !slices.Contains(kinds, typ.Kind) {
		var sb strings.Builder

		for i, kind := range kinds {
			if i > 0 {
				sb.WriteString(" or a ")
			}
			sb.WriteString(kind.String())
		}

		panic("ir.Emitter.() - Required a " + sb.String() + ", got " + reflect.TypeOf(typ).String())
	}
}

func assertIntegerType(typ Type, minBits, maxBits uint8) {
	if typ, ok := typ.(*IntegerType); !ok || typ.Bits < minBits || typ.Bits > maxBits {
		panic(fmt.Sprintf("ir.Emitter.() - Required a i%d - i%d, got %s", minBits, maxBits, reflect.TypeOf(typ).String()))
	}
}

func assertIntegerOrPointerType(typ Type, minBits, maxBits uint8) {
	var bits uint8

	switch typ := typ.(type) {
	case *IntegerType:
		bits = typ.Bits
	case *SimpleType:
		if typ.Kind == PointerKind {
			bits = 64
		}
	}

	if bits == 0 || bits < minBits || bits > maxBits {
		panic(fmt.Sprintf("ir.Emitter.() - Required a i%d - i%d (or a pointer), got %s", minBits, maxBits, reflect.TypeOf(typ).String()))
	}
}

func assertVectorType(typ Type) {
	if _, ok := typ.(*VectorType); !ok {
		panic("ir.Emitter.() - Required a vector, got " + reflect.TypeOf(typ).String())
	}
}

func assertAggregateType(typ Type, negate bool) {
	ok := false

	switch typ.(type) {
	case *ArrayType, *StructType, *RefStructType:
		ok = true
	}

	if negate && ok {
		panic("ir.Emitter.() - Required a non-aggregate, got " + reflect.TypeOf(typ).String())
	} else if !negate && !ok {
		panic("ir.Emitter.() - Required an aggregate, got " + reflect.TypeOf(typ).String())
	}
}

func assertExactType(typ1, typ2 Type) {
	if !typ1.Equals(typ2) {
		panic("ir.Emitter.() - Required exactly the same types, got " + reflect.TypeOf(typ1).String() + " and " + reflect.TypeOf(typ2).String())
	}
}
