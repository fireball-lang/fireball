package asm

import (
	"fmt"
	"math"
)

type Label struct {
	id             uint32
	symbolDefIndex int
}

type SymbolDef struct {
	Name   string
	Offset int

	Function bool
	Global   bool
}

type RelocKind uint8

const (
	RelocPC32 RelocKind = iota
	RelocAbs64
	RelocAbs32
	RelocSigned32
)

type Relocation struct {
	Offset int

	Symbol string
	Type   RelocKind

	UserAddend    int64
	TrailingBytes int
}

// ElfAddend returns the addend required by ELF (R_X86_64_*) relocations.
func (r Relocation) ElfAddend() int64 {
	if r.Type == RelocPC32 {
		return int64(-4-r.TrailingBytes) + r.UserAddend
	}

	return r.UserAddend
}

// CoffType returns the Microsoft PE/COFF relocation constant (IMAGE_REL_AMD64_*).
func (r Relocation) CoffType() uint16 {
	switch r.Type {
	case RelocAbs64:
		return 0x0001 // IMAGE_REL_AMD64_ADDR64
	case RelocAbs32:
		return 0x0002 // IMAGE_REL_AMD64_ADDR32
	case RelocSigned32:
		return 0x0003 // IMAGE_REL_AMD64_ADDR32NB (or REL32 depending on usage)

	case RelocPC32:
		// In PE/COFF, IMAGE_REL_AMD64_REL32 (0x0004) through REL32_5 (0x0009)
		// map sequentially to the number of trailing bytes.
		return 0x0004 + uint16(r.TrailingBytes)

	default:
		panic("amd64.Relocation.CoffType() - Unsupported relocation kind for COFF")
	}
}

type branchFixup struct {
	offset int
	target Label
}

type Assembler struct {
	bytes       []uint8
	relocations []Relocation
	symbolDefs  []SymbolDef

	nextLabelId uint32
	labels      map[Label]int
	fixups      []branchFixup
}

func (a *Assembler) Assemble() ([]uint8, []Relocation, []SymbolDef) {
	for _, f := range a.fixups {
		targetOffset, ok := a.labels[f.target]
		if !ok {
			panic(fmt.Sprintf("amd64.Assembler.Assemble() - Label %d was referenced but never bound", f.target))
		}

		disp := int32(targetOffset - (f.offset + 4))

		a.bytes[f.offset] = uint8(disp)
		a.bytes[f.offset+1] = uint8(disp >> 8)
		a.bytes[f.offset+2] = uint8(disp >> 16)
		a.bytes[f.offset+3] = uint8(disp >> 24)
	}

	a.fixups = nil

	return a.bytes, a.relocations, a.symbolDefs
}

// Integer arithmetic

// Add does a 64-bit addition.
func (a *Assembler) Add[D RegMem, S RegMem | int32](dst D, src S) {
	a.add(true, dst, src)
}

// Add32 does a 32-bit addition, if `dst` is a register, then the upper half is zeroed out.
func (a *Assembler) Add32[D RegMem, S RegMem | int32](dst D, src S) {
	a.add(false, dst, src)
}

func (a *Assembler) add[D RegMem, S RegMem | int32](is64 bool, dst D, src S) {
	switch d := any(dst).(type) {
	case Reg:
		switch s := any(src).(type) {
		case Reg:
			a.emitRR(is64, 0x01, d, s)
		case Mem:
			a.emitRM(is64, 0x03, d, s)
		case int32:
			a.emitAluRI(is64, 0, d, s)
		}

	case Mem:
		switch s := any(src).(type) {
		case Reg:
			a.emitMR(is64, 0x01, d, s)
		case int32:
			a.emitAluMI(is64, 0, d, s)
		case Mem:
			panic("amd64.Assembler.Add() - memory-to-memory operations are not supported")
		}
	}
}

// Sub does a 64-bit subtraction.
func (a *Assembler) Sub[D RegMem, S RegMem | int32](dst D, src S) {
	a.sub(true, dst, src)
}

// Sub32 does a 32-bit subtraction, if `dst` is a register, then the upper half is zeroed out.
func (a *Assembler) Sub32[D RegMem, S RegMem | int32](dst D, src S) {
	a.sub(false, dst, src)
}

func (a *Assembler) sub[D RegMem, S RegMem | int32](is64 bool, dst D, src S) {
	switch d := any(dst).(type) {
	case Reg:
		switch s := any(src).(type) {
		case Reg:
			a.emitRR(is64, 0x29, d, s)
		case Mem:
			a.emitRM(is64, 0x2B, d, s)
		case int32:
			a.emitAluRI(is64, 5, d, s)
		}

	case Mem:
		switch s := any(src).(type) {
		case Reg:
			a.emitMR(is64, 0x29, d, s)
		case int32:
			a.emitAluMI(is64, 5, d, s)
		case Mem:
			panic("amd64.Assembler.Sub() - memory-to-memory operations are not supported")
		}
	}
}

