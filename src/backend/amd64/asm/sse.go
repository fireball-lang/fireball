package asm

// Floating point arithmetic instructions

// Addss does a 32-bit float addition.
func (a *Assembler) Addss[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF3, 0x58, dst, src)
}

// Addsd does a 64-bit float addition.
func (a *Assembler) Addsd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF2, 0x58, dst, src)
}

// Subss does a 32-bit float subtraction.
func (a *Assembler) Subss[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF3, 0x5C, dst, src)
}

// Subsd does a 64-bit float subtraction.
func (a *Assembler) Subsd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF2, 0x5C, dst, src)
}

// Mulss does a 32-bit float multiplication.
func (a *Assembler) Mulss[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF3, 0x59, dst, src)
}

// Mulsd does a 64-bit float multiplication.
func (a *Assembler) Mulsd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF2, 0x59, dst, src)
}

// Divss does a 32-bit float division.
func (a *Assembler) Divss[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF3, 0x5E, dst, src)
}

// Divsd does a 64-bit float division.
func (a *Assembler) Divsd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF2, 0x5E, dst, src)
}

// Binary operation instructions

// Andps does a 128-bit bitwise AND.
func (a *Assembler) Andps[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0x00, 0x54, dst, src)
}

// Andpd does a 128-bit bitwise AND.
func (a *Assembler) Andpd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0x66, 0x54, dst, src)
}

// Orps does a 128-bit bitwise OR.
func (a *Assembler) Orps[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0x00, 0x56, dst, src)
}

// Orpd does a 128-bit bitwise OR.
func (a *Assembler) Orpd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0x66, 0x56, dst, src)
}

// Xorps does a 128-bit bitwise XOR.
func (a *Assembler) Xorps[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0x00, 0x57, dst, src)
}

// Xorpd does a 128-bit bitwise XOR.
func (a *Assembler) Xorpd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0x66, 0x57, dst, src)
}

// Memory instructions

// Movss moves a 32-bit float value.
func (a *Assembler) Movss[D XmmRegMem, S XmmRegMem](dst D, src S) {
	a.movs(0xF3, dst, src)
}

// Movsd moves a 64-bit float value.
func (a *Assembler) Movsd[D XmmRegMem, S XmmRegMem](dst D, src S) {
	a.movs(0xF2, dst, src)
}

func (a *Assembler) movs[D XmmRegMem, S XmmRegMem](prefix uint8, dst D, src S) {
	switch d := any(dst).(type) {
	case XmmReg:
		switch s := any(src).(type) {
		case XmmReg:
			a.emitSseRR(prefix, false, 0x10, d, s)
		case Mem:
			a.emitSseRM(prefix, false, 0x10, d, s)
		}

	case Mem:
		switch s := any(src).(type) {
		case XmmReg:
			a.emitSseMR(prefix, false, 0x11, d, s)
		case Mem:
			panic("amd64.Assembler - memory-to-memory float move not supported")
		}
	}
}

// MovdToXmm moves a 32-bit integer register into the low 32 bits of an XMM register.
func (a *Assembler) MovdToXmm(dst XmmReg, src Reg) {
	a.emitSseRR(0x66, false, 0x6E, dst, XmmReg(src))
}

// MovdToReg moves the low 32 bits of an XMM register into a 32-bit integer register.
func (a *Assembler) MovdToReg(dst Reg, src XmmReg) {
	a.emitSseRR(0x66, false, 0x7E, src, XmmReg(dst))
}

// MovqToXmm moves a 64-bit integer register into an XMM register.
func (a *Assembler) MovqToXmm(dst XmmReg, src Reg) {
	a.emitSseRR(0x66, true, 0x6E, dst, XmmReg(src))
}

// MovqToReg moves the low 64 bits of an XMM register into a 64-bit integer register.
func (a *Assembler) MovqToReg(dst Reg, src XmmReg) {
	a.emitSseRR(0x66, true, 0x7E, src, XmmReg(dst))
}

// Control flow instructions

// Jp jumps if the parity flag is set (PF=1), typically used to detect NaN.
func (a *Assembler) Jp(target Label) {
	a.emitJcc(0x8A, target)
}

// Jnp jumps if the parity flag is not set (PF=0).
func (a *Assembler) Jnp(target Label) {
	a.emitJcc(0x8B, target)
}

// Conversion instructions

// Cvtsi2ss converts a 32-bit or 64-bit integer to a 32-bit float.
func (a *Assembler) Cvtsi2ss[S RegMem](is64 bool, dst XmmReg, src S) {
	a.emitIntToFp(0xF3, is64, dst, src)
}

// Cvtsi2sd converts a 32-bit or 64-bit integer to a 64-bit float.
func (a *Assembler) Cvtsi2sd[S RegMem](is64 bool, dst XmmReg, src S) {
	a.emitIntToFp(0xF2, is64, dst, src)
}

func (a *Assembler) emitIntToFp[S RegMem](prefix uint8, is64 bool, dst XmmReg, src S) {
	switch s := any(src).(type) {
	case Reg:
		a.emitSseRR(prefix, is64, 0x2A, dst, XmmReg(s))
	case Mem:
		a.emitSseRM(prefix, is64, 0x2A, dst, s)
	}
}

