package codegen

import (
	"fireball/ir"
	"fireball/lexer"
	"fireball/utils"
)

type Emitter struct {
	Ir ir.Emitter
}

func (e *Emitter) PushScope(ref ir.MetaRef) {
	e.Ir.PushScope(ref)
}

func (e *Emitter) PeekScope() ir.MetaRef {
	return e.Ir.PeekScope()
}

func (e *Emitter) PopScope() {
	e.Ir.PopScope()
}

func (e *Emitter) SetDebugLocation(loc lexer.Pos) {
	e.Ir.SetDebugLocation(loc)
}

func (e *Emitter) GetLocMetaRef() ir.MetaRef {
	return e.Ir.GetLocMetaRef()
}

func (e *Emitter) Begin(block *ir.Block) {
	e.Ir.Begin(block)
}

// Terminator instructions

func (e *Emitter) Ret(value Value) {
	e.Ir.Ret(value.Ir)
}

func (e *Emitter) Br(label *ir.Block) {
	e.Ir.Br(label)
}

func (e *Emitter) BrCond(condition Value, ifTrue, ifFalse *ir.Block) {
	e.Ir.BrCond(condition.Ir, ifTrue, ifFalse)
}

// Unary instructions

func (e *Emitter) Fneg(value Value) Value {
	return toValue(e.Ir.Fneg(value.Ir))
}

// Binary instructions

func (e *Emitter) Add(left, right Value) Value {
	return toValue(e.Ir.Add(left.Ir, right.Ir))
}

func (e *Emitter) Sub(left, right Value) Value {
	return toValue(e.Ir.Sub(left.Ir, right.Ir))
}

func (e *Emitter) Mul(left, right Value) Value {
	return toValue(e.Ir.Mul(left.Ir, right.Ir))
}

func (e *Emitter) Div(kind ir.DivKind, left, right Value) Value {
	return toValue(e.Ir.Div(kind, left.Ir, right.Ir))
}

func (e *Emitter) Rem(kind ir.DivKind, left, right Value) Value {
	return toValue(e.Ir.Rem(kind, left.Ir, right.Ir))
}

// Bitwise binary instructions

func (e *Emitter) Shl(left, right Value) Value {
	return toValue(e.Ir.Shl(left.Ir, right.Ir))
}

func (e *Emitter) Shr(signExt bool, left, right Value) Value {
	return toValue(e.Ir.Shr(signExt, left.Ir, right.Ir))
}

func (e *Emitter) And(left, right Value) Value {
	return toValue(e.Ir.And(left.Ir, right.Ir))
}

func (e *Emitter) Or(left, right Value) Value {
	return toValue(e.Ir.Or(left.Ir, right.Ir))
}

func (e *Emitter) Xor(left, right Value) Value {
	return toValue(e.Ir.Xor(left.Ir, right.Ir))
}

// Vector instructions

func (e *Emitter) ExtractElement(value, index Value) Value {
	return toValue(e.Ir.ExtractElement(value.Ir, index.Ir))
}

func (e *Emitter) InsertElement(value, element, index Value) Value {
	return toValue(e.Ir.InsertElement(value.Ir, element.Ir, index.Ir))
}

func (e *Emitter) ShuffleVector(value1, value2, mask Value) Value {
	return toValue(e.Ir.ShuffleVector(value1.Ir, value2.Ir, mask.Ir))
}

// Aggregate instructions

func (e *Emitter) ExtractValue(value Value, indices ...uint32) Value {
	return toValue(e.Ir.ExtractValue(value.Ir, indices...))
}

func (e *Emitter) InsertValue(value, element Value, indices ...uint32) Value {
	return toValue(e.Ir.InsertValue(value.Ir, element.Ir, indices...))
}

// Memory access and addressing instructions

func (e *Emitter) Alloca(typ ir.Type, count uint32) Value {
	return Value{
		Ir:          e.Ir.Alloca(typ, count),
		Typ:         typ,
		Addressable: true,
	}
}

func (e *Emitter) Load(typ ir.Type, pointer Value) Value {
	return toValue(e.Ir.Load(typ, pointer.Ir))
}

