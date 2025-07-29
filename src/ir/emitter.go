package ir

import "fireball/lexer"

type Emitter struct {
	Module *Module

	scope []MetaRef

	loc    lexer.Pos
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

func (e *Emitter) SetDebugLocation(loc lexer.Pos) {
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
}

func (e *Emitter) Block() *Block {
	return e.block
}

func emit[T Instruction](e *Emitter, in T) T {
	in.SetMeta(e.GetLocMetaRef())
	e.block.AddLast(in)

	return in
}

type dummyInstruction struct {
	baseVoidInstruction
}

var dummy = &dummyInstruction{}

// Terminator instructions

func (e *Emitter) Ret(value Value) Instruction {
	if e.skip {
		return dummy
	}
	e.skip = true

	return emit(e, &Ret{
		Value: value,
	})
}

func (e *Emitter) Br(label *Block) Instruction {
	if e.skip {
		return dummy
	}
	e.skip = true

	return emit(e, &Br{
		Label: label,
	})
}

func (e *Emitter) BrCond(condition Value, ifTrue, ifFalse *Block) Instruction {
	if e.skip {
		return dummy
	}
	e.skip = true

	return emit(e, &BrCond{
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

	return emit(e, &FNeg{
		Value: value,
	})
}

// Binary instructions

func (e *Emitter) Add(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Add{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Sub(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Sub{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Mul(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Mul{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Div(kind DivKind, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Div{
		Kind:  kind,
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Rem(kind DivKind, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Rem{
		Kind:  kind,
		Left:  left,
		Right: right,
	})
}

// Bitwise binary instructions

func (e *Emitter) Shl(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Shl{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Shr(signExt bool, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Shr{
		SignExt: signExt,
		Left:    left,
		Right:   right,
	})
}

func (e *Emitter) And(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &And{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Or(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Or{
		Left:  left,
		Right: right,
	})
}

func (e *Emitter) Xor(left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Xor{
		Left:  left,
		Right: right,
	})
}

// Vector instructions

func (e *Emitter) ExtractElement(value, index Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &ExtractElement{
		Value: value,
		Index: index,
	})
}

func (e *Emitter) InsertElement(value, element, index Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &InsertElement{
		Value:   value,
		Element: element,
		Index:   index,
	})
}

func (e *Emitter) ShuffleVector(value1, value2, mask Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &ShuffleVector{
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

	return emit(e, &ExtractValue{
		Value:   value,
		Indices: indices,
	})
}

func (e *Emitter) InsertValue(value, element Value, indices ...uint32) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &InsertValue{
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

	return emit(e, &Alloca{
		Typ:   typ,
		Count: count,
	})
}

func (e *Emitter) Load(typ Type, pointer Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Load{
		Typ:     typ,
		Pointer: pointer,
	})
}

func (e *Emitter) Store(value, pointer Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Store{
		Value:   value,
		Pointer: pointer,
	})
}

func (e *Emitter) GetElementPtrConst(typ Type, pointer Value, pointerIndex, elementIndex uint32) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &GetElementPtrConst{
		Typ:          typ,
		Pointer:      pointer,
		PointerIndex: pointerIndex,
		ElementIndex: elementIndex,
	})
}

func (e *Emitter) GetElementPtrDyn(typ Type, pointer, pointerIndex, elementIndex Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &GetElementPtrDyn{
		Typ:          typ,
		Pointer:      pointer,
		PointerIndex: pointerIndex,
		ElementIndex: elementIndex,
	})
}

// Conversion instructions

func (e *Emitter) Trunc(value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Trunc{
		Value: value,
		Typ:   typ,
	})
}

func (e *Emitter) Ext(kind DivKind, value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Ext{
		Kind:  kind,
		Value: value,
		Typ:   typ,
	})
}

func (e *Emitter) FpToInt(signed bool, value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &FpToInt{
		Signed: signed,
		Value:  value,
		Typ:    typ,
	})
}

func (e *Emitter) IntToFp(signed bool, value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &IntToFp{
		Signed: signed,
		Value:  value,
		Typ:    typ,
	})
}

func (e *Emitter) PtrToInt(value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &PtrToInt{
		Value: value,
		Typ:   typ,
	})
}

func (e *Emitter) IntToPtr(value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &IntToPtr{
		Value: value,
		Typ:   typ,
	})
}

func (e *Emitter) BitCast(value Value, typ Type) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &BitCast{
		Value: value,
		Typ:   typ,
	})
}

// Other instructions

func (e *Emitter) ICmp(op CmpOp, signed bool, left, right Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &ICmp{
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

	return emit(e, &FCmp{
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

	return emit(e, &Phi{
		Pairs: pairs,
	})
}

func (e *Emitter) Select(condition, ifTrue, ifFalse Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Select{
		Condition: condition,
		IfTrue:    ifTrue,
		IfFalse:   ifFalse,
	})
}

func (e *Emitter) Call(typ Type, callee Value, args []Value) Instruction {
	if e.skip {
		return dummy
	}

	return emit(e, &Call{
		Typ:    typ.(*FunctionType),
		Callee: callee,
		Args:   args,
	})
}

// Debug instructions

func (e *Emitter) DbgDeclare(pointer Value, variableRef, locationRef MetaRef) Instruction {
	if e.skip {
		return dummy
	}

	return e.block.AddLast(&DbgDeclare{
		Pointer:     pointer,
		VariableRef: variableRef,
		LocationRef: locationRef,
	})
}
