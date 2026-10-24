package asm

import (
	"fireball/backend/obj"
)

type Label struct {
	id     uint32
	symbol *obj.Symbol
}

type Emitter struct {
	instructions []Instruction
	nextLabelID  uint32
	labelSymbols map[uint32]*obj.Symbol
}

func NewEmitter() *Emitter {
	return &Emitter{
		instructions: make([]Instruction, 0, 1024),
		labelSymbols: make(map[uint32]*obj.Symbol),
	}
}

func (e *Emitter) Instructions() []Instruction          { return e.instructions }
func (e *Emitter) LabelSymbols() map[uint32]*obj.Symbol { return e.labelSymbols }

func (e *Emitter) Reset() {
	e.instructions = e.instructions[:0]
	clear(e.labelSymbols)
	e.nextLabelID = 0
}

// Labels & Alignment

func (e *Emitter) Label(symbol *obj.Symbol) Label {
	e.nextLabelID++

	lbl := Label{id: e.nextLabelID, symbol: symbol}
	if symbol != nil {
		e.labelSymbols[lbl.id] = symbol
	}

	return lbl
}

func (e *Emitter) Bind(l Label) {
	e.instructions = append(e.instructions, Instruction{
		Op:      OcLabel,
		NumArgs: 1,
		Args:    [3]Operand{{Kind: OpLabel, Val: int64(l.id)}},
	})
}

func (e *Emitter) Align(align int) {
	e.instructions = append(e.instructions, Instruction{
		Op:      OcAlign,
		NumArgs: 1,
		Args:    [3]Operand{{Kind: OpImm, Val: int64(align)}},
	})
}

func (e *Emitter) EndFunction(symbol *obj.Symbol) {
	e.instructions = append(e.instructions, Instruction{
		Op:      OcFuncEnd,
		NumArgs: 1,
		Args:    [3]Operand{{Kind: OpSym, SymbolID: symbol.ID}},
	})
}

// Integer Arithmetic

func (e *Emitter) Add(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcAdd, 8, dst, src)
}

func (e *Emitter) Add32(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcAdd32, 4, dst, src)
}

func (e *Emitter) Sub(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcSub, 8, dst, src)
}

func (e *Emitter) Sub32(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcSub32, 4, dst, src)
}

func (e *Emitter) Mul(src Operand) {
	src.Assert(OpReg, OpMem)
	e.emit1(OcMul, 8, src)
}

func (e *Emitter) Mul32(src Operand) {
	src.Assert(OpReg, OpMem)
	e.emit1(OcMul32, 4, src)
}

func (e *Emitter) Imul(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcImul, 8, dst, src)
}

func (e *Emitter) Imul32(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcImul32, 4, dst, src)
}

func (e *Emitter) Div(divisor Operand) {
	divisor.Assert(OpReg, OpMem)
	e.emit1(OcDiv, 8, divisor)
}

func (e *Emitter) Div32(divisor Operand) {
	divisor.Assert(OpReg, OpMem)
	e.emit1(OcDiv32, 4, divisor)
}

func (e *Emitter) Idiv(divisor Operand) {
	divisor.Assert(OpReg, OpMem)
	e.emit1(OcIdiv, 8, divisor)
}

func (e *Emitter) Idiv32(divisor Operand) {
	divisor.Assert(OpReg, OpMem)
	e.emit1(OcIdiv32, 4, divisor)
}

func (e *Emitter) Neg(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcNeg, 8, dst)
}

func (e *Emitter) Neg32(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcNeg32, 4, dst)
}

// Binary Operations & Shifts

func (e *Emitter) And(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcAnd, 8, dst, src)
}

func (e *Emitter) And32(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcAnd32, 4, dst, src)
}

func (e *Emitter) Or(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcOr, 8, dst, src)
}

func (e *Emitter) Or32(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcOr32, 4, dst, src)
}

func (e *Emitter) Xor(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcXor, 8, dst, src)
}

func (e *Emitter) Xor32(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcXor32, 4, dst, src)
}

