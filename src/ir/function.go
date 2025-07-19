package ir

import (
	"fireball/utils"
	"iter"
)

// Function

type FunctionFlags uint8

const (
	Declare FunctionFlags = 1 << iota
	DsoLocal
)

type Param struct {
	Typ  Type
	Name string
}

func (p *Param) Type() Type {
	return p.Typ
}

type Function struct {
	baseRuntimeValue
	Module *Module

	Name       string
	Flags      FunctionFlags
	Typ        *FunctionType
	ParamNames []string

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
	Name string

	headInstruction Instruction
	tailInstruction Instruction
}

func (b *Block) AddFirst(in Instruction) Instruction {
	in.setNext(in)
	b.headInstruction = in

	if utils.IsNil(b.tailInstruction) {
		b.tailInstruction = in
	}

	return in
}

func (b *Block) AddLast(in Instruction) Instruction {
	if utils.IsNil(b.headInstruction) {
		b.headInstruction = in
	} else {
		b.tailInstruction.setNext(in)
	}

	b.tailInstruction = in
	return in
}

func (b *Block) Instructions() iter.Seq[Instruction] {
	return iterLinkedList(b.headInstruction)
}