// Mul does an unsigned 64-bit multiplication.
func (a *Assembler) Mul[S RegMem](src S) {
	a.emitUnaryOp(true, 4, src)
}

// Mul32 does an unsigned 32-bit multiplication.
func (a *Assembler) Mul32[S RegMem](src S) {
	a.emitUnaryOp(false, 4, src)
}

// Imul does a signed 64-bit multiplication.
func (a *Assembler) Imul[S RegMem | int32](dst Reg, src S) {
	a.imul(true, dst, src)
}

// Imul32 does a signed 32-bit multiplication, if `dst` is a register, then the upper half is zeroed out.
func (a *Assembler) Imul32[S RegMem | int32](dst Reg, src S) {
	a.imul(false, dst, src)
}

func (a *Assembler) imul[S RegMem | int32](is64 bool, dst Reg, src S) {
	switch s := any(src).(type) {
	case Reg:
		a.emitRex(is64, dst, s)
		a.bytes = append(a.bytes, 0x0F, 0xAF)
		a.emitModRm(modReg, dst, s)

	case Mem:
		if s.Symbol.Name != "" {
			a.emitRex(is64, dst, 0)
			a.bytes = append(a.bytes, 0x0F, 0xAF)
			a.emitRipDisp(s, dst, 0)
			return
		}

		a.emitRex(is64, dst, s.Base)
		a.bytes = append(a.bytes, 0x0F, 0xAF)
		a.emitMemDisp(s, dst)

	case int32:
		a.emitRex(is64, dst, dst)

		if s >= -128 && s <= 127 {
			a.bytes = append(a.bytes, 0x6B)
			a.emitModRm(modReg, dst, dst)
			a.bytes = append(a.bytes, uint8(int8(s)))
		} else {
			a.bytes = append(a.bytes, 0x69)
			a.emitModRm(modReg, dst, dst)
			a.emitInt32(s)
		}
	}
}

// Div does an unsigned 64-bit division.
func (a *Assembler) Div[D RegMem](divisor D) {
	a.emitUnaryOp(true, 6, divisor)
}

// Div32 does an unsigned 32-bit division.
func (a *Assembler) Div32[D RegMem](divisor D) {
	a.emitUnaryOp(false, 6, divisor)
}

// Idiv does a signed 64-bit division.
func (a *Assembler) Idiv[D RegMem](divisor D) {
	a.emitUnaryOp(true, 7, divisor)
}

// Idiv32 does a signed 32-bit division.
func (a *Assembler) Idiv32[D RegMem](divisor D) {
	a.emitUnaryOp(false, 7, divisor)
}

// Neg does a signed 64-bit twos-complement negation.
func (a *Assembler) Neg[D RegMem](dst D) {
	a.emitUnaryOp(true, 3, dst)
}

// Neg32 does a signed 32-bit twos-complement negation.
func (a *Assembler) Neg32[D RegMem](dst D) {
	a.emitUnaryOp(false, 3, dst)
}

// Binary operations

// And does a 64-bit binary and.
func (a *Assembler) And[D RegMem, S RegMem | int32](dst D, src S) {
	a.and(true, dst, src)
}

// And32 does a 32-bit binary and, if `dst` is a register, then the upper half is zeroed out.
func (a *Assembler) And32[D RegMem, S RegMem | int32](dst D, src S) {
	a.and(false, dst, src)
}

func (a *Assembler) and[D RegMem, S RegMem | int32](is64 bool, dst D, src S) {
	switch d := any(dst).(type) {
	case Reg:
		switch s := any(src).(type) {
		case Reg:
			a.emitRR(is64, 0x21, d, s)
		case Mem:
			a.emitRM(is64, 0x23, d, s)
		case int32:
			a.emitAluRI(is64, 4, d, s)
		}

	case Mem:
		switch s := any(src).(type) {
		case Reg:
			a.emitMR(is64, 0x21, d, s)
		case int32:
			a.emitAluMI(is64, 4, d, s)
		case Mem:
			panic("amd64.Assembler.And() - memory-to-memory operations are not supported")
		}
	}
}

// Or does a 64-bit binary or.
func (a *Assembler) Or[D RegMem, S RegMem | int32](dst D, src S) {
	a.or(true, dst, src)
}