func (e *Emitter) Not(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcNot, 8, dst)
}

func (e *Emitter) Not32(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcNot32, 4, dst)
}

func (e *Emitter) ShlImm(dst Operand, count uint8) {
	dst.Assert(OpReg, OpMem)
	e.emit2(OcShlImm, 8, dst, Imm8(count))
}

func (e *Emitter) ShlImm32(dst Operand, count uint8) {
	dst.Assert(OpReg, OpMem)
	e.emit2(OcShlImm32, 4, dst, Imm8(count))
}

func (e *Emitter) ShrImm(dst Operand, count uint8) {
	dst.Assert(OpReg, OpMem)
	e.emit2(OcShrImm, 8, dst, Imm8(count))
}

func (e *Emitter) ShrImm32(dst Operand, count uint8) {
	dst.Assert(OpReg, OpMem)
	e.emit2(OcShrImm32, 4, dst, Imm8(count))
}

func (e *Emitter) SarImm(dst Operand, count uint8) {
	dst.Assert(OpReg, OpMem)
	e.emit2(OcSarImm, 8, dst, Imm8(count))
}

func (e *Emitter) SarImm32(dst Operand, count uint8) {
	dst.Assert(OpReg, OpMem)
	e.emit2(OcSarImm32, 4, dst, Imm8(count))
}

func (e *Emitter) ShlCl(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcShlCl, 8, dst)
}

func (e *Emitter) ShlCl32(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcShlCl32, 4, dst)
}

func (e *Emitter) ShrCl(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcShrCl, 8, dst)
}

func (e *Emitter) ShrCl32(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcShrCl32, 4, dst)
}

func (e *Emitter) SarCl(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcSarCl, 8, dst)
}

func (e *Emitter) SarCl32(dst Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit1(OcSarCl32, 4, dst)
}

// Memory & Moves

func (e *Emitter) Push(r Operand) {
	r.Assert(OpReg)
	e.emit1(OcPush, 8, r)
}

func (e *Emitter) Pop(r Operand) {
	r.Assert(OpReg)
	e.emit1(OcPop, 8, r)
}

func (e *Emitter) Mov(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	e.emit2(OcMov, 8, dst, src)
}

func (e *Emitter) Mov32(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcMov32, 4, dst, src)
}

func (e *Emitter) Mov8(dst, src Operand) {
	dst.Assert(OpMem)
	src.Assert(OpReg)

	e.emit2(OcMov8, 1, dst, src)
}

func (e *Emitter) Mov16(dst, src Operand) {
	dst.Assert(OpMem)
	src.Assert(OpReg)

	e.emit2(OcMov16, 2, dst, src)
}

func (e *Emitter) Movzx8(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcMovzx8, 8, dst, src)
}

func (e *Emitter) Movzx16(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcMovzx16, 8, dst, src)
}

func (e *Emitter) Movsx8(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcMovsx8, 8, dst, src)
}

func (e *Emitter) Movsx8to32(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcMovsx8to32, 4, dst, src)
}

func (e *Emitter) Movsx16(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcMovsx16, 8, dst, src)
}

func (e *Emitter) Movsx16to32(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcMovsx16to32, 4, dst, src)
}

func (e *Emitter) Movsxd(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcMovsxd, 8, dst, src)
}

func (e *Emitter) Lea(dst, mem Operand) {
	dst.Assert(OpReg)
	mem.Assert(OpMem)

	e.emit2(OcLea, 8, dst, mem)
}

// Control Flow

func (e *Emitter) Jmp(target Operand) {
	target.Assert(OpLabel, OpReg, OpMem, OpSym)
	e.emit1(OcJmp, 8, target)
}

func (e *Emitter) Jb(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJb, 8, target)
}

func (e *Emitter) Jbe(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJbe, 8, target)
}

func (e *Emitter) Ja(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJa, 8, target)
}

func (e *Emitter) Jae(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJae, 8, target)
}

func (e *Emitter) Je(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJe, 8, target)
}

