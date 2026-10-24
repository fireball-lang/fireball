package asm

import (
	"fireball/backend/obj"
	"fmt"
	"slices"
	"strings"
)

type Register uint8

type OperandKind uint8

const (
	OpNone OperandKind = iota
	OpReg
	OpXmmReg
	OpImm
	OpMem
	OpLabel
	OpSym
)

func (o OperandKind) String() string {
	switch o {
	case OpNone:
		return "none"
	case OpReg:
		return "register"
	case OpXmmReg:
		return "xmm register"
	case OpImm:
		return "immediate"
	case OpMem:
		return "memory"
	case OpLabel:
		return "label"
	case OpSym:
		return "symbol"
	default:
		panic("backend.amd64.asm.OperandKind.String() - Invalid kind")
	}
}

type Operand struct {
	Val      int64       // Immediate, Memory Displacement, Label ID, or Symbol Addend
	SymbolID uint32      // 1-based index into Emitter.symbols (0 = none)
	Reg      Register    // Direct register OR Base register in Mem
	Index    Register    // Index register (for SIB addressing)
	Scale    uint8       // Scale factor (1, 2, 4, 8)
	Size     uint8       // Byte width (1, 2, 4, 8)
	Kind     OperandKind // OpReg, OpXmmReg, OpImm, OpMem, OpLabel, OpSym
}

// Registers

var (
	RAX = Operand{Kind: OpReg, Reg: 0, Size: 8}
	RCX = Operand{Kind: OpReg, Reg: 1, Size: 8}
	RDX = Operand{Kind: OpReg, Reg: 2, Size: 8}
	RBX = Operand{Kind: OpReg, Reg: 3, Size: 8}
	RSP = Operand{Kind: OpReg, Reg: 4, Size: 8}
	RBP = Operand{Kind: OpReg, Reg: 5, Size: 8}
	RSI = Operand{Kind: OpReg, Reg: 6, Size: 8}
	RDI = Operand{Kind: OpReg, Reg: 7, Size: 8}
	R8  = Operand{Kind: OpReg, Reg: 8, Size: 8}
	R9  = Operand{Kind: OpReg, Reg: 9, Size: 8}
	R10 = Operand{Kind: OpReg, Reg: 10, Size: 8}
	R11 = Operand{Kind: OpReg, Reg: 11, Size: 8}
	R12 = Operand{Kind: OpReg, Reg: 12, Size: 8}
	R13 = Operand{Kind: OpReg, Reg: 13, Size: 8}
	R14 = Operand{Kind: OpReg, Reg: 14, Size: 8}
	R15 = Operand{Kind: OpReg, Reg: 15, Size: 8}

	XMM0  = Operand{Kind: OpXmmReg, Reg: 0, Size: 16}
	XMM1  = Operand{Kind: OpXmmReg, Reg: 1, Size: 16}
	XMM2  = Operand{Kind: OpXmmReg, Reg: 2, Size: 16}
	XMM3  = Operand{Kind: OpXmmReg, Reg: 3, Size: 16}
	XMM4  = Operand{Kind: OpXmmReg, Reg: 4, Size: 16}
	XMM5  = Operand{Kind: OpXmmReg, Reg: 5, Size: 16}
	XMM6  = Operand{Kind: OpXmmReg, Reg: 6, Size: 16}
	XMM7  = Operand{Kind: OpXmmReg, Reg: 7, Size: 16}
	XMM8  = Operand{Kind: OpXmmReg, Reg: 8, Size: 16}
	XMM9  = Operand{Kind: OpXmmReg, Reg: 9, Size: 16}
	XMM10 = Operand{Kind: OpXmmReg, Reg: 10, Size: 16}
	XMM11 = Operand{Kind: OpXmmReg, Reg: 11, Size: 16}
	XMM12 = Operand{Kind: OpXmmReg, Reg: 12, Size: 16}
	XMM13 = Operand{Kind: OpXmmReg, Reg: 13, Size: 16}
	XMM14 = Operand{Kind: OpXmmReg, Reg: 14, Size: 16}
	XMM15 = Operand{Kind: OpXmmReg, Reg: 15, Size: 16}
)

// Constructors

// Ptr [base]
func Ptr(base Operand) Operand {
	return Operand{Kind: OpMem, Reg: base.Reg, Size: base.Size}
}

// RegDisp [base + disp]
func RegDisp(base Operand, disp int32) Operand {
	return Operand{Kind: OpMem, Reg: base.Reg, Val: int64(disp), Size: base.Size}
}

// BaseIndex [base + index] (scale 1)
func BaseIndex(base Operand, index Operand) Operand {
	return Operand{Kind: OpMem, Reg: base.Reg, Index: index.Reg, Scale: 1, Size: base.Size}
}

// BaseIndexDisp [base + index + disp] (scale 1)
func BaseIndexDisp(base Operand, index Operand, disp int32) Operand {
	return Operand{Kind: OpMem, Reg: base.Reg, Index: index.Reg, Scale: 1, Val: int64(disp), Size: base.Size}
}

// Index [base + index*scale]
func Index(base Operand, index Operand, scale uint8) Operand {
	return Operand{Kind: OpMem, Reg: base.Reg, Index: index.Reg, Scale: scale, Size: base.Size}
}

// IndexDisp [base + index*scale + disp]
func IndexDisp(base Operand, index Operand, scale uint8, disp int32) Operand {
	return Operand{Kind: OpMem, Reg: base.Reg, Index: index.Reg, Scale: scale, Val: int64(disp), Size: base.Size}
}