// Or32 does a 32-bit binary or, if `dst` is a register, then the upper half is zeroed out.
func (a *Assembler) Or32[D RegMem, S RegMem | int32](dst D, src S) {
	a.or(false, dst, src)
}

func (a *Assembler) or[D RegMem, S RegMem | int32](is64 bool, dst D, src S) {
	switch d := any(dst).(type) {
	case Reg:
		switch s := any(src).(type) {
		case Reg:
			a.emitRR(is64, 0x09, d, s)
		case Mem:
			a.emitRM(is64, 0x0B, d, s)
		case int32:
			a.emitAluRI(is64, 1, d, s)
		}

	case Mem:
		switch s := any(src).(type) {
		case Reg:
			a.emitMR(is64, 0x09, d, s)
		case int32:
			a.emitAluMI(is64, 1, d, s)
		case Mem:
			panic("amd64.Assembler.Or() - memory-to-memory operations are not supported")
		}
	}
}

// Xor does a 64-bit binary xor.
func (a *Assembler) Xor[D RegMem, S RegMem | int32](dst D, src S) {
	a.xor(true, dst, src)
}

// Xor32 does a 32-bit binary xor, if `dst` is a register, then the upper half is zeroed out.
func (a *Assembler) Xor32[D RegMem, S RegMem | int32](dst D, src S) {
	a.xor(false, dst, src)
}

func (a *Assembler) xor[D RegMem, S RegMem | int32](is64 bool, dst D, src S) {
	switch d := any(dst).(type) {
	case Reg:
		switch s := any(src).(type) {
		case Reg:
			a.emitRR(is64, 0x31, d, s)
		case Mem:
			a.emitRM(is64, 0x33, d, s)
		case int32:
			a.emitAluRI(is64, 6, d, s)
		}

	case Mem:
		switch s := any(src).(type) {
		case Reg:
			a.emitMR(is64, 0x31, d, s)
		case int32:
			a.emitAluMI(is64, 6, d, s)
		case Mem:
			panic("amd64.Assembler.Xor() - memory-to-memory operations are not supported")
		}
	}
}

// Not does an unsigned 64-bit binary not.
func (a *Assembler) Not[D RegMem](dst D) {
	a.emitUnaryOp(true, 2, dst)
}

// Not32 does an unsigned 32-bit binary not.
func (a *Assembler) Not32[D RegMem](dst D) {
	a.emitUnaryOp(false, 2, dst)
}

// ShlImm does a 64-bit binary shift left.
func (a *Assembler) ShlImm[D RegMem](dst D, count uint8) {
	a.emitShiftImm(true, 4, dst, count)
}

// ShlImm32 does a 32-bit binary shift left.
func (a *Assembler) ShlImm32[D RegMem](dst D, count uint8) {
	a.emitShiftImm(false, 4, dst, count)
}

// ShrImm does a 64-bit binary shift right.
func (a *Assembler) ShrImm[D RegMem](dst D, count uint8) {
	a.emitShiftImm(true, 5, dst, count)
}

// ShrImm32 does a 32-bit binary shift right.
func (a *Assembler) ShrImm32[D RegMem](dst D, count uint8) {
	a.emitShiftImm(false, 5, dst, count)
}

// SarImm does a 64-bit arithmetic shift right.
func (a *Assembler) SarImm[D RegMem](dst D, count uint8) {
	a.emitShiftImm(true, 7, dst, count)
}

// SarImm32 does a 32-bit arithmetic shift right.
func (a *Assembler) SarImm32[D RegMem](dst D, count uint8) {
	a.emitShiftImm(false, 7, dst, count)
}

// ShlCl does a 64-bit binary shift left.
func (a *Assembler) ShlCl[D RegMem](dst D) {
	a.emitShiftCl(true, 4, dst)
}

// ShlCl32 does a 32-bit binary shift left.
func (a *Assembler) ShlCl32[D RegMem](dst D) {
	a.emitShiftCl(false, 4, dst)
}

// ShrCl does a 64-bit binary shift right.
func (a *Assembler) ShrCl[D RegMem](dst D) {
	a.emitShiftCl(true, 5, dst)
}

// ShrCl32 does a 32-bit binary shift right.
func (a *Assembler) ShrCl32[D RegMem](dst D) {
	a.emitShiftCl(false, 5, dst)
}

// SarCl does a 64-bit arithmetic shift right.
func (a *Assembler) SarCl[D RegMem](dst D) {
	a.emitShiftCl(true, 7, dst)
}

// SarCl32 does a 32-bit arithmetic shift right.
func (a *Assembler) SarCl32[D RegMem](dst D) {
	a.emitShiftCl(false, 7, dst)
}

