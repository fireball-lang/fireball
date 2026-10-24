package asm

type Opcode uint16

type Flags uint8

const (
	FlagDeleted Flags = 1 << iota
)

type Instruction struct {
	Op      Opcode
	Flags   Flags
	NumArgs uint8
	Args    [3]Operand
}

const (
	OcNop Opcode = iota
	OcLabel
	OcAlign
	OcFuncEnd

	// Integer ALU

	OcAdd
	OcAdd32
	OcSub
	OcSub32
	OcMul
	OcMul32
	OcImul
	OcImul32
	OcDiv
	OcDiv32
	OcIdiv
	OcIdiv32
	OcNeg
	OcNeg32

	// Bitwise & Shifts

	OcAnd
	OcAnd32
	OcOr
	OcOr32
	OcXor
	OcXor32
	OcNot
	OcNot32
	OcShlImm
	OcShlImm32
	OcShrImm
	OcShrImm32
	OcSarImm
	OcSarImm32
	OcShlCl
	OcShlCl32
	OcShrCl
	OcShrCl32
	OcSarCl
	OcSarCl32

	// Memory & Movement

	OcPush
	OcPop
	OcMov
	OcMov32
	OcMov8
	OcMov16
	OcMovzx8
	OcMovzx16
	OcMovsx8
	OcMovsx8to32
	OcMovsx16
	OcMovsx16to32
	OcMovsxd
	OcLea

	// Control Flow

	OcJmp
	OcJb
	OcJbe
	OcJa
	OcJae
	OcJe
	OcJne
	OcJl
	OcJle
	OcJg
	OcJge
	OcJs
	OcJns
	OcJp
	OcJnp
	OcCall
	OcRet

	// Comparisons & Conditions

	OcTest
	OcTest32
	OcCmp
	OcCmp32
	OcSetb
	OcSetae
	OcSete
	OcSetne
	OcSetbe
	OcSeta
	OcSetp
	OcSetnp
	OcSetl
	OcSetge
	OcSetle
	OcSetg

	// Sign Extension & System

	OcCqo
	OcCdq
	OcSyscall

	// Floating Point SSE/AVX Arithmetic

	OcAddss
	OcAddsd
	OcSubss
	OcSubsd
	OcMulss
	OcMulsd
	OcDivss
	OcDivsd
	OcAndps
	OcAndpd
	OcOrps
	OcOrpd
	OcXorps
	OcXorpd

	// FP Movement & Conversions

	OcMovss
	OcMovsd
	OcMovdToXmm
	OcMovdToReg
	OcMovqToXmm
	OcMovqToReg
	OcCvtsi2ss
	OcCvtsi2sd
	OcCvttss2si
	OcCvttsd2si
	OcCvtss2sd
	OcCvtsd2ss

	// FP Math & Compare

	OcMinss
	OcMinsd
	OcMaxss
	OcMaxsd
	OcSqrtss
	OcSqrtsd
	OcUcomiss
	OcUcomisd
)
