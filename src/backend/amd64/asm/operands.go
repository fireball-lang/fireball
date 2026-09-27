package asm

import "fireball/backend/obj"

type Reg uint8

const (
	RAX Reg = iota
	RCX
	RDX
	RBX
	RSP
	RBP
	RSI
	RDI
	R8
	R9
	R10
	R11
	R12
	R13
	R14
	R15
)

type XmmReg uint8

const (
	XMM0 XmmReg = iota
	XMM1
	XMM2
	XMM3
	XMM4
	XMM5
	XMM6
	XMM7
	XMM8
	XMM9
	XMM10
	XMM11
	XMM12
	XMM13
	XMM14
	XMM15
)

type Sym struct {
	Symbol *obj.Symbol
	Addend int64
}

func Symbol(symbol *obj.Symbol) Sym {
	return Sym{Symbol: symbol}
}

func SymbolAddend(symbol *obj.Symbol, addend int64) Sym {
	return Sym{Symbol: symbol, Addend: addend}
}

type Mem struct {
	Base Reg
	Disp int32
	Sym  Sym
}

func RegDisp(base Reg, disp int32) Mem {
	return Mem{
		Base: base,
		Disp: disp,
	}
}

func Ptr(base Reg) Mem {
	return Mem{Base: base}
}

func Rip(symbol *obj.Symbol) Mem {
	return Mem{
		Sym: Symbol(symbol),
	}
}

func RipDisp(symbol *obj.Symbol, disp int32) Mem {
	return Mem{
		Disp: disp,
		Sym:  Symbol(symbol),
	}
}

type RegMem interface {
	Reg | Mem
}

type XmmRegMem interface {
	XmmReg | Mem
}
