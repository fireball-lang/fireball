package asm

import (
	"fireball/backend/obj"
	"fireball/core"
	"fmt"
	"math"
)

type branchFixup struct {
	offset int
	target Label
}

type encoder struct {
	bytes       []uint8
	relocations []obj.Relocation

	labels       map[Label]int
	fixups       []branchFixup
	symbols      []*obj.Symbol
	labelSymbols map[uint32]*obj.Symbol
}

func Encode(instructions []Instruction, symbols []*obj.Symbol, labelSymbols map[uint32]*obj.Symbol) ([]uint8, []obj.Relocation) {
	defer core.Scope()()

	// Encode
	e := encoder{
		bytes: make([]uint8, 0, len(instructions)*4),

		labels:       make(map[Label]int, 16),
		fixups:       make([]branchFixup, 0, 16),
		symbols:      symbols,
		labelSymbols: labelSymbols,
	}

	for i := range instructions {
		inst := &instructions[i]

		if inst.Flags&FlagDeleted != 0 {
			continue
		}

		e.Encode(inst)
	}

	// Resolve branch fixups
	for _, f := range e.fixups {
		targetOffset, ok := e.labels[f.target]
		if !ok {
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.Assemble() - Label %d was referenced but never bound", f.target.id))
		}

		disp := int32(targetOffset - (f.offset + 4))

		e.bytes[f.offset] = uint8(disp)
		e.bytes[f.offset+1] = uint8(disp >> 8)
		e.bytes[f.offset+2] = uint8(disp >> 16)
		e.bytes[f.offset+3] = uint8(disp >> 24)
	}

	return e.bytes, e.relocations
}