// Rip [rip + symbol]
func Rip(symbol *obj.Symbol) Operand {
	return Operand{Kind: OpMem, SymbolID: symbol.ID}
}

// RipDisp [rip + symbol + disp]
func RipDisp(symbol *obj.Symbol, disp int32) Operand {
	return Operand{Kind: OpMem, SymbolID: symbol.ID, Val: int64(disp)}
}

func Imm(val int64) Operand {
	return Operand{Kind: OpImm, Val: val, Size: 8}
}

func Imm32(val int32) Operand {
	return Operand{Kind: OpImm, Val: int64(val), Size: 4}
}

func Imm16(val int16) Operand {
	return Operand{Kind: OpImm, Val: int64(val), Size: 2}
}

func Imm8(val uint8) Operand {
	return Operand{Kind: OpImm, Val: int64(val), Size: 1}
}

func (l Label) Op() Operand {
	return Operand{Kind: OpLabel, Val: int64(l.id)}
}

func LabelOp(l Label) Operand {
	return Operand{Kind: OpLabel, Val: int64(l.id)}
}

func Sym(symbol *obj.Symbol) Operand {
	return Operand{Kind: OpSym, SymbolID: symbol.ID, Size: 8}
}

func SymAddend(symbol *obj.Symbol, addend int64) Operand {
	return Operand{Kind: OpSym, SymbolID: symbol.ID, Val: addend, Size: 8}
}

// Helpers

func (o Operand) IsReg(regs ...Operand) bool {
	if o.Kind != OpReg {
		return false
	}

	if len(regs) > 0 {
		return o.Reg == regs[0].Reg
	}

	return true
}

func (o Operand) IsXmm(regs ...Operand) bool {
	if o.Kind != OpXmmReg {
		return false
	}

	if len(regs) > 0 {
		return o.Reg == regs[0].Reg
	}

	return true
}

func (o Operand) IsImm(vals ...int64) bool {
	if o.Kind != OpImm {
		return false
	}

	if len(vals) > 0 {
		return o.Val == vals[0]
	}

	return true
}

func (o Operand) IsMem() bool {
	return o.Kind == OpMem
}

func (o Operand) IsSym() bool {
	return o.Kind == OpSym
}

func (o Operand) IsLabel(labels ...Label) bool {
	if o.Kind != OpLabel {
		return false
	}

	if len(labels) > 0 {
		return o.Val == int64(labels[0].id)
	}

	return true
}

func (o Operand) Assert(kinds ...OperandKind) {
	if !slices.Contains(kinds, o.Kind) {
		var sb strings.Builder

		sb.WriteString("backend.amd64.asm.Operand.Assert() - Operand '")
		sb.WriteString(o.Kind.String())
		sb.WriteString("' needs to be")

		for i, kind := range kinds {
			if i > 0 {
				sb.WriteString(", '")
			} else {
				sb.WriteString(" '")
			}

			sb.WriteString(kind.String())
			sb.WriteString("'")
		}

		panic(sb.String())
	}
}

var reg64Names = []string{"rax", "rcx", "rdx", "rbx", "rsp", "rbp", "rsi", "rdi", "r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15"}
var reg32Names = []string{"eax", "ecx", "edx", "ebx", "esp", "ebp", "esi", "edi", "r8d", "r9d", "r10d", "r11d", "r12d", "r13d", "r14d", "r15d"}
var reg16Names = []string{"ax", "cx", "dx", "bx", "sp", "bp", "si", "di", "r8w", "r9w", "r10w", "r11w", "r12w", "r13w", "r14w", "r15w"}
var reg8Names = []string{"al", "cl", "dl", "bl", "spl", "bpl", "sil", "dil", "r8b", "r9b", "r10b", "r11b", "r12b", "r13b", "r14b", "r15b"}

func (o Operand) String() string {
	switch o.Kind {
	case OpReg:
		idx := int(o.Reg)
		if idx < 0 || idx >= len(reg64Names) {
			return fmt.Sprintf("r?%d", o.Reg)
		}

		switch o.Size {
		case 1:
			return reg8Names[idx]
		case 2:
			return reg16Names[idx]
		case 4:
			return reg32Names[idx]
		default:
			return reg64Names[idx]
		}

	case OpXmmReg:
		return fmt.Sprintf("xmm%d", o.Reg)

	case OpImm:
		return fmt.Sprintf("$0x%x", o.Val)

	case OpLabel:
		return fmt.Sprintf(".L%d", o.Val)

	case OpSym:
		if o.SymbolID != 0 {
			if o.Val != 0 {
				return fmt.Sprintf("sym%d+%d", o.SymbolID, o.Val)
			}

			return fmt.Sprintf("sym%d", o.SymbolID)
		}

		return "sym?"

	case OpMem:
		if o.SymbolID != 0 {
			if o.Val != 0 {
				return fmt.Sprintf("[rip + sym%d + %d]", o.SymbolID, o.Val)
			}

			return fmt.Sprintf("[rip + sym%d]", o.SymbolID)
		}

		baseName := reg64Names[o.Reg]
		if o.Index != 0 || o.Scale != 0 {
			indexName := reg64Names[o.Index]
			scale := o.Scale

			if scale == 0 {
				scale = 1
			}

			if o.Val != 0 {
				return fmt.Sprintf("[%s + %s*%d + %d]", baseName, indexName, scale, o.Val)
			}

			return fmt.Sprintf("[%s + %s*%d]", baseName, indexName, scale)
		}

		if o.Val != 0 {
			return fmt.Sprintf("[%s + %d]", baseName, o.Val)
		}

		return fmt.Sprintf("[%s]", baseName)

	default:
		return "none"
	}
}
