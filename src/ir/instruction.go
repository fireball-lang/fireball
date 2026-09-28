package ir

import (
	"fireball/core"
	"iter"
)

type Instruction interface {
	RuntimeValue

	Name() string
	SetName(name string)

	Values() iter.Seq[Value]

	Block() *Block
	setBlock(b *Block)

	Next() Instruction
	setNext(in Instruction)
}

type baseInstruction struct {
	baseRuntimeValue

	type_ Type
	name  string

	block  *Block
	nextIn Instruction
}

func (b *baseInstruction) Name() string {
	return b.name
}

func (b *baseInstruction) SetName(name string) {
	b.name = name
}

func (b *baseInstruction) Block() *Block {
	return b.block
}

func (b *baseInstruction) setBlock(block *Block) {
	b.block = block
}

func (b *baseInstruction) Next() Instruction {
	return b.nextIn
}

func (b *baseInstruction) setNext(i Instruction) {
	b.nextIn = i
}

type baseVoidInstruction struct {
	baseInstruction
}

func (b *baseVoidInstruction) Type() Type {
	return Void
}

// Terminator instructions

type Ret struct {
	baseVoidInstruction

	Value Value
}

func (r *Ret) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(r.Value)
	}
}

type Br struct {
	baseVoidInstruction

	Label *Block
}

func (b *Br) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {}
}

type BrCond struct {
	baseVoidInstruction

	Condition Value
	IfTrue    *Block
	IfFalse   *Block
}

func (b *BrCond) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(b.Condition)
	}
}

// Unary instructions

type FNeg struct {
	baseInstruction

	Value Value
}

func (f *FNeg) Type() Type {
	return f.Value.Type()
}

func (f *FNeg) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(f.Value)
	}
}

// Binary instructions

type DivKind uint8

const (
	Unsigned DivKind = iota
	Signed
)

type IAdd struct {
	baseInstruction

	Left  Value
	Right Value
}

func (i *IAdd) Type() Type {
	return i.Left.Type()
}

func (i *IAdd) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(i.Left) {
			return
		}
		yield(i.Right)
	}
}

type FAdd struct {
	baseInstruction

	Left  Value
	Right Value
}

func (f *FAdd) Type() Type {
	return f.Left.Type()
}

func (f *FAdd) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(f.Left) {
			return
		}
		yield(f.Right)
	}
}

type ISub struct {
	baseInstruction

	Left  Value
	Right Value
}

func (i *ISub) Type() Type {
	return i.Left.Type()
}

func (i *ISub) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(i.Left) {
			return
		}
		yield(i.Right)
	}
}

type FSub struct {
	baseInstruction

	Left  Value
	Right Value
}

func (f *FSub) Type() Type {
	return f.Left.Type()
}

func (f *FSub) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(f.Left) {
			return
		}
		yield(f.Right)
	}
}

type IMul struct {
	baseInstruction

	Left  Value
	Right Value
}

func (i *IMul) Type() Type {
	return i.Left.Type()
}

func (i *IMul) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(i.Left) {
			return
		}
		yield(i.Right)
	}
}

type FMul struct {
	baseInstruction

	Left  Value
	Right Value
}

func (f *FMul) Type() Type {
	return f.Left.Type()
}

func (f *FMul) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(f.Left) {
			return
		}
		yield(f.Right)
	}
}

type IDiv struct {
	baseInstruction

	Kind  DivKind
	Left  Value
	Right Value
}

func (i *IDiv) Type() Type {
	return i.Left.Type()
}

func (i *IDiv) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(i.Left) {
			return
		}
		yield(i.Right)
	}
}

type FDiv struct {
	baseInstruction

	Left  Value
	Right Value
}

func (f *FDiv) Type() Type {
	return f.Left.Type()
}

func (f *FDiv) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(f.Left) {
			return
		}
		yield(f.Right)
	}
}

type IRem struct {
	baseInstruction

	Kind  DivKind
	Left  Value
	Right Value
}

func (i *IRem) Type() Type {
	return i.Left.Type()
}

func (i *IRem) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(i.Left) {
			return
		}
		yield(i.Right)
	}
}

type FRem struct {
	baseInstruction

	Left  Value
	Right Value
}