func (a *encoder) Encode(inst *Instruction) {
	switch inst.Op {
	case OcNop:
		a.bytes = append(a.bytes, 0x90)

	case OcLabel:
		lbl := a.getLabel(inst.Args[0])
		currentOffset := len(a.bytes)

		a.labels[lbl] = currentOffset

		if lbl.symbol != nil {
			lbl.symbol.Value = uint64(currentOffset)
		}

	case OcAlign:
		align := int(inst.Args[0].Val)

		for len(a.bytes)%align != 0 {
			a.bytes = append(a.bytes, 0x90)
		}

	case OcFuncEnd:
		sym := a.symbols[inst.Args[0].SymbolID-1]
		sym.Size = uint64(len(a.bytes)) - sym.Value

	// Integer ALU

	case OcAdd:
		a.encodeAlu(true, 0x01, 0, inst.Args[0], inst.Args[1])
	case OcAdd32:
		a.encodeAlu(false, 0x01, 0, inst.Args[0], inst.Args[1])
	case OcSub:
		a.encodeAlu(true, 0x29, 5, inst.Args[0], inst.Args[1])
	case OcSub32:
		a.encodeAlu(false, 0x29, 5, inst.Args[0], inst.Args[1])
	case OcMul:
		a.encodeUnary(true, 4, inst.Args[0])
	case OcMul32:
		a.encodeUnary(false, 4, inst.Args[0])
	case OcImul:
		a.encodeImul(true, inst.Args[0].Reg, inst.Args[1])
	case OcImul32:
		a.encodeImul(false, inst.Args[0].Reg, inst.Args[1])
	case OcDiv:
		a.encodeUnary(true, 6, inst.Args[0])
	case OcDiv32:
		a.encodeUnary(false, 6, inst.Args[0])
	case OcIdiv:
		a.encodeUnary(true, 7, inst.Args[0])
	case OcIdiv32:
		a.encodeUnary(false, 7, inst.Args[0])
	case OcNeg:
		a.encodeUnary(true, 3, inst.Args[0])
	case OcNeg32:
		a.encodeUnary(false, 3, inst.Args[0])

	// Bitwise & Shifts

	case OcAnd:
		a.encodeAlu(true, 0x21, 4, inst.Args[0], inst.Args[1])
	case OcAnd32:
		a.encodeAlu(false, 0x21, 4, inst.Args[0], inst.Args[1])
	case OcOr:
		a.encodeAlu(true, 0x09, 1, inst.Args[0], inst.Args[1])
	case OcOr32:
		a.encodeAlu(false, 0x09, 1, inst.Args[0], inst.Args[1])
	case OcXor:
		a.encodeAlu(true, 0x31, 6, inst.Args[0], inst.Args[1])
	case OcXor32:
		a.encodeAlu(false, 0x31, 6, inst.Args[0], inst.Args[1])
	case OcNot:
		a.encodeUnary(true, 2, inst.Args[0])
	case OcNot32:
		a.encodeUnary(false, 2, inst.Args[0])
	case OcShlImm:
		a.encodeShiftImm(true, 4, inst.Args[0], inst.Args[1])
	case OcShlImm32:
		a.encodeShiftImm(false, 4, inst.Args[0], inst.Args[1])
	case OcShrImm:
		a.encodeShiftImm(true, 5, inst.Args[0], inst.Args[1])
	case OcShrImm32:
		a.encodeShiftImm(false, 5, inst.Args[0], inst.Args[1])
	case OcSarImm:
		a.encodeShiftImm(true, 7, inst.Args[0], inst.Args[1])
	case OcSarImm32:
		a.encodeShiftImm(false, 7, inst.Args[0], inst.Args[1])
	case OcShlCl:
		a.encodeShiftCl(true, 4, inst.Args[0])
	case OcShlCl32:
		a.encodeShiftCl(false, 4, inst.Args[0])
	case OcShrCl:
		a.encodeShiftCl(true, 5, inst.Args[0])
	case OcShrCl32:
		a.encodeShiftCl(false, 5, inst.Args[0])
	case OcSarCl:
		a.encodeShiftCl(true, 7, inst.Args[0])
	case OcSarCl32:
		a.encodeShiftCl(false, 7, inst.Args[0])

	// Memory & Movement

	case OcPush:
		a.encodeRex(false, 0, inst.Args[0].Reg)
		a.bytes = append(a.bytes, 0x50+(uint8(inst.Args[0].Reg)&7))
	case OcPop:
		a.encodeRex(false, 0, inst.Args[0].Reg)
		a.bytes = append(a.bytes, 0x58+(uint8(inst.Args[0].Reg)&7))
	case OcMov:
		a.encodeMov(inst.Args[0], inst.Args[1])
	case OcMov32:
		a.encodeMov32(inst.Args[0], inst.Args[1])
	case OcMov8:
		switch inst.Args[1].Kind {
		case OpReg:
			a.encodeMR(false, 0x88, inst.Args[0], inst.Args[1].Reg)
		case OpImm:
			a.encodeMI8(inst.Args[0], int8(inst.Args[1].Val))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.Encode() - Invalid src kind '%s' for OcMov8", inst.Args[1].Kind))
		}
	case OcMov16:
		switch inst.Args[1].Kind {
		case OpReg:
			a.bytes = append(a.bytes, 0x66)
			a.encodeMR(false, 0x89, inst.Args[0], inst.Args[1].Reg)
		case OpImm:
			a.encodeMI16(inst.Args[0], int16(inst.Args[1].Val))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.Encode() - Invalid src kind '%s' for OcMov16", inst.Args[1].Kind))
		}
	case OcMovzx8:
		a.encodeExt(true, 0xB6, inst.Args[0].Reg, inst.Args[1])
	case OcMovzx16:
		a.encodeExt(true, 0xB7, inst.Args[0].Reg, inst.Args[1])
	case OcMovsx8:
		a.encodeExt(true, 0xBE, inst.Args[0].Reg, inst.Args[1])
	case OcMovsx8to32:
		a.encodeExt(false, 0xBE, inst.Args[0].Reg, inst.Args[1])
	case OcMovsx16:
		a.encodeExt(true, 0xBF, inst.Args[0].Reg, inst.Args[1])
	case OcMovsx16to32:
		a.encodeExt(false, 0xBF, inst.Args[0].Reg, inst.Args[1])
	case OcMovsxd:
		a.encodeMovsxd(inst.Args[0].Reg, inst.Args[1])
	case OcLea:
		a.encodeLea(inst.Args[0].Reg, inst.Args[1])

	// Control Flow

	case OcJmp:
		a.encodeJmp(inst.Args[0])
	case OcJb:
		a.encodeJcc(0x82, a.getLabel(inst.Args[0]))
	case OcJbe:
		a.encodeJcc(0x86, a.getLabel(inst.Args[0]))
	case OcJa:
		a.encodeJcc(0x87, a.getLabel(inst.Args[0]))
	case OcJae:
		a.encodeJcc(0x83, a.getLabel(inst.Args[0]))
	case OcJe:
		a.encodeJcc(0x84, a.getLabel(inst.Args[0]))
	case OcJne:
		a.encodeJcc(0x85, a.getLabel(inst.Args[0]))
	case OcJl:
		a.encodeJcc(0x8C, a.getLabel(inst.Args[0]))
	case OcJle:
		a.encodeJcc(0x8E, a.getLabel(inst.Args[0]))
	case OcJg:
		a.encodeJcc(0x8F, a.getLabel(inst.Args[0]))
	case OcJge:
		a.encodeJcc(0x8D, a.getLabel(inst.Args[0]))
	case OcJs:
		a.encodeJcc(0x88, a.getLabel(inst.Args[0]))
	case OcJns:
		a.encodeJcc(0x89, a.getLabel(inst.Args[0]))
	case OcJp:
		a.encodeJcc(0x8A, a.getLabel(inst.Args[0]))
	case OcJnp:
		a.encodeJcc(0x8B, a.getLabel(inst.Args[0]))
	case OcCall:
		a.encodeCall(inst.Args[0])
	case OcRet:
		a.bytes = append(a.bytes, 0xC3)

	// Comparisons & Conditions

	case OcTest:
		a.encodeTest(true, inst.Args[0].Reg, inst.Args[1])
	case OcTest32:
		a.encodeTest(false, inst.Args[0].Reg, inst.Args[1])
	case OcCmp:
		a.encodeCmp(true, inst.Args[0], inst.Args[1])
	case OcCmp32:
		a.encodeCmp(false, inst.Args[0], inst.Args[1])
	case OcSetb:
		a.encodeSet(0x92, inst.Args[0].Reg)
	case OcSetae:
		a.encodeSet(0x93, inst.Args[0].Reg)
	case OcSete:
		a.encodeSet(0x94, inst.Args[0].Reg)
	case OcSetne:
		a.encodeSet(0x95, inst.Args[0].Reg)
	case OcSetbe:
		a.encodeSet(0x96, inst.Args[0].Reg)
	case OcSeta:
		a.encodeSet(0x97, inst.Args[0].Reg)
	case OcSetp:
		a.encodeSet(0x9A, inst.Args[0].Reg)
	case OcSetnp:
		a.encodeSet(0x9B, inst.Args[0].Reg)
	case OcSetl:
		a.encodeSet(0x9C, inst.Args[0].Reg)
	case OcSetge:
		a.encodeSet(0x9D, inst.Args[0].Reg)
	case OcSetle:
		a.encodeSet(0x9E, inst.Args[0].Reg)
	case OcSetg:
		a.encodeSet(0x9F, inst.Args[0].Reg)

	// Sign Extension & System

	case OcCqo:
		a.bytes = append(a.bytes, 0x48, 0x99)
	case OcCdq:
		a.bytes = append(a.bytes, 0x99)
	case OcSyscall:
		a.bytes = append(a.bytes, 0x0F, 0x05)

	// Floating Point SSE/AVX Arithmetic

	case OcAddss:
		a.encodeFpArith(0xF3, 0x58, inst.Args[0].Reg, inst.Args[1])
	case OcAddsd:
		a.encodeFpArith(0xF2, 0x58, inst.Args[0].Reg, inst.Args[1])
	case OcSubss:
		a.encodeFpArith(0xF3, 0x5C, inst.Args[0].Reg, inst.Args[1])
	case OcSubsd:
		a.encodeFpArith(0xF2, 0x5C, inst.Args[0].Reg, inst.Args[1])
	case OcMulss:
		a.encodeFpArith(0xF3, 0x59, inst.Args[0].Reg, inst.Args[1])
	case OcMulsd:
		a.encodeFpArith(0xF2, 0x59, inst.Args[0].Reg, inst.Args[1])
	case OcDivss:
		a.encodeFpArith(0xF3, 0x5E, inst.Args[0].Reg, inst.Args[1])
	case OcDivsd:
		a.encodeFpArith(0xF2, 0x5E, inst.Args[0].Reg, inst.Args[1])
	case OcAndps:
		a.encodeFpArith(0x00, 0x54, inst.Args[0].Reg, inst.Args[1])
	case OcAndpd:
		a.encodeFpArith(0x66, 0x54, inst.Args[0].Reg, inst.Args[1])
	case OcOrps:
		a.encodeFpArith(0x00, 0x56, inst.Args[0].Reg, inst.Args[1])
	case OcOrpd:
		a.encodeFpArith(0x66, 0x56, inst.Args[0].Reg, inst.Args[1])
	case OcXorps:
		a.encodeFpArith(0x00, 0x57, inst.Args[0].Reg, inst.Args[1])
	case OcXorpd:
		a.encodeFpArith(0x66, 0x57, inst.Args[0].Reg, inst.Args[1])

	// FP Movement & Conversions

	case OcMovss:
		a.encodeMovs(0xF3, inst.Args[0], inst.Args[1])
	case OcMovsd:
		a.encodeMovs(0xF2, inst.Args[0], inst.Args[1])
	case OcMovdToXmm:
		a.encodeSseRR(0x66, false, 0x6E, inst.Args[0].Reg, inst.Args[1].Reg)
	case OcMovdToReg:
		a.encodeSseRR(0x66, false, 0x7E, inst.Args[1].Reg, inst.Args[0].Reg)
	case OcMovqToXmm:
		a.encodeSseRR(0x66, true, 0x6E, inst.Args[0].Reg, inst.Args[1].Reg)
	case OcMovqToReg:
		a.encodeSseRR(0x66, true, 0x7E, inst.Args[1].Reg, inst.Args[0].Reg)
	case OcCvtsi2ss:
		a.encodeIntToFp(0xF3, inst.Args[1].Size == 8, inst.Args[0].Reg, inst.Args[1])
	case OcCvtsi2sd:
		a.encodeIntToFp(0xF2, inst.Args[1].Size == 8, inst.Args[0].Reg, inst.Args[1])
	case OcCvttss2si:
		a.encodeFpToInt(0xF3, inst.Args[0].Size == 8, inst.Args[0].Reg, inst.Args[1])
	case OcCvttsd2si:
		a.encodeFpToInt(0xF2, inst.Args[0].Size == 8, inst.Args[0].Reg, inst.Args[1])
	case OcCvtss2sd:
		a.encodeFpArith(0xF3, 0x5A, inst.Args[0].Reg, inst.Args[1])
	case OcCvtsd2ss:
		a.encodeFpArith(0xF2, 0x5A, inst.Args[0].Reg, inst.Args[1])

	// FP Math & Compare

	case OcMinss:
		a.encodeFpArith(0xF3, 0x5D, inst.Args[0].Reg, inst.Args[1])
	case OcMinsd:
		a.encodeFpArith(0xF2, 0x5D, inst.Args[0].Reg, inst.Args[1])
	case OcMaxss:
		a.encodeFpArith(0xF3, 0x5F, inst.Args[0].Reg, inst.Args[1])
	case OcMaxsd:
		a.encodeFpArith(0xF2, 0x5F, inst.Args[0].Reg, inst.Args[1])
	case OcSqrtss:
		a.encodeFpArith(0xF3, 0x51, inst.Args[0].Reg, inst.Args[1])
	case OcSqrtsd:
		a.encodeFpArith(0xF2, 0x51, inst.Args[0].Reg, inst.Args[1])
	case OcUcomiss:
		a.encodeUcomi(0x00, inst.Args[0].Reg, inst.Args[1])
	case OcUcomisd:
		a.encodeUcomi(0x66, inst.Args[0].Reg, inst.Args[1])

	default:
		panic("backend.amd64.asm.Assembler.Encode() - Invalid instruction opcode")
	}
}

