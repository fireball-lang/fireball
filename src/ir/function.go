package ir

import (
	"fireball/core"
	"iter"
)

// Function

type FunctionFlags uint8

const (
	Declare FunctionFlags = 1 << iota
	DsoLocal
	LinkOnceODR
)

type ParamAttribute uint8

const (
	NonNull ParamAttribute = 1 << iota
	ReadOnly
	WriteOnly
)

type Signature struct {
	Returns Type
	SRet    Type

	Params  []Type
	VarArgs bool
}

type Param struct {
	Attributes ParamAttribute
	Name       string
}

type ParamValue struct {
	Typ  Type
	Name string
}

func (p *ParamValue) Type() Type {
	return p.Typ
}

type Function struct {
	baseRuntimeValue
	Module *Module

	Name             string
	Flags            FunctionFlags
	Signature        *Signature
	Params           []Param
	ReturnAttributes ParamAttribute

	ParamValues []Value
	Blocks      []*Block
}

func (f *Function) Type() Type {
	return Pointer
}

func (f *Function) NewBlock(name string) *Block {
	block := &Block{
		Name: name,
	}

	f.Blocks = append(f.Blocks, block)
	return block
}

func (f *Function) AddFirst(in Instruction) Instruction {
	if len(f.Blocks) == 0 {
		panic("ir.Function.AddFirst() - No blocks in fuction")
	}

	return f.Blocks[0].AddFirst(in)
}

func (f *Function) AddLast(in Instruction) Instruction {
	if len(f.Blocks) == 0 {
		panic("ir.Function.AddLast() - No blocks in fuction")
	}

	return f.Blocks[len(f.Blocks)-1].AddLast(in)
}

// Block

type Block struct {
	Name             string
	InstructionCount uint32

	headInstruction Instruction
	tailInstruction Instruction
}

func (b *Block) AddFirst(in Instruction) Instruction {
	in.setNext(b.headInstruction)
	b.headInstruction = in

	if core.IsNil(b.tailInstruction) {
		b.tailInstruction = in
	}

	b.InstructionCount++

	return in
}

func (b *Block) AddLast(in Instruction) Instruction {
	if core.IsNil(b.headInstruction) {
		b.headInstruction = in
	} else {
		b.tailInstruction.setNext(in)
	}

	b.tailInstruction = in
	b.InstructionCount++

	return in
}

func (b *Block) Instructions() iter.Seq[Instruction] {
	return iterLinkedList(b.headInstruction)
}
