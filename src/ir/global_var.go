package ir

type GlobalVarFlags uint8

const (
	Private GlobalVarFlags = 1 << iota
	External
	UnnamedAddr
	Constant
)

type GlobalVar struct {
	baseRuntimeValue
	Module *Module

	Name        string
	Typ         Type
	Flags       GlobalVarFlags
	Initializer Value
}

func (g *GlobalVar) Type() Type {
	return Pointer
}