func (f *FRem) Type() Type {
	return f.Left.Type()
}

func (f *FRem) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(f.Left) {
			return
		}
		yield(f.Right)
	}
}

// Bitwise binary instructions

type Shl struct {
	baseInstruction

	Left  Value
	Right Value
}

func (s *Shl) Type() Type {
	return s.Left.Type()
}

func (s *Shl) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(s.Left) {
			return
		}
		yield(s.Right)
	}
}

type Shr struct {
	baseInstruction

	SignExt bool
	Left    Value
	Right   Value
}

func (s *Shr) Type() Type {
	return s.Left.Type()
}

func (s *Shr) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(s.Left) {
			return
		}
		yield(s.Right)
	}
}

type And struct {
	baseInstruction

	Left  Value
	Right Value
}

func (a *And) Type() Type {
	return a.Left.Type()
}

func (a *And) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(a.Left) {
			return
		}
		yield(a.Right)
	}
}

type Or struct {
	baseInstruction

	Left  Value
	Right Value
}

func (o *Or) Type() Type {
	return o.Left.Type()
}

func (o *Or) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(o.Left) {
			return
		}
		yield(o.Right)
	}
}

type Xor struct {
	baseInstruction

	Left  Value
	Right Value
}

func (x *Xor) Type() Type {
	return x.Left.Type()
}

func (x *Xor) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(x.Left) {
			return
		}
		yield(x.Right)
	}
}

// Vector instructions

type ExtractElement struct {
	baseInstruction

	Value Value
	Index Value
}

func (e *ExtractElement) Type() Type {
	return e.Value.Type().(*VectorType).Element
}

func (e *ExtractElement) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(e.Value) {
			return
		}
		yield(e.Index)
	}
}

type InsertElement struct {
	baseInstruction

	Value   Value
	Element Value
	Index   Value
}

func (i *InsertElement) Type() Type {
	return i.Value.Type()
}

func (i *InsertElement) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(i.Value) {
			return
		}
		if !yield(i.Element) {
			return
		}
		yield(i.Index)
	}
}

type ShuffleVector struct {
	baseInstruction

	Value1 Value
	Value2 Value
	Mask   Value
}

func (s *ShuffleVector) Type() Type {
	return &VectorType{
		Length:  s.Mask.Type().(*VectorType).Length,
		Element: s.Value1.Type().(*VectorType).Element,
	}
}

func (s *ShuffleVector) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(s.Value1) {
			return
		}
		if !yield(s.Value2) {
			return
		}
		yield(s.Mask)
	}
}

// Aggregate instructions

type ExtractValue struct {
	baseInstruction

	Value   Value
	Indices []uint32
}

func (e *ExtractValue) Type() Type {
	typ := e.Value.Type()

	for _, index := range e.Indices {
		switch type_ := typ.(type) {
		case *ArrayType:
			typ = type_.Element
		case *StructType:
			typ = type_.Fields[index].Type
		case *RefStructType:
			typ = type_.Struct.Fields[index].Type

		default:
			panic("ir.ExtractValue.Type() - Invalid aggregate value type")
		}
	}

	return typ
}

func (e *ExtractValue) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(e.Value)
	}
}

type InsertValue struct {
	baseInstruction

	Value   Value
	Element Value
	Indices []uint32
}

func (i *InsertValue) Type() Type {
	return i.Value.Type()
}

func (i *InsertValue) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(i.Value) {
			return
		}
		yield(i.Element)
	}
}

// Memory access and addressing instructions

type Alloca struct {
	baseInstruction

	Typ   Type
	Count uint32
}

func (a *Alloca) Type() Type {
	return Pointer
}

func (a *Alloca) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {}
}

type Load struct {
	baseInstruction

	Typ     Type
	Pointer Value
}

func (l *Load) Type() Type {
	return l.Typ
}

func (l *Load) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(l.Pointer)
	}
}

type Store struct {
	baseVoidInstruction

	Value   Value
	Pointer Value
}

func (s *Store) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(s.Value) {
			return
		}
		yield(s.Pointer)
	}
}

type GetElementPtrConst struct {
	baseInstruction

	Typ     Type
	Pointer Value

	Indices [4]uint32
}