// Memory instructions

func (a *Assembler) Push(r Reg) {
	a.emitRex(false, 0, r)
	a.bytes = append(a.bytes, 0x50+(uint8(r)&7))
}

func (a *Assembler) Pop(r Reg) {
	a.emitRex(false, 0, r)
	a.bytes = append(a.bytes, 0x58+(uint8(r)&7))
}

// Mov moves a 64-bit value.
func (a *Assembler) Mov[D RegMem, S RegMem | Sym | int64](dst D, src S) {
	switch d := any(dst).(type) {
	case Reg:
		switch s := any(src).(type) {
		case Reg:
			a.emitRR(true, 0x89, d, s)
		case Mem:
			a.emitRM(true, 0x8B, d, s)
		case Sym:
			a.emitRex(true, 0, d)
			a.bytes = append(a.bytes, 0xB8+(uint8(d)&7))
			a.relocations = append(a.relocations, Relocation{
				Offset:     len(a.bytes),
				Symbol:     s.Name,
				Type:       RelocAbs64,
				UserAddend: s.Addend,
			})
			a.emitInt64(0)
		case int64:
			if s >= 0 && s <= math.MaxUint32 {
				a.emitRex(false, 0, d)
				a.bytes = append(a.bytes, 0xB8+(uint8(d)&7))
				a.emitInt32(int32(s))
			} else if s >= math.MinInt32 && s < 0 {
				a.emitRex(true, 0, d)
				a.bytes = append(a.bytes, 0xC7)
				a.emitModRm(modReg, 0, d)
				a.emitInt32(int32(s))
			} else {
				a.emitRex(true, 0, d)
				a.bytes = append(a.bytes, 0xB8+(uint8(d)&7))
				a.emitInt64(s)
			}
		}

	case Mem:
		switch s := any(src).(type) {
		case Reg:
			a.emitMR(true, 0x89, d, s)
		case Mem:
			panic("amd64.Assembler.Mov() - memory-to-memory operations are not supported")
		case Sym:
			panic("amd64.Assembler.Mov() - symbol-to-memory operations are not supported")
		case int64:
			if s < math.MinInt32 || s > math.MaxInt32 {
				panic(fmt.Sprintf("amd64.Assembler.Mov() - immediate %d exceeds 32-bit sign-extended range for memory destination", s))
			}
			a.emitMI(true, 0xC7, 0, d, int32(s))
		}
	}
}

// Mov32 moves a 32-bit value, if `dst` is a register, then the upper half is zeroed out.
func (a *Assembler) Mov32[D RegMem, S RegMem | int32](dst D, src S) {
	switch d := any(dst).(type) {
	case Reg:
		switch s := any(src).(type) {
		case Reg:
			a.emitRR(false, 0x89, d, s)
		case Mem:
			a.emitRM(false, 0x8B, d, s)
		case int32:
			a.emitRex(false, 0, d)
			a.bytes = append(a.bytes, 0xB8+(uint8(d)&7))
			a.emitInt32(s)
		}

	case Mem:
		switch s := any(src).(type) {
		case Reg:
			a.emitMR(false, 0x89, d, s)
		case int32:
			a.emitMI(false, 0xC7, 0, d, s)
		case Mem:
			panic("amd64.Assembler.Mov32() - memory-to-memory operations are not supported")
		}
	}
}

// Movzx8 loads an 8-bit value into a register, zero-extending it to 64 bits.
func (a *Assembler) Movzx8[S RegMem](dst Reg, src S) {
	a.emitExt(true, 0xB6, dst, src)
}

// Movzx16 loads a 16-bit value into a register, zero-extending it to 64 bits.
func (a *Assembler) Movzx16[S RegMem](dst Reg, src S) {
	a.emitExt(true, 0xB7, dst, src)
}

// Movsx8 loads an 8-bit value into a register, sign-extending it to 64 bits.
func (a *Assembler) Movsx8[S RegMem](dst Reg, src S) {
	a.emitExt(true, 0xBE, dst, src)
}

// Movsx8to32 loads an 8-bit value, sign-extending it to 32 bits (upper 32 bits zeroed).
func (a *Assembler) Movsx8to32[S RegMem](dst Reg, src S) {
	a.emitExt(false, 0xBE, dst, src)
}

// Movsx16 loads a 16-bit value into a register, sign-extending it to 64 bits.
func (a *Assembler) Movsx16[S RegMem](dst Reg, src S) {
	a.emitExt(true, 0xBF, dst, src)
}

