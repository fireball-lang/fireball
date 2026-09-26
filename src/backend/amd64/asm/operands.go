package asm

import "fireball/backend/obj"

type Reg uint8

const (
	RAX Reg = 0
	RCX Reg = 1
	RDX Reg = 2
	RBX Reg = 3
	RSP Reg = 4
	RBP Reg = 5
	RSI Reg = 6
	RDI Reg = 7
	R8  Reg = 8
	R9  Reg = 9
	R10 Reg = 10
	R11 Reg = 11
	R12 Reg = 12
	R13 Reg = 13
	R14 Reg = 14
	R15 Reg = 15
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
