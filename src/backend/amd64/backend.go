package amd64

import (
	"fireball/backend/amd64/asm"
	"fireball/backend/obj"
	"fireball/core"
	"fireball/ir"
)

type backend struct {
	module *ir.Module
	file   *obj.File

	intParamRegs []asm.Reg
	fpParamRegs  []asm.XmmReg

	text   *obj.Section
	data   *obj.Section
	rodata *obj.Section

	gVarSymbols map[*ir.GlobalVar]*obj.Symbol
	funSymbols  map[*ir.Function]*obj.Symbol

	symbols map[string]*obj.Symbol

	asm asm.Assembler

	layout      frameLayout
	epilogue    asm.Label
	blockLabels map[*ir.Block]asm.Label
}

type frameLayout struct {
	offsets map[ir.Value]int32
	size    uint32
}

var intParamRegsSysV = []asm.Reg{asm.RDI, asm.RSI, asm.RDX, asm.RCX, asm.R8, asm.R9}
var fpParamRegsSysV = []asm.XmmReg{asm.XMM0, asm.XMM1, asm.XMM2, asm.XMM3, asm.XMM4, asm.XMM5, asm.XMM6, asm.XMM7}

var intParamRegsWin = []asm.Reg{asm.RCX, asm.RDX, asm.R8, asm.R9}
var fpParamRegsWin = []asm.XmmReg{asm.XMM0, asm.XMM1, asm.XMM2, asm.XMM3}

func Generate(module *ir.Module) *obj.File {
	defer core.Scope()()

	// Create backend
	b := backend{
		module:      module,
		file:        &obj.File{Arch: obj.AMD64},
		gVarSymbols: make(map[*ir.GlobalVar]*obj.Symbol),
		funSymbols:  make(map[*ir.Function]*obj.Symbol),
		symbols:     make(map[string]*obj.Symbol),
	}

	switch b.module.Triple {
	case "x86_64-pc-linux-gnu":
		b.intParamRegs = intParamRegsSysV
		b.fpParamRegs = fpParamRegsSysV

	case "x86_64-pc-windows-gnu":
		b.intParamRegs = intParamRegsWin
		b.fpParamRegs = fpParamRegsWin

	default:
		panic("amd64.Generate() - Invalid target triple: " + b.module.Triple)
	}

	// Create basic sections
	b.text = b.file.AddSection(&obj.Section{
		Name:  ".text",
		Kind:  obj.SecText,
		Align: 16,
	})
	b.data = b.file.AddSection(&obj.Section{
		Name:  ".data",
		Kind:  obj.SecData,
		Align: 8,
	})
	b.rodata = b.file.AddSection(&obj.Section{
		Name:  ".rodata",
		Kind:  obj.SecReadOnly,
		Align: 8,
	})

	// Create symbols
	for gVar := range b.module.GlobalVars() {
		b.CreateGlobalVar(gVar)
	}

	for fun := range b.module.Functions() {
		b.CreateFunction(fun)
	}

	// Generate symbols (initializers, instructions)
	for gVar := range b.module.GlobalVars() {
		b.GenerateGlobalVar(gVar)
	}

	for fun := range b.module.Functions() {
		b.GenerateFunction(fun)
	}

	// Finish
	b.text.Data, b.text.Relocations = b.asm.Assemble()

	return b.file
}

func (b *backend) GetFuncSymbol(name string) *obj.Symbol {
	// Check cache
	if symbol, ok := b.symbols[name]; ok {
		return symbol
	}

	// Check all symbols in object file
	for _, symbol := range b.file.Symbols {
		if symbol.Name == name {
			return symbol
		}
	}

	// Add external symbol
	symbol := b.file.AddSymbol(&obj.Symbol{
		Name:  name,
		Kind:  obj.SymFunction,
		Scope: obj.ScopeGlobal,
	})

	b.symbols[name] = symbol
	return symbol
}