func (g *GetElementPtrConst) Type() Type {
	return Pointer
}

func (g *GetElementPtrConst) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(g.Pointer)
	}
}

type GetElementPtrDyn struct {
	baseInstruction

	Typ     Type
	Pointer Value

	Indices [4]Value
}

func (g *GetElementPtrDyn) Type() Type {
	return Pointer
}

func (g *GetElementPtrDyn) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(g.Pointer) {
			return
		}
		for _, index := range g.Indices {
			if !core.IsNil(index) && !yield(index) {
				return
			}
		}
	}
}

// Conversion instructions

type ITrunc struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (i *ITrunc) Type() Type {
	return i.Typ
}

func (i *ITrunc) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(i.Value)
	}
}

type FTrunc struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (f *FTrunc) Type() Type {
	return f.Typ
}

func (f *FTrunc) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(f.Value)
	}
}

type IExt struct {
	baseInstruction

	Kind  DivKind
	Value Value
	Typ   Type
}

func (i *IExt) Type() Type {
	return i.Typ
}

func (i *IExt) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(i.Value)
	}
}

type FExt struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (f *FExt) Type() Type {
	return f.Typ
}

func (f *FExt) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(f.Value)
	}
}

type FpToInt struct {
	baseInstruction

	Signed bool
	Value  Value
	Typ    Type
}

func (f *FpToInt) Type() Type {
	return f.Typ
}

func (f *FpToInt) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(f.Value)
	}
}

type IntToFp struct {
	baseInstruction

	Signed bool
	Value  Value
	Typ    Type
}

func (i *IntToFp) Type() Type {
	return i.Typ
}

func (i *IntToFp) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(i.Value)
	}
}

type PtrToInt struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (p *PtrToInt) Type() Type {
	return p.Typ
}

func (p *PtrToInt) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(p.Value)
	}
}

type IntToPtr struct {
	baseInstruction

	Value Value
}

func (i *IntToPtr) Type() Type {
	return Pointer
}

func (i *IntToPtr) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(i.Value)
	}
}

type BitCast struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (b *BitCast) Type() Type {
	return b.Typ
}

func (b *BitCast) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(b.Value)
	}
}

// Other instructions

type CmpOp uint8

const (
	Eq CmpOp = iota
	Ne
	Gt
	Ge
	Lt
	Le
)

type ICmp struct {
	baseInstruction

	Op     CmpOp
	Signed bool
	Left   Value
	Right  Value
}

func (i *ICmp) Type() Type {
	return I1
}

func (i *ICmp) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(i.Left) {
			return
		}
		yield(i.Right)
	}
}

type FCmp struct {
	baseInstruction

	Op      CmpOp
	Ordered bool
	Left    Value
	Right   Value
}

func (f *FCmp) Type() Type {
	return I1
}

func (f *FCmp) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(f.Left) {
			return
		}
		yield(f.Right)
	}
}

type PhiPair struct {
	Block *Block
	Value Value
}

type Phi struct {
	baseInstruction

	Pairs []PhiPair
}

func (p *Phi) Type() Type {
	return p.Pairs[0].Value.Type()
}

func (p *Phi) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		for _, pair := range p.Pairs {
			if !yield(pair.Value) {
				return
			}
		}
	}
}

type Select struct {
	baseInstruction

	Condition Value
	IfTrue    Value
	IfFalse   Value
}

func (s *Select) Type() Type {
	return s.IfTrue.Type()
}

func (s *Select) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(s.Condition) {
			return
		}
		if !yield(s.IfTrue) {
			return
		}
		yield(s.IfFalse)
	}
}

type Call struct {
	baseInstruction

	Signature *Signature
	Callee    Value
	Args      []Value
}

func (c *Call) Type() Type {
	return c.Signature.Returns
}

func (c *Call) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		if !yield(c.Callee) {
			return
		}
		for _, arg := range c.Args {
			if !yield(arg) {
				return
			}
		}
	}
}

// Debug instructions

type DbgDeclare struct {
	baseVoidInstruction

	Pointer     Value
	VariableRef MetaRef
	LocationRef MetaRef
}

func (d *DbgDeclare) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(d.Pointer)
	}
}