// Movsx16to32 loads a 16-bit value, sign-extending it to 32 bits (upper 32 bits zeroed).
func (a *Assembler) Movsx16to32[S RegMem](dst Reg, src S) {
	a.emitExt(false, 0xBF, dst, src)
}

// Movsxd loads a 32-bit value into a register, sign-extending it to 64 bits.
func (a *Assembler) Movsxd[S RegMem](dst Reg, src S) {
	switch s := any(src).(type) {
	case Reg:
		a.emitRex(true, dst, s)
		a.bytes = append(a.bytes, 0x63)
		a.emitModRm(modReg, dst, s)

	case Mem:
		if s.Symbol.Name != "" {
			a.emitRex(true, dst, 0)
			a.bytes = append(a.bytes, 0x63)
			a.emitRipDisp(s, dst, 0)
			return
		}

		a.emitRex(true, dst, s.Base)
		a.bytes = append(a.bytes, 0x63)
		a.emitMemDisp(s, dst)
	}
}

// Control flow instructions

func (a *Assembler) Label() Label {
	a.nextLabelId++

	return Label{
		id:             a.nextLabelId,
		symbolDefIndex: -1,
	}
}

func (a *Assembler) Global(name string) Label {
	a.nextLabelId++

	a.symbolDefs = append(a.symbolDefs, SymbolDef{
		Name:     name,
		Offset:   0,
		Function: true,
		Global:   true,
	})

	return Label{
		id:             a.nextLabelId,
		symbolDefIndex: len(a.symbolDefs) - 1,
	}
}

func (a *Assembler) Bind(l Label) {
	if a.labels == nil {
		a.labels = make(map[Label]int)
	}

	if _, exists := a.labels[l]; exists {
		panic(fmt.Sprintf("amd64.Assembler.Bind() - Label %d already bound", l))
	}

	a.labels[l] = len(a.bytes)

	if l.symbolDefIndex != -1 {
		a.symbolDefs[l.symbolDefIndex].Offset = len(a.bytes)
	}
}

// Jmp unconditional jump.
func (a *Assembler) Jmp[T Label | Reg | Sym](target T) {
	switch t := any(target).(type) {
	case Label:
		a.bytes = append(a.bytes, 0xE9)
		a.emitBranchFixup(t)

	case Reg:
		a.emitRex(false, 0, t)
		a.bytes = append(a.bytes, 0xFF)
		a.emitModRm(modReg, 4, t)

	case Sym:
		a.bytes = append(a.bytes, 0xE9)
		a.relocations = append(a.relocations, Relocation{
			Offset:     len(a.bytes),
			Symbol:     t.Name,
			Type:       RelocPC32,
			UserAddend: t.Addend,
		})
		a.emitInt32(0)
	}
}

// Jb unsigned < jump.
func (a *Assembler) Jb(target Label) {
	a.emitJcc(0x82, target)
}

// Jbe unsigned <= jump.
func (a *Assembler) Jbe(target Label) {
	a.emitJcc(0x86, target)
}

// Ja unsigned > jump.
func (a *Assembler) Ja(target Label) {
	a.emitJcc(0x87, target)
}

// Jae unsigned >= jump.
func (a *Assembler) Jae(target Label) {
	a.emitJcc(0x83, target)
}

// Je == jump.
func (a *Assembler) Je(target Label) {
	a.emitJcc(0x84, target)
}

// Jne != jump.
func (a *Assembler) Jne(target Label) {
	a.emitJcc(0x85, target)
}

// Jl signed < jump.
func (a *Assembler) Jl(target Label) {
	a.emitJcc(0x8C, target)
}

// Jle signed <= jump.
func (a *Assembler) Jle(target Label) {
	a.emitJcc(0x8E, target)
}

// Jg signed > jump.
func (a *Assembler) Jg(target Label) {
	a.emitJcc(0x8F, target)
}

// Jge signed >= jump.
func (a *Assembler) Jge(target Label) {
	a.emitJcc(0x8D, target)
}

func (a *Assembler) Call[C Label | Reg | Sym](callee C) {
	switch c := any(callee).(type) {
	case Label:
		a.bytes = append(a.bytes, 0xE8)
		a.emitBranchFixup(c)

	case Reg:
		a.emitRex(false, 0, c)
		a.bytes = append(a.bytes, 0xFF)
		a.emitModRm(modReg, 2, c)

	case Sym:
		a.bytes = append(a.bytes, 0xE8)
		a.relocations = append(a.relocations, Relocation{
			Offset:     len(a.bytes),
			Symbol:     c.Name,
			Type:       RelocPC32,
			UserAddend: c.Addend,
		})
		a.emitInt32(0)
	}
}