func (e *Emitter) Jne(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJne, 8, target)
}

func (e *Emitter) Jl(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJl, 8, target)
}

func (e *Emitter) Jle(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJle, 8, target)
}

func (e *Emitter) Jg(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJg, 8, target)
}

func (e *Emitter) Jge(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJge, 8, target)
}

func (e *Emitter) Js(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJs, 8, target)
}

func (e *Emitter) Jns(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJns, 8, target)
}

func (e *Emitter) Jp(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJp, 8, target)
}

func (e *Emitter) Jnp(target Operand) {
	target.Assert(OpLabel)
	e.emit1(OcJnp, 8, target)
}

func (e *Emitter) Call(callee Operand) {
	callee.Assert(OpLabel, OpReg, OpMem, OpSym)
	e.emit1(OcCall, 8, callee)
}

func (e *Emitter) Ret() {
	e.instructions = append(e.instructions, Instruction{Op: OcRet})
}

// Comparisons & Flags

func (e *Emitter) Test(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcTest, 8, dst, src)
}

func (e *Emitter) Test32(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcTest32, 4, dst, src)
}

func (e *Emitter) Cmp(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcCmp, 8, dst, src)
}

func (e *Emitter) Cmp32(dst, src Operand) {
	dst.Assert(OpReg, OpMem)
	src.Assert(OpReg, OpMem, OpImm)

	e.emit2(OcCmp32, 4, dst, src)
}

// SetCC

func (e *Emitter) Setb(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetb, 1, dst)
}

func (e *Emitter) Setae(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetae, 1, dst)
}

func (e *Emitter) Sete(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSete, 1, dst)
}

func (e *Emitter) Setne(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetne, 1, dst)
}

func (e *Emitter) Setbe(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetbe, 1, dst)
}

func (e *Emitter) Seta(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSeta, 1, dst)
}

func (e *Emitter) Setp(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetp, 1, dst)
}

func (e *Emitter) Setnp(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetnp, 1, dst)
}

func (e *Emitter) Setl(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetl, 1, dst)
}

func (e *Emitter) Setge(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetge, 1, dst)
}

func (e *Emitter) Setle(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetle, 1, dst)
}

func (e *Emitter) Setg(dst Operand) {
	dst.Assert(OpReg)
	e.emit1(OcSetg, 1, dst)
}

// Sign Extension & Misc

func (e *Emitter) Cqo()     { e.instructions = append(e.instructions, Instruction{Op: OcCqo}) }
func (e *Emitter) Cdq()     { e.instructions = append(e.instructions, Instruction{Op: OcCdq}) }
func (e *Emitter) Syscall() { e.instructions = append(e.instructions, Instruction{Op: OcSyscall}) }
func (e *Emitter) Nop()     { e.instructions = append(e.instructions, Instruction{Op: OcNop}) }

// Floating Point Arithmetic

func (e *Emitter) Addss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcAddss, 4, dst, src)
}

func (e *Emitter) Addsd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcAddsd, 8, dst, src)
}

func (e *Emitter) Subss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcSubss, 4, dst, src)
}

func (e *Emitter) Subsd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcSubsd, 8, dst, src)
}

func (e *Emitter) Mulss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcMulss, 4, dst, src)
}

func (e *Emitter) Mulsd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcMulsd, 8, dst, src)
}

func (e *Emitter) Divss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcDivss, 4, dst, src)
}

func (e *Emitter) Divsd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcDivsd, 8, dst, src)
}

// FP Bitwise

func (e *Emitter) Andps(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcAndps, 16, dst, src)
}

func (e *Emitter) Andpd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcAndpd, 16, dst, src)
}

func (e *Emitter) Orps(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcOrps, 16, dst, src)
}

func (e *Emitter) Orpd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcOrpd, 16, dst, src)
}

func (e *Emitter) Xorps(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcXorps, 16, dst, src)
}

func (e *Emitter) Xorpd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcXorpd, 16, dst, src)
}