// Cvttss2si converts a 32-bit float to a 32-bit or 64-bit integer with truncation.
func (a *Assembler) Cvttss2si[S XmmRegMem](is64 bool, dst Reg, src S) {
	a.emitFpToInt(0xF3, is64, dst, src)
}

// Cvttsd2si converts a 64-bit float to a 32-bit or 64-bit integer with truncation.
func (a *Assembler) Cvttsd2si[S XmmRegMem](is64 bool, dst Reg, src S) {
	a.emitFpToInt(0xF2, is64, dst, src)
}

func (a *Assembler) emitFpToInt[S XmmRegMem](prefix uint8, is64 bool, dst Reg, src S) {
	switch s := any(src).(type) {
	case XmmReg:
		a.emitSseRR(prefix, is64, 0x2C, XmmReg(dst), s)
	case Mem:
		a.emitSseRM(prefix, is64, 0x2C, XmmReg(dst), s)
	}
}

// Cvtss2sd converts a 32-bit float to a 64-bit float.
func (a *Assembler) Cvtss2sd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF3, 0x5A, dst, src)
}

// Cvtsd2ss converts a 64-bit float to a 32-bit float.
func (a *Assembler) Cvtsd2ss[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF2, 0x5A, dst, src)
}

// Other instructions

// Minss computes the minimum of two 32-bit floats.
func (a *Assembler) Minss[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF3, 0x5D, dst, src)
}

// Minsd computes the minimum of two 64-bit floats.
func (a *Assembler) Minsd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF2, 0x5D, dst, src)
}

// Maxss computes the maximum of two 32-bit floats.
func (a *Assembler) Maxss[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF3, 0x5F, dst, src)
}

// Maxsd computes the maximum of two 64-bit floats.
func (a *Assembler) Maxsd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF2, 0x5F, dst, src)
}

// Sqrtss computes the square root of a 32-bit float.
func (a *Assembler) Sqrtss[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF3, 0x51, dst, src)
}

// Sqrtsd computes the square root of a 64-bit float.
func (a *Assembler) Sqrtsd[S XmmRegMem](dst XmmReg, src S) {
	a.emitFpArith(0xF2, 0x51, dst, src)
}

// Ucomiss compares two 32-bit floats and sets CPU flags.
func (a *Assembler) Ucomiss[S XmmRegMem](dst XmmReg, src S) {
	a.emitUcomi(0x00, dst, src)
}

// Ucomisd compares two 64-bit floats and sets CPU flags.
func (a *Assembler) Ucomisd[S XmmRegMem](dst XmmReg, src S) {
	a.emitUcomi(0x66, dst, src)
}

func (a *Assembler) emitUcomi[S XmmRegMem](prefix uint8, dst XmmReg, src S) {
	switch s := any(src).(type) {
	case XmmReg:
		a.emitSseRR(prefix, false, 0x2E, dst, s)
	case Mem:
		a.emitSseRM(prefix, false, 0x2E, dst, s)
	}
}

// Utils

func (a *Assembler) emitFpArith[S XmmRegMem](prefix uint8, opcode uint8, dst XmmReg, src S) {
	switch s := any(src).(type) {
	case XmmReg:
		a.emitSseRR(prefix, false, opcode, dst, s)
	case Mem:
		a.emitSseRM(prefix, false, opcode, dst, s)
	}
}

// emitSseRR encodes: [prefix] [REX] 0F <opcode> <reg, rm>
func (a *Assembler) emitSseRR(prefix uint8, is64 bool, opcode uint8, dst XmmReg, src XmmReg) {
	if prefix != 0 {
		a.bytes = append(a.bytes, prefix)
	}

	// dst goes to ModRM.reg (REX.R), src goes to ModRM.rm (REX.B)
	a.emitRex(is64, Reg(dst), Reg(src))
	a.bytes = append(a.bytes, 0x0F, opcode)
	a.emitModRm(modReg, Reg(dst), Reg(src))
}

// emitSseRM encodes: [prefix] [REX] 0F <opcode> <reg, [mem]>
func (a *Assembler) emitSseRM(prefix uint8, is64 bool, opcode uint8, dst XmmReg, src Mem) {
	if prefix != 0 {
		a.bytes = append(a.bytes, prefix)
	}

	if src.Sym.Symbol != nil {
		a.emitRex(is64, Reg(dst), 0)
		a.bytes = append(a.bytes, 0x0F, opcode)
		a.emitRipDisp(src, Reg(dst), 0)
		return
	}

	a.emitRexMem(is64, Reg(dst), src)
	a.bytes = append(a.bytes, 0x0F, opcode)
	a.emitMemDisp(src, Reg(dst))
}

// emitSseMR encodes: [prefix] [REX] 0F <opcode> <[mem], reg>
func (a *Assembler) emitSseMR(prefix uint8, is64 bool, opcode uint8, dst Mem, src XmmReg) {
	if prefix != 0 {
		a.bytes = append(a.bytes, prefix)
	}

	if dst.Sym.Symbol != nil {
		a.emitRex(is64, Reg(src), 0)
		a.bytes = append(a.bytes, 0x0F, opcode)
		a.emitRipDisp(dst, Reg(src), 0)
		return
	}

	a.emitRexMem(is64, Reg(src), dst)
	a.bytes = append(a.bytes, 0x0F, opcode)
	a.emitMemDisp(dst, Reg(src))
}