func (a *Assembler) Ret() {
	a.bytes = append(a.bytes, 0xC3)
}

// Other

// Test does a 64-bit test.
func (a *Assembler) Test[S RegMem | int32](dst Reg, src S) {
	a.test(true, dst, src)
}

// Test32 does a 32-bit test.
func (a *Assembler) Test32[S RegMem | int32](dst Reg, src S) {
	a.test(false, dst, src)
}

func (a *Assembler) test[S RegMem | int32](is64 bool, dst Reg, src S) {
	switch s := any(src).(type) {
	case Reg:
		a.emitRR(is64, 0x85, s, dst)
	case Mem:
		a.emitRM(is64, 0x85, dst, s)
	case int32:
		a.emitRex(is64, 0, dst)
		a.bytes = append(a.bytes, 0xF7)
		a.emitModRm(modReg, 0, dst)
		a.emitInt32(s)
	}
}

// Cmp does a 64-bit comparison.
func (a *Assembler) Cmp[D RegMem, S RegMem | int32](dst D, src S) {
	a.cmp(true, dst, src)
}

// Cmp32 does a 32-bit comparison.
func (a *Assembler) Cmp32[D RegMem, S RegMem | int32](dst D, src S) {
	a.cmp(false, dst, src)
}

func (a *Assembler) cmp[D RegMem, S RegMem | int32](is64 bool, dst D, src S) {
	switch d := any(dst).(type) {
	case Reg:
		switch s := any(src).(type) {
		case Reg:
			a.emitRR(is64, 0x39, d, s)
		case Mem:
			a.emitRM(is64, 0x3B, d, s)
		case int32:
			a.emitAluRI(is64, 7, d, s)
		}

	case Mem:
		switch s := any(src).(type) {
		case Reg:
			a.emitMR(is64, 0x39, d, s)
		case int32:
			a.emitAluMI(is64, 7, d, s)
		case Mem:
			panic("amd64.Assembler.Cmp() - memory-to-memory operations are not supported")
		}
	}
}

func (a *Assembler) Lea(dst Reg, mem Mem) {
	if mem.Symbol.Name != "" {
		a.emitRex(true, dst, 0)
		a.bytes = append(a.bytes, 0x8D)
		a.emitRipDisp(mem, dst, 0)
		return
	}

	a.emitRex(true, dst, mem.Base)
	a.bytes = append(a.bytes, 0x8D)
	a.emitMemDisp(mem, dst)
}

// Cqo sign-extends RAX into RDX:RAX.
func (a *Assembler) Cqo() {
	a.bytes = append(a.bytes, 0x48, 0x99)
}

// Cdq sign-extends EAX into EDX:EAX.
func (a *Assembler) Cdq() {
	a.bytes = append(a.bytes, 0x99)
}

func (a *Assembler) Syscall() {
	a.bytes = append(a.bytes, 0x0F, 0x05)
}

func (a *Assembler) Nop() {
	a.bytes = append(a.bytes, 0x90)
}

// Utils

func (a *Assembler) emitExt[S RegMem](is64 bool, opcode2 uint8, dst Reg, src S) {
	switch s := any(src).(type) {
	case Reg:
		isByteSrc := opcode2 == 0xB6 || opcode2 == 0xBE
		forceRex := !is64 && isByteSrc && (s >= 4 && s <= 7)

		var rex uint8 = 0x40
		if is64 {
			rex |= 1 << 3
		}
		if dst >= 8 {
			rex |= 1 << 2
		}
		if s >= 8 {
			rex |= 1 << 0
		}

		if is64 || rex != 0x40 || forceRex {
			a.bytes = append(a.bytes, rex)
		}

		a.bytes = append(a.bytes, 0x0F, opcode2)
		a.emitModRm(modReg, dst, s)

	case Mem:
		if s.Symbol.Name != "" {
			a.emitRex(is64, dst, 0)
			a.bytes = append(a.bytes, 0x0F, opcode2)
			a.emitRipDisp(s, dst, 0)
			return
		}

		a.emitRex(is64, dst, s.Base)
		a.bytes = append(a.bytes, 0x0F, opcode2)
		a.emitMemDisp(s, dst)
	}
}

func (a *Assembler) emitJcc(code uint8, target Label) {
	a.bytes = append(a.bytes, 0x0F, code)
	a.emitBranchFixup(target)
}

func (a *Assembler) emitBranchFixup(target Label) {
	a.fixups = append(a.fixups, branchFixup{
		offset: len(a.bytes),
		target: target,
	})

	a.emitInt32(0)
}