// FP Moves & Conversions

func (e *Emitter) Movss(dst, src Operand) {
	dst.Assert(OpXmmReg, OpMem)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcMovss, 4, dst, src)
}

func (e *Emitter) Movsd(dst, src Operand) {
	dst.Assert(OpXmmReg, OpMem)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcMovsd, 8, dst, src)
}

func (e *Emitter) MovdToXmm(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpReg)

	e.emit2(OcMovdToXmm, 4, dst, src)
}

func (e *Emitter) MovdToReg(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpXmmReg)

	e.emit2(OcMovdToReg, 4, dst, src)
}

func (e *Emitter) MovqToXmm(dst, src Operand) {
	dst.Assert(OpXmmReg)
	src.Assert(OpReg)

	e.emit2(OcMovqToXmm, 8, dst, src)
}

func (e *Emitter) MovqToReg(dst, src Operand) {
	dst.Assert(OpReg)
	src.Assert(OpXmmReg)

	e.emit2(OcMovqToReg, 8, dst, src)
}

func (e *Emitter) Cvtsi2ss(is64 bool, dst, src Operand) {
	size := uint8(4)
	if is64 {
		size = 8
	}

	dst.Assert(OpXmmReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcCvtsi2ss, size, dst, src)
}

func (e *Emitter) Cvtsi2sd(is64 bool, dst, src Operand) {
	size := uint8(4)
	if is64 {
		size = 8
	}

	dst.Assert(OpXmmReg)
	src.Assert(OpReg, OpMem)

	e.emit2(OcCvtsi2sd, size, dst, src)
}

func (e *Emitter) Cvttss2si(is64 bool, dst, src Operand) {
	size := uint8(4)
	if is64 {
		size = 8
	}

	dst.Assert(OpReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcCvttss2si, size, dst, src)
}

func (e *Emitter) Cvttsd2si(is64 bool, dst, src Operand) {
	size := uint8(4)
	if is64 {
		size = 8
	}

	dst.Assert(OpReg)
	src.Assert(OpXmmReg, OpMem)

	e.emit2(OcCvttsd2si, size, dst, src)
}

func (e *Emitter) Cvtss2sd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcCvtss2sd, 4, dst, src)
}

func (e *Emitter) Cvtsd2ss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcCvtsd2ss, 8, dst, src)
}

// FP Comparisons & Math

func (e *Emitter) Minss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcMinss, 4, dst, src)
}

func (e *Emitter) Minsd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcMinsd, 8, dst, src)
}

func (e *Emitter) Maxss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcMaxss, 4, dst, src)
}

func (e *Emitter) Maxsd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcMaxsd, 8, dst, src)
}

func (e *Emitter) Sqrtss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcSqrtss, 4, dst, src)
}

func (e *Emitter) Sqrtsd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcSqrtsd, 8, dst, src)
}

func (e *Emitter) Ucomiss(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcUcomiss, 4, dst, src)
}

func (e *Emitter) Ucomisd(dst, src Operand) {
	dst.Assert(OpXmmReg)
	e.emit2(OcUcomisd, 8, dst, src)
}

// Helpers

func (e *Emitter) emit1(op Opcode, size uint8, arg0 Operand) {
	if arg0.Kind == OpReg || arg0.Kind == OpXmmReg {
		arg0.Size = size
	}

	e.instructions = append(e.instructions, Instruction{
		Op:      op,
		NumArgs: 1,
		Args:    [3]Operand{arg0},
	})
}

func (e *Emitter) emit2(op Opcode, size uint8, arg0, arg1 Operand) {
	if arg0.Kind == OpReg || arg0.Kind == OpXmmReg {
		arg0.Size = size
	}
	if arg1.Kind == OpReg || arg1.Kind == OpXmmReg {
		arg1.Size = size
	}

	e.instructions = append(e.instructions, Instruction{
		Op:      op,
		NumArgs: 2,
		Args:    [3]Operand{arg0, arg1},
	})
}
