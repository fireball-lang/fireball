package asm

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
	Name   string
	Addend int64
}

func Symbol(name string) Sym {
	return Sym{Name: name}
}

func SymbolAddend(name string, addend int64) Sym {
	return Sym{Name: name, Addend: addend}
}

type Mem struct {
	Base   Reg
	Disp   int32
	Symbol Sym
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

func Rip(sym string) Mem {
	return Mem{
		Symbol: Symbol(sym),
	}
}

func RipDisp(sym string, disp int32) Mem {
	return Mem{
		Disp:   disp,
		Symbol: Symbol(sym),
	}
}

type RegMem interface {
	Reg | Mem
}