func (a *Assembler) emitShiftImm[D RegMem](is64 bool, ext uint8, target D, count uint8) {
	opcode := uint8(0xC1)
	isOne := count == 1
	if isOne {
		opcode = 0xD1
	}

	switch t := any(target).(type) {
	case Reg:
		a.emitRex(is64, Reg(ext), t)
		a.bytes = append(a.bytes, opcode)
		a.emitModRm(modReg, Reg(ext), t)

	case Mem:
		if t.Symbol.Name != "" {
			a.emitRex(is64, Reg(ext), 0)
			a.bytes = append(a.bytes, opcode)

			trailing := 0
			if !isOne {
				trailing = 1
			}

			a.emitRipDisp(t, Reg(ext), trailing)
		} else {
			a.emitRex(is64, Reg(ext), t.Base)
			a.bytes = append(a.bytes, opcode)
			a.emitMemDisp(t, Reg(ext))
		}
	}

	if !isOne {
		a.bytes = append(a.bytes, count)
	}
}

func (a *Assembler) emitShiftCl[D RegMem](is64 bool, ext uint8, target D) {
	switch t := any(target).(type) {
	case Reg:
		a.emitRex(is64, Reg(ext), t)
		a.bytes = append(a.bytes, 0xD3)
		a.emitModRm(modReg, Reg(ext), t)

	case Mem:
		if t.Symbol.Name != "" {
			a.emitRex(is64, Reg(ext), 0)
			a.bytes = append(a.bytes, 0xD3)
			a.emitRipDisp(t, Reg(ext), 0)
			return
		}

		a.emitRex(is64, Reg(ext), t.Base)
		a.bytes = append(a.bytes, 0xD3)
		a.emitMemDisp(t, Reg(ext))
	}
}

func (a *Assembler) emitUnaryOp[D RegMem](is64 bool, ext uint8, target D) {
	switch t := any(target).(type) {
	case Reg:
		a.emitRex(is64, Reg(ext), t)
		a.bytes = append(a.bytes, 0xF7)
		a.emitModRm(modReg, Reg(ext), t)

	case Mem:
		if t.Symbol.Name != "" {
			a.emitRex(is64, Reg(ext), 0)
			a.bytes = append(a.bytes, 0xF7)
			a.emitRipDisp(t, Reg(ext), 0)
			return
		}

		a.emitRex(is64, Reg(ext), t.Base)
		a.bytes = append(a.bytes, 0xF7)
		a.emitMemDisp(t, Reg(ext))
	}
}

func (a *Assembler) emitAluRI(is64 bool, regOpcodeExt uint8, dst Reg, imm int32) {
	a.emitRex(is64, Reg(regOpcodeExt), dst)

	if imm >= -128 && imm <= 127 {
		a.bytes = append(a.bytes, 0x83)
		a.emitModRm(modReg, Reg(regOpcodeExt), dst)
		a.bytes = append(a.bytes, uint8(int8(imm)))
	} else {
		a.bytes = append(a.bytes, 0x81)
		a.emitModRm(modReg, Reg(regOpcodeExt), dst)
		a.emitInt32(imm)
	}
}

func (a *Assembler) emitAluMI(is64 bool, regOpcodeExt uint8, dst Mem, imm int32) {
	if dst.Symbol.Name != "" {
		a.emitRex(is64, Reg(regOpcodeExt), 0)

		if imm >= -128 && imm <= 127 {
			a.bytes = append(a.bytes, 0x83)
			a.emitRipDisp(dst, Reg(regOpcodeExt), 1)
			a.bytes = append(a.bytes, uint8(int8(imm)))
		} else {
			a.bytes = append(a.bytes, 0x81)
			a.emitRipDisp(dst, Reg(regOpcodeExt), 4)
			a.emitInt32(imm)
		}

		return
	}

	a.emitRex(is64, Reg(regOpcodeExt), dst.Base)

	if imm >= -128 && imm <= 127 {
		a.bytes = append(a.bytes, 0x83)
		a.emitMemDisp(dst, Reg(regOpcodeExt))
		a.bytes = append(a.bytes, uint8(int8(imm)))
	} else {
		a.bytes = append(a.bytes, 0x81)
		a.emitMemDisp(dst, Reg(regOpcodeExt))
		a.emitInt32(imm)
	}
}

const modNoDisp uint8 = 0b00
const modReg uint8 = 0b11
const modDisp8 uint8 = 0b01
const modDisp32 uint8 = 0b10

// emitRR encodes: op reg, reg
func (a *Assembler) emitRR(is64 bool, opcode uint8, dst Reg, src Reg) {
	a.emitRex(is64, src, dst)
	a.bytes = append(a.bytes, opcode)
	a.emitModRm(modReg, src, dst)
}