// Encoder helpers

func (a *encoder) encodeAlu(is64 bool, opcode uint8, regOpcodeExt uint8, dst Operand, src Operand) {
	switch dst.Kind {
	case OpReg:
		switch src.Kind {
		case OpReg:
			a.encodeRR(is64, opcode, dst.Reg, src.Reg)
		case OpMem:
			a.encodeRM(is64, opcode+2, dst.Reg, src)
		case OpImm:
			a.encodeAluRI(is64, regOpcodeExt, dst.Reg, int32(src.Val))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeAlu() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	case OpMem:
		switch src.Kind {
		case OpReg:
			a.encodeMR(is64, opcode, dst, src.Reg)
		case OpImm:
			a.encodeAluMI(is64, regOpcodeExt, dst, int32(src.Val))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeAlu() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeAlu() - Invalid dst '%s'", dst.Kind))
	}
}

func (a *encoder) encodeCmp(is64 bool, dst Operand, src Operand) {
	switch dst.Kind {
	case OpReg:
		switch src.Kind {
		case OpReg:
			a.encodeRR(is64, 0x39, dst.Reg, src.Reg)
		case OpMem:
			a.encodeRM(is64, 0x3B, dst.Reg, src)
		case OpImm:
			a.encodeAluRI(is64, 7, dst.Reg, int32(src.Val))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeCmp() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	case OpMem:
		switch src.Kind {
		case OpReg:
			a.encodeMR(is64, 0x39, dst, src.Reg)
		case OpImm:
			a.encodeAluMI(is64, 7, dst, int32(src.Val))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeCmp() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeCmp() - Invalid dst '%s'", dst.Kind))
	}
}

func (a *encoder) encodeImul(is64 bool, dst Register, src Operand) {
	switch src.Kind {
	case OpReg:
		a.encodeRex(is64, dst, src.Reg)
		a.bytes = append(a.bytes, 0x0F, 0xAF)
		a.encodeModRm(modReg, dst, src.Reg)

	case OpMem:
		if src.SymbolID != 0 {
			a.encodeRex(is64, dst, 0)
			a.bytes = append(a.bytes, 0x0F, 0xAF)
			a.encodeRipDisp(src, dst, 0)
			return
		}

		a.encodeRexMem(is64, dst, src, false)
		a.bytes = append(a.bytes, 0x0F, 0xAF)
		a.encodeMemDisp(src, dst)

	case OpImm:
		imm := int32(src.Val)
		a.encodeRex(is64, dst, dst)

		if imm >= -128 && imm <= 127 {
			a.bytes = append(a.bytes, 0x6B)
			a.encodeModRm(modReg, dst, dst)
			a.bytes = append(a.bytes, uint8(int8(imm)))
		} else {
			a.bytes = append(a.bytes, 0x69)
			a.encodeModRm(modReg, dst, dst)
			a.encodeInt32(imm)
		}

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeImul() - Invalid combination of src '%s'", src.Kind))
	}
}

func (a *encoder) encodeUnary(is64 bool, ext uint8, target Operand) {
	switch target.Kind {
	case OpReg:
		a.encodeRex(is64, Register(ext), target.Reg)
		a.bytes = append(a.bytes, 0xF7)
		a.encodeModRm(modReg, Register(ext), target.Reg)

	case OpMem:
		if target.SymbolID != 0 {
			a.encodeRex(is64, Register(ext), 0)
			a.bytes = append(a.bytes, 0xF7)
			a.encodeRipDisp(target, Register(ext), 0)
			return
		}

		a.encodeRexMem(is64, Register(ext), target, false)
		a.bytes = append(a.bytes, 0xF7)
		a.encodeMemDisp(target, Register(ext))

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeUnary() - Invalid target '%s'", target.Kind))
	}
}

func (a *encoder) encodeShiftImm(is64 bool, ext uint8, target Operand, count Operand) {
	if count.Val < 0 || count.Val > 255 {
		panic("backend.amd64.asm.Assembler.encodeShiftImm() - Invalid count")
	}

	opcode := uint8(0xC1)
	isOne := count.Val == 1
	if isOne {
		opcode = 0xD1
	}

	switch target.Kind {
	case OpReg:
		a.encodeRex(is64, Register(ext), target.Reg)
		a.bytes = append(a.bytes, opcode)
		a.encodeModRm(modReg, Register(ext), target.Reg)

	case OpMem:
		if target.SymbolID != 0 {
			a.encodeRex(is64, Register(ext), 0)
			a.bytes = append(a.bytes, opcode)

			trailing := 0
			if !isOne {
				trailing = 1
			}

			a.encodeRipDisp(target, Register(ext), trailing)
		} else {
			a.encodeRexMem(is64, Register(ext), target, false)
			a.bytes = append(a.bytes, opcode)
			a.encodeMemDisp(target, Register(ext))
		}

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeShiftImm() - Invalid target '%s'", target.Kind))
	}

	if !isOne {
		a.bytes = append(a.bytes, uint8(count.Val))
	}
}

func (a *encoder) encodeShiftCl(is64 bool, ext uint8, target Operand) {
	switch target.Kind {
	case OpReg:
		a.encodeRex(is64, Register(ext), target.Reg)
		a.bytes = append(a.bytes, 0xD3)
		a.encodeModRm(modReg, Register(ext), target.Reg)

	case OpMem:
		if target.SymbolID != 0 {
			a.encodeRex(is64, Register(ext), 0)
			a.bytes = append(a.bytes, 0xD3)
			a.encodeRipDisp(target, Register(ext), 0)
			return
		}

		a.encodeRexMem(is64, Register(ext), target, false)
		a.bytes = append(a.bytes, 0xD3)
		a.encodeMemDisp(target, Register(ext))

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeShiftCl() - Invalid target '%s'", target.Kind))
	}
}

func (a *encoder) encodeMov(dst Operand, src Operand) {
	switch dst.Kind {
	case OpReg:
		switch src.Kind {
		case OpReg:
			a.encodeRR(true, 0x89, dst.Reg, src.Reg)
		case OpMem:
			a.encodeRM(true, 0x8B, dst.Reg, src)
		case OpSym:
			a.encodeRex(true, 0, dst.Reg)
			a.bytes = append(a.bytes, 0xB8+(uint8(dst.Reg)&7))
			a.relocations = append(a.relocations, obj.Relocation{
				Offset: len(a.bytes),
				Target: a.symbols[src.SymbolID-1],
				Kind:   obj.RelocAbs64,
				Addend: src.Val,
			})
			a.encodeInt64(0)
		case OpImm:
			d := dst.Reg
			s := src.Val

			if s >= 0 && s <= math.MaxUint32 {
				a.encodeRex(false, 0, d)
				a.bytes = append(a.bytes, 0xB8+(uint8(d)&7))
				a.encodeInt32(int32(s))
			} else if s >= math.MinInt32 && s < 0 {
				a.encodeRex(true, 0, d)
				a.bytes = append(a.bytes, 0xC7)
				a.encodeModRm(modReg, 0, d)
				a.encodeInt32(int32(s))
			} else {
				a.encodeRex(true, 0, d)
				a.bytes = append(a.bytes, 0xB8+(uint8(d)&7))
				a.encodeInt64(s)
			}
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMov() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	case OpMem:
		switch src.Kind {
		case OpReg:
			a.encodeMR(true, 0x89, dst, src.Reg)
		case OpImm:
			s := src.Val

			if s < math.MinInt32 || s > math.MaxInt32 {
				panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMov() - immediate %d exceeds 32-bit sign-extended range for memory destination", s))
			}
			a.encodeMI(true, 0xC7, 0, dst, int32(s))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMov() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMov() - Invalid dst '%s'", dst.Kind))
	}
}

func (a *encoder) encodeMov32(dst Operand, src Operand) {
	switch dst.Kind {
	case OpReg:
		switch src.Kind {
		case OpReg:
			a.encodeRR(false, 0x89, dst.Reg, src.Reg)
		case OpMem:
			a.encodeRM(false, 0x8B, dst.Reg, src)
		case OpImm:
			a.encodeRex(false, 0, dst.Reg)
			a.bytes = append(a.bytes, 0xB8+(uint8(dst.Reg)&7))
			a.encodeInt32(int32(src.Val))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMov32() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	case OpMem:
		switch src.Kind {
		case OpReg:
			a.encodeMR(false, 0x89, dst, src.Reg)
		case OpImm:
			a.encodeMI(false, 0xC7, 0, dst, int32(src.Val))
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMov32() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMov32() - Invalid dst '%s'", dst.Kind))
	}
}

func (a *encoder) encodeMovsxd(dst Register, src Operand) {
	switch src.Kind {
	case OpReg:
		a.encodeRex(true, dst, src.Reg)
		a.bytes = append(a.bytes, 0x63)
		a.encodeModRm(modReg, dst, src.Reg)

	case OpMem:
		if src.SymbolID != 0 {
			a.encodeRex(true, dst, 0)
			a.bytes = append(a.bytes, 0x63)
			a.encodeRipDisp(src, dst, 0)
			return
		}

		a.encodeRexMem(true, dst, src, false)
		a.bytes = append(a.bytes, 0x63)
		a.encodeMemDisp(src, dst)

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMovsxd() - Invalid src '%s'", src.Kind))
	}
}

func (a *encoder) encodeLea(dst Register, mem Operand) {
	if mem.SymbolID != 0 {
		a.encodeRex(true, dst, 0)
		a.bytes = append(a.bytes, 0x8D)
		a.encodeRipDisp(mem, dst, 0)
		return
	}

	a.encodeRexMem(true, dst, mem, false)
	a.bytes = append(a.bytes, 0x8D)
	a.encodeMemDisp(mem, dst)
}

func (a *encoder) encodeJmp(target Operand) {
	switch target.Kind {
	case OpLabel:
		a.bytes = append(a.bytes, 0xE9)
		a.encodeBranchFixup(a.getLabel(target))

	case OpReg:
		a.encodeRex(false, 0, target.Reg)
		a.bytes = append(a.bytes, 0xFF)
		a.encodeModRm(modReg, 4, target.Reg)

	case OpMem:
		if target.SymbolID != 0 {
			a.encodeRex(false, 4, 0)
			a.bytes = append(a.bytes, 0xFF)
			a.encodeRipDisp(target, 4, 0)
		} else {
			a.encodeRexMem(false, 4, target, false)
			a.bytes = append(a.bytes, 0xFF)
			a.encodeMemDisp(target, 4)
		}

	case OpSym:
		a.bytes = append(a.bytes, 0xE9)
		a.relocations = append(a.relocations, obj.Relocation{
			Offset: len(a.bytes),
			Target: a.symbols[target.SymbolID-1],
			Kind:   obj.RelocPC32,
			Addend: target.Val,
		})
		a.encodeInt32(0)

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeJmp() - Invalid target '%s'", target.Kind))
	}
}

func (a *encoder) encodeCall(callee Operand) {
	switch callee.Kind {
	case OpLabel:
		a.bytes = append(a.bytes, 0xE8)
		a.encodeBranchFixup(a.getLabel(callee))

	case OpReg:
		a.encodeRex(false, 0, callee.Reg)
		a.bytes = append(a.bytes, 0xFF)
		a.encodeModRm(modReg, 2, callee.Reg)

	case OpMem:
		if callee.SymbolID != 0 {
			a.encodeRex(false, 2, 0)
			a.bytes = append(a.bytes, 0xFF)
			a.encodeRipDisp(callee, 2, 0)
		} else {
			a.encodeRexMem(false, 2, callee, false)
			a.bytes = append(a.bytes, 0xFF)
			a.encodeMemDisp(callee, 2)
		}

	case OpSym:
		a.bytes = append(a.bytes, 0xE8)
		a.relocations = append(a.relocations, obj.Relocation{
			Offset: len(a.bytes),
			Target: a.symbols[callee.SymbolID-1],
			Kind:   obj.RelocPC32,
			Addend: callee.Val,
		})
		a.encodeInt32(0)

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeCall() - Invalid callee '%s'", callee.Kind))
	}
}

func (a *encoder) encodeTest(is64 bool, dst Register, src Operand) {
	switch src.Kind {
	case OpReg:
		a.encodeRR(is64, 0x85, src.Reg, dst)
	case OpMem:
		a.encodeRM(is64, 0x85, dst, src)
	case OpImm:
		a.encodeRex(is64, 0, dst)
		a.bytes = append(a.bytes, 0xF7)
		a.encodeModRm(modReg, 0, dst)
		a.encodeInt32(int32(src.Val))

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeTest() - Invalid src '%s'", src.Kind))
	}
}

func (a *encoder) encodeSet(code uint8, dst Register) {
	// REX prefix is required for byte registers SPL, BPL, SIL, DIL (4..7) and R8B..R15B (8..15)
	if dst >= 4 {
		var rex uint8 = 0x40
		if dst >= 8 {
			rex |= 1 << 0 // B bit (extends ModRM rm field)
		}
		a.bytes = append(a.bytes, rex)
	}

	a.bytes = append(a.bytes, 0x0F, code)
	a.encodeModRm(modReg, 0, dst)
}

func (a *encoder) encodeFpArith(prefix uint8, opcode uint8, dst Register, src Operand) {
	switch src.Kind {
	case OpXmmReg:
		a.encodeSseRR(prefix, false, opcode, dst, src.Reg)
	case OpMem:
		a.encodeSseRM(prefix, false, opcode, dst, src)

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeFpArith() - Invalid src '%s'", src.Kind))
	}
}

func (a *encoder) encodeFpToInt(prefix uint8, is64 bool, dst Register, src Operand) {
	switch src.Kind {
	case OpXmmReg:
		a.encodeSseRR(prefix, is64, 0x2C, dst, src.Reg)
	case OpMem:
		a.encodeSseRM(prefix, is64, 0x2C, dst, src)

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeFpToInt() - Invalid src '%s'", src.Kind))
	}
}

func (a *encoder) encodeIntToFp(prefix uint8, is64 bool, dst Register, src Operand) {
	switch src.Kind {
	case OpReg:
		a.encodeSseRR(prefix, is64, 0x2A, dst, src.Reg)
	case OpMem:
		a.encodeSseRM(prefix, is64, 0x2A, dst, src)

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeIntToFp() - Invalid src '%s'", src.Kind))
	}
}

func (a *encoder) encodeMovs(prefix uint8, dst Operand, src Operand) {
	switch dst.Kind {
	case OpXmmReg:
		switch src.Kind {
		case OpXmmReg:
			a.encodeSseRR(prefix, false, 0x10, dst.Reg, src.Reg)
		case OpMem:
			a.encodeSseRM(prefix, false, 0x10, dst.Reg, src)
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMovs() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	case OpMem:
		switch src.Kind {
		case OpXmmReg:
			a.encodeSseMR(prefix, false, 0x11, dst, src.Reg)
		default:
			panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMovs() - Invalid combination of dst '%s' and src '%s'", dst.Kind, src.Kind))
		}

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeMovs() - Invalid dst '%s'", dst.Kind))
	}
}

func (a *encoder) encodeUcomi(prefix uint8, dst Register, src Operand) {
	switch src.Kind {
	case OpXmmReg:
		a.encodeSseRR(prefix, false, 0x2E, dst, src.Reg)
	case OpMem:
		a.encodeSseRM(prefix, false, 0x2E, dst, src)

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeUcomi() - Invalid src '%s'", src.Kind))
	}
}

// Low level encoding helpers

// encodeSseRR encodes: [prefix] [REX] 0F <opcode> <reg, rm>
func (a *encoder) encodeSseRR(prefix uint8, is64 bool, opcode uint8, dst Register, src Register) {
	if prefix != 0 {
		a.bytes = append(a.bytes, prefix)
	}

	// dst goes to ModRM.reg (REX.R), src goes to ModRM.rm (REX.B)
	a.encodeRex(is64, dst, src)
	a.bytes = append(a.bytes, 0x0F, opcode)
	a.encodeModRm(modReg, dst, src)
}

// encodeSseRM encodes: [prefix] [REX] 0F <opcode> <reg, [mem]>
func (a *encoder) encodeSseRM(prefix uint8, is64 bool, opcode uint8, dst Register, src Operand) {
	if prefix != 0 {
		a.bytes = append(a.bytes, prefix)
	}

	if src.SymbolID != 0 {
		a.encodeRex(is64, dst, 0)
		a.bytes = append(a.bytes, 0x0F, opcode)
		a.encodeRipDisp(src, dst, 0)
		return
	}

	a.encodeRexMem(is64, dst, src, false)
	a.bytes = append(a.bytes, 0x0F, opcode)
	a.encodeMemDisp(src, dst)
}

// encodeSseMR encodes: [prefix] [REX] 0F <opcode> <[mem], reg>
func (a *encoder) encodeSseMR(prefix uint8, is64 bool, opcode uint8, dst Operand, src Register) {
	if prefix != 0 {
		a.bytes = append(a.bytes, prefix)
	}

	if dst.SymbolID != 0 {
		a.encodeRex(is64, src, 0)
		a.bytes = append(a.bytes, 0x0F, opcode)
		a.encodeRipDisp(dst, src, 0)
		return
	}

	a.encodeRexMem(is64, src, dst, false)
	a.bytes = append(a.bytes, 0x0F, opcode)
	a.encodeMemDisp(dst, src)
}

func (a *encoder) encodeJcc(code uint8, target Label) {
	a.bytes = append(a.bytes, 0x0F, code)
	a.encodeBranchFixup(target)
}

func (a *encoder) encodeBranchFixup(target Label) {
	a.fixups = append(a.fixups, branchFixup{
		offset: len(a.bytes),
		target: target,
	})

	a.encodeInt32(0)
}

func (a *encoder) encodeAluRI(is64 bool, regOpcodeExt uint8, dst Register, imm int32) {
	a.encodeRex(is64, Register(regOpcodeExt), dst)

	if imm >= -128 && imm <= 127 {
		a.bytes = append(a.bytes, 0x83)
		a.encodeModRm(modReg, Register(regOpcodeExt), dst)
		a.bytes = append(a.bytes, uint8(int8(imm)))
	} else {
		a.bytes = append(a.bytes, 0x81)
		a.encodeModRm(modReg, Register(regOpcodeExt), dst)
		a.encodeInt32(imm)
	}
}

func (a *encoder) encodeAluMI(is64 bool, regOpcodeExt uint8, dst Operand, imm int32) {
	if dst.SymbolID != 0 {
		a.encodeRex(is64, Register(regOpcodeExt), 0)

		if imm >= -128 && imm <= 127 {
			a.bytes = append(a.bytes, 0x83)
			a.encodeRipDisp(dst, Register(regOpcodeExt), 1)
			a.bytes = append(a.bytes, uint8(int8(imm)))
		} else {
			a.bytes = append(a.bytes, 0x81)
			a.encodeRipDisp(dst, Register(regOpcodeExt), 4)
			a.encodeInt32(imm)
		}

		return
	}

	a.encodeRexMem(is64, Register(regOpcodeExt), dst, false)

	if imm >= -128 && imm <= 127 {
		a.bytes = append(a.bytes, 0x83)
		a.encodeMemDisp(dst, Register(regOpcodeExt))
		a.bytes = append(a.bytes, uint8(int8(imm)))
	} else {
		a.bytes = append(a.bytes, 0x81)
		a.encodeMemDisp(dst, Register(regOpcodeExt))
		a.encodeInt32(imm)
	}
}

func (a *encoder) encodeExt(is64 bool, opcode2 uint8, dst Register, src Operand) {
	switch src.Kind {
	case OpReg:
		isByteSrc := opcode2 == 0xB6 || opcode2 == 0xBE
		forceRex := !is64 && isByteSrc && (src.Reg >= 4 && src.Reg <= 7)

		var rex uint8 = 0x40
		if is64 {
			rex |= 1 << 3
		}
		if dst >= 8 {
			rex |= 1 << 2
		}
		if src.Reg >= 8 {
			rex |= 1 << 0
		}

		if is64 || rex != 0x40 || forceRex {
			a.bytes = append(a.bytes, rex)
		}

		a.bytes = append(a.bytes, 0x0F, opcode2)
		a.encodeModRm(modReg, dst, src.Reg)

	case OpMem:
		if src.SymbolID != 0 {
			a.encodeRex(is64, dst, 0)
			a.bytes = append(a.bytes, 0x0F, opcode2)
			a.encodeRipDisp(src, dst, 0)
			return
		}

		a.encodeRexMem(is64, dst, src, false)
		a.bytes = append(a.bytes, 0x0F, opcode2)
		a.encodeMemDisp(src, dst)

	default:
		panic(fmt.Sprintf("backend.amd64.asm.Assembler.encodeExt() - Invalid src '%s'", src.Kind))
	}
}

const modNoDisp uint8 = 0b00
const modReg uint8 = 0b11
const modDisp8 uint8 = 0b01
const modDisp32 uint8 = 0b10

// encodeRR encodes: op reg, reg
func (a *encoder) encodeRR(is64 bool, opcode uint8, dst Register, src Register) {
	a.encodeRex(is64, src, dst)
	a.bytes = append(a.bytes, opcode)
	a.encodeModRm(modReg, src, dst)
}

// encodeRM encodes: op reg, [mem]
func (a *encoder) encodeRM(is64 bool, opcode uint8, dst Register, src Operand) {
	if src.SymbolID != 0 {
		a.encodeRex(is64, dst, 0)
		a.bytes = append(a.bytes, opcode)
		a.encodeRipDisp(src, dst, 0)
		return
	}

	a.encodeRexMem(is64, dst, src, false)
	a.bytes = append(a.bytes, opcode)
	a.encodeMemDisp(src, dst)
}

// encodeMR encodes: op [mem], reg
func (a *encoder) encodeMR(is64 bool, opcode uint8, dst Operand, src Register) {
	forceRex := opcode == 0x88 && (src >= 4 && src <= 7) // 0x88 is 8-bit mov

	if dst.SymbolID != 0 {
		if forceRex {
			a.bytes = append(a.bytes, 0x40)
		} else {
			a.encodeRex(is64, src, 0)
		}

		a.bytes = append(a.bytes, opcode)
		a.encodeRipDisp(dst, src, 0)
		return
	}

	a.encodeRexMem(is64, src, dst, forceRex)
	a.bytes = append(a.bytes, opcode)
	a.encodeMemDisp(dst, src)
}

// encodeRI encodes: op reg, imm32 (81 /0 id)
func (a *encoder) encodeRI(is64 bool, opcode uint8, regOpcodeExt Register, dst Register, imm int32) {
	a.encodeRex(is64, regOpcodeExt, dst)
	a.bytes = append(a.bytes, opcode)
	a.encodeModRm(modReg, regOpcodeExt, dst)
	a.encodeInt32(imm)
}

// encodeMI encodes: op [mem], imm32 (C7 /0 id)
func (a *encoder) encodeMI(is64 bool, opcode uint8, regOpcodeExt Register, dst Operand, imm int32) {
	if dst.SymbolID != 0 {
		a.encodeRex(is64, regOpcodeExt, 0)
		a.bytes = append(a.bytes, opcode)
		a.encodeRipDisp(dst, regOpcodeExt, 4)
		a.encodeInt32(imm)
		return
	}

	a.encodeRexMem(is64, regOpcodeExt, dst, false)
	a.bytes = append(a.bytes, opcode)
	a.encodeMemDisp(dst, regOpcodeExt)
	a.encodeInt32(imm)
}

// encodeMI16 encodes: 66 [REX] C7 /0 [mem] imm16
func (a *encoder) encodeMI16(dst Operand, imm int16) {
	a.bytes = append(a.bytes, 0x66)

	if dst.SymbolID != 0 {
		a.encodeRex(false, 0, 0)
		a.bytes = append(a.bytes, 0xC7)
		a.encodeRipDisp(dst, 0, 2)
		a.encodeInt16(imm)
		return
	}

	a.encodeRexMem(false, 0, dst, false)
	a.bytes = append(a.bytes, 0xC7)
	a.encodeMemDisp(dst, 0)
	a.encodeInt16(imm)
}

// encodeMI8 encodes: [REX] C6 /0 [mem] imm8
func (a *encoder) encodeMI8(dst Operand, imm int8) {
	if dst.SymbolID != 0 {
		a.encodeRex(false, 0, 0)
		a.bytes = append(a.bytes, 0xC6)
		a.encodeRipDisp(dst, 0, 1)
		a.bytes = append(a.bytes, uint8(imm))
		return
	}

	a.encodeRexMem(false, 0, dst, false)
	a.bytes = append(a.bytes, 0xC6)
	a.encodeMemDisp(dst, 0)
	a.bytes = append(a.bytes, uint8(imm))
}

func (a *encoder) encodeMemDisp(dst Operand, src Register) {
	hasIndex := dst.Scale != 0
	baseLow := dst.Reg & 7
	needsSib := hasIndex || baseLow == 4

	// If SIB follows, ModRM rm must be 4
	rm := dst.Reg
	if needsSib {
		rm = 4
	}

	// Base & 7 == 5 (RBP / R13) cannot use mod=00 with disp=0
	if dst.Val == 0 && baseLow != 5 {
		a.encodeModRm(modNoDisp, src, rm)
		if needsSib {
			a.encodeSib(dst)
		}
	} else if dst.Val >= -128 && dst.Val <= 127 {
		a.encodeModRm(modDisp8, src, rm)
		if needsSib {
			a.encodeSib(dst)
		}
		a.bytes = append(a.bytes, uint8(int8(dst.Val)))
	} else {
		a.encodeModRm(modDisp32, src, rm)
		if needsSib {
			a.encodeSib(dst)
		}
		a.encodeInt32(int32(dst.Val))
	}
}

func (a *encoder) encodeRipDisp(dst Operand, reg Register, trailingBytes int) {
	if dst.Scale != 0 {
		panic("amd64.Assembler - RIP-relative addressing cannot have an index register")
	}

	a.encodeModRm(modNoDisp, reg, 5)
	a.relocations = append(a.relocations, obj.Relocation{
		Offset:        len(a.bytes),
		Target:        a.symbols[dst.SymbolID-1],
		Kind:          obj.RelocPC32,
		Addend:        dst.Val,
		TrailingBytes: trailingBytes,
	})
	a.encodeInt32(0)
}

func (a *encoder) encodeSib(mem Operand) {
	if mem.Scale != 0 {
		if mem.Index == RSP.Reg {
			panic("amd64.Assembler - RSP cannot be used as an SIB index register")
		}

		var scaleBits uint8
		switch mem.Scale {
		case 1:
			scaleBits = 0b00
		case 2:
			scaleBits = 0b01
		case 4:
			scaleBits = 0b10
		case 8:
			scaleBits = 0b11
		default:
			panic(fmt.Sprintf("amd64.Assembler - invalid SIB scale %d (must be 1, 2, 4, or 8)", mem.Scale))
		}

		sib := (scaleBits << 6) | ((uint8(mem.Index) & 7) << 3) | (uint8(mem.Reg) & 7)
		a.bytes = append(a.bytes, sib)
	} else if (mem.Reg & 7) == 4 {
		// RSP (4) or R12 (12) without index requires dummy SIB byte 0x24:
		// scale=0 (1x), index=4 (none), base=4 (RSP/R12)
		a.bytes = append(a.bytes, 0x24)
	}
}

// encodeRexMem encodes REX prefix for an instruction with a register and a memory operand.
func (a *encoder) encodeRexMem(is64 bool, reg Register, mem Operand, forceRex bool) {
	var rex uint8 = 0x40

	if is64 {
		rex |= 1 << 3 // W bit (64-bit width)
	}
	if reg >= 8 {
		rex |= 1 << 2 // R bit (extension of ModRM reg field)
	}
	if mem.Scale != 0 && mem.Index >= 8 {
		rex |= 1 << 1 // X bit (extension of SIB index field)
	}
	if mem.Reg >= 8 {
		rex |= 1 << 0 // B bit (extension of SIB base field or ModRM rm field)
	}

	if is64 || rex != 0x40 || forceRex {
		a.bytes = append(a.bytes, rex)
	}
}

func (a *encoder) encodeRex(is64 bool, reg Register, rm Register) {
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

func (a *encoder) encodeModRm(mod uint8, reg Register, rm Register) {
	b := (mod << 6) | ((uint8(reg) & 7) << 3) | (uint8(rm) & 7)
	a.bytes = append(a.bytes, b)
}

func (a *encoder) encodeInt64(v int64) {
	a.encodeInt32(int32(v))
	a.encodeInt32(int32(v >> 32))
}

func (a *encoder) encodeInt32(v int32) {
	a.bytes = append(a.bytes,
		uint8(v),
		uint8(v>>8),
		uint8(v>>16),
		uint8(v>>24),
	)
}

func (a *encoder) encodeInt16(v int16) {
	a.bytes = append(a.bytes,
		uint8(v),
		uint8(v>>8),
	)
}

// Utils

func (a *encoder) getLabel(o Operand) Label {
	id := uint32(o.Val)
	return Label{id: id, symbol: a.labelSymbols[id]}
}