func (e *Emitter) Store(value, pointer Value) {
	e.Ir.Store(value.Ir, pointer.Ir)
}

func (e *Emitter) GetElementPtrConst(typ ir.Type, pointer Value, pointerIndex, elementIndex uint32) Value {
	var elementTyp ir.Type

	switch typ := typ.(type) {
	case *ir.VectorType:
		elementTyp = typ.Element
	case *ir.ArrayType:
		elementTyp = typ.Element
	case *ir.StructType:
		elementTyp = typ.Fields[elementIndex]
	case *ir.RefStructType:
		elementTyp = typ.Struct.Fields[elementIndex]
	default:
		panic("codegen.Emitter.GetElementPtrConst() - Invalid typ")
	}

	return Value{
		Ir:          e.Ir.GetElementPtrConst(typ, pointer.Ir, pointerIndex, elementIndex),
		Typ:         elementTyp,
		Addressable: true,
	}
}

func (e *Emitter) GetElementPtrDyn(typ ir.Type, pointer, pointerIndex, elementIndex Value) Value {
	var elementTyp ir.Type

	if utils.IsNil(elementIndex.Ir) {
		elementTyp = typ
	} else {
		switch typ := typ.(type) {
		case *ir.VectorType:
			elementTyp = typ.Element
		case *ir.ArrayType:
			elementTyp = typ.Element
		default:
			panic("codegen.Emitter.GetElementPtrConst() - Invalid typ")
		}
	}

	return Value{
		Ir:          e.Ir.GetElementPtrDyn(typ, pointer.Ir, pointerIndex.Ir, elementIndex.Ir),
		Typ:         elementTyp,
		Addressable: true,
	}
}

// Conversion instructions

func (e *Emitter) Trunc(value Value, typ ir.Type) Value {
	return toValue(e.Ir.Trunc(value.Ir, typ))
}

func (e *Emitter) Ext(kind ir.DivKind, value Value, typ ir.Type) Value {
	return toValue(e.Ir.Ext(kind, value.Ir, typ))
}

func (e *Emitter) FpToInt(signed bool, value Value, typ ir.Type) Value {
	return toValue(e.Ir.FpToInt(signed, value.Ir, typ))
}

func (e *Emitter) IntToFp(signed bool, value Value, typ ir.Type) Value {
	return toValue(e.Ir.IntToFp(signed, value.Ir, typ))
}

func (e *Emitter) PtrToInt(value Value, typ ir.Type) Value {
	return toValue(e.Ir.PtrToInt(value.Ir, typ))
}

func (e *Emitter) IntToPtr(value Value, typ ir.Type) Value {
	return toValue(e.Ir.IntToPtr(value.Ir, typ))
}

func (e *Emitter) BitCast(value Value, typ ir.Type) Value {
	return toValue(e.Ir.BitCast(value.Ir, typ))
}

// Other instructions

func (e *Emitter) ICmp(op ir.CmpOp, signed bool, left, right Value) Value {
	return toValue(e.Ir.ICmp(op, signed, left.Ir, right.Ir))
}

func (e *Emitter) FCmp(op ir.CmpOp, ordered bool, left, right Value) Value {
	return toValue(e.Ir.FCmp(op, ordered, left.Ir, right.Ir))
}

func (e *Emitter) Phi(pairs ...ir.PhiPair) Value {
	return toValue(e.Ir.Phi(pairs...))
}

func (e *Emitter) Select(condition, ifTrue, ifFalse Value) Value {
	return toValue(e.Ir.Select(condition.Ir, ifTrue.Ir, ifFalse.Ir))
}

func (e *Emitter) Call(typ ir.Type, callee Value, args []ir.Value) Value {
	return toValue(e.Ir.Call(typ, callee.Ir, args))
}

// Debug instructions

func (e *Emitter) DbgDeclare(pointer Value, variableRef, locationRef ir.MetaRef) {
	e.Ir.DbgDeclare(pointer.Ir, variableRef, locationRef)
}