// emitRM encodes: op reg, [mem]
func (a *Assembler) emitRM(is64 bool, opcode uint8, dst Reg, src Mem) {
	if src.Symbol.Name != "" {
		a.emitRex(is64, dst, 0)
		a.bytes = append(a.bytes, opcode)
		a.emitRipDisp(src, dst, 0)
		return
	}

	a.emitRex(is64, dst, src.Base)
	a.bytes = append(a.bytes, opcode)
	a.emitMemDisp(src, dst)
}

// emitMR encodes: op [mem], reg
func (a *Assembler) emitMR(is64 bool, opcode uint8, dst Mem, src Reg) {
	if dst.Symbol.Name != "" {
		a.emitRex(is64, src, 0)
		a.bytes = append(a.bytes, opcode)
		a.emitRipDisp(dst, src, 0)
		return
	}

	a.emitRex(is64, src, dst.Base)
	a.bytes = append(a.bytes, opcode)
	a.emitMemDisp(dst, src)
}

// emitRI encodes: op reg, imm32 (81 /0 id)
func (a *Assembler) emitRI(is64 bool, opcode uint8, regOpcodeExt uint8, dst Reg, imm int32) {
	a.emitRex(is64, Reg(regOpcodeExt), dst)
	a.bytes = append(a.bytes, opcode)
	a.emitModRm(modReg, Reg(regOpcodeExt), dst)
	a.emitInt32(imm)
}

// emitMI encodes: op [mem], imm32 (C7 /0 id)
func (a *Assembler) emitMI(is64 bool, opcode uint8, regOpcodeExt uint8, dst Mem, imm int32) {
	if dst.Symbol.Name != "" {
		a.emitRex(is64, Reg(regOpcodeExt), 0)
		a.bytes = append(a.bytes, opcode)
		a.emitRipDisp(dst, Reg(regOpcodeExt), 4)
		a.emitInt32(imm)
		return
	}

	a.emitRex(is64, Reg(regOpcodeExt), dst.Base)
	a.bytes = append(a.bytes, opcode)
	a.emitMemDisp(dst, Reg(regOpcodeExt))
	a.emitInt32(imm)
}

func (a *Assembler) emitMemDisp(dst Mem, src Reg) {
	if dst.Disp >= -128 && dst.Disp <= 127 {
		a.emitModRm(modDisp8, src, dst.Base)
		a.emitSib(dst.Base)
		a.bytes = append(a.bytes, uint8(int8(dst.Disp)))
	} else {
		a.emitModRm(modDisp32, src, dst.Base)
		a.emitSib(dst.Base)
		a.emitInt32(dst.Disp)
	}
}

func (a *Assembler) emitRipDisp(dst Mem, reg Reg, trailingBytes int) {
	a.emitModRm(modNoDisp, reg, 5)
	a.relocations = append(a.relocations, Relocation{
		Offset:        len(a.bytes),
		Symbol:        dst.Symbol.Name,
		Type:          RelocPC32,
		UserAddend:    dst.Symbol.Addend + int64(dst.Disp),
		TrailingBytes: trailingBytes,
	})
	a.emitInt32(0)
}

// x86 quirk: RSP (4) and R12 (12) require a dummy SIB byte (0x24)
func (a *Assembler) emitSib(base Reg) {
	if (base & 7) == 4 {
		a.bytes = append(a.bytes, 0x24)
	}
}

func (a *Assembler) emitRex(is64 bool, reg Reg, rm Reg) {
	var rex uint8 = 0x40

	if is64 {
		rex |= 1 << 3 // W bit (64-bit width)
	}
	if reg >= 8 {
		rex |= 1 << 2 // R bit (extension of reg field)
	}
	if rm >= 8 {
		rex |= 1 << 0 // B bit (extension of r/m field)
	}

	// Only emit REX if it's 64-bit or using R8-R15
	if is64 || rex != 0x40 {
		a.bytes = append(a.bytes, rex)
	}
}

func (a *Assembler) emitModRm(mod uint8, reg Reg, rm Reg) {
	b := (mod << 6) | ((uint8(reg) & 7) << 3) | (uint8(rm) & 7)
	a.bytes = append(a.bytes, b)
}

func (a *Assembler) emitInt64(v int64) {
	a.emitInt32(int32(v))
	a.emitInt32(int32(v >> 32))
}

func (a *Assembler) emitInt32(v int32) {
	a.bytes = append(a.bytes,
		uint8(v),
		uint8(v>>8),
		uint8(v>>16),
		uint8(v>>24),
	)
}
