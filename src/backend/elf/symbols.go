package elf

import (
	"encoding/binary"
	"fireball/backend/amd64/asm"
)

type SymBinding uint8

const (
	BindingLocal SymBinding = iota
	BindingGlobal
	BindingWeak
)

type SymType uint8

const (
	TypeNoType SymType = iota
	TypeObject
	TypeFunc
	TypeSection
)

type Symbol struct {
	Name string

	Binding SymBinding
	Type    SymType

	Section *SectionHeader
	Value   uint64
	Size    uint64
}

type SymbolTable struct {
	symbols []Symbol
}

func (st *SymbolTable) AddDefs(section *SectionHeader, defs []asm.SymbolDef) {
	for _, def := range defs {
		type_ := TypeNoType
		if def.Function {
			type_ = TypeFunc
		}

		binding := BindingLocal
		if def.Global {
			binding = BindingGlobal
		}

		st.symbols = append(st.symbols, Symbol{
			Name:    def.Name,
			Binding: binding,
			Type:    type_,
			Section: section,
			Value:   uint64(def.Offset),
			Size:    0,
		})
	}
}

func (st *SymbolTable) Add(symbol Symbol) {
	st.symbols = append(st.symbols, symbol)
}

func (st *SymbolTable) Build(sections []*SectionHeader) (symtabSec *SectionHeader, strtabSec *SectionHeader, symIndices map[string]uint32) {
	symIndices = make(map[string]uint32)

	// String table starts with empty string at offset 0
	strtab := []byte{0}

	addString := func(s string) uint32 {
		if s == "" {
			return 0
		}

		offset := uint32(len(strtab))
		strtab = append(strtab, s...)
		strtab = append(strtab, 0)

		return offset
	}

	// 1. Separate locals and globals
	var locals []*Symbol
	var globals []*Symbol

	for i := range st.symbols {
		s := &st.symbols[i]

		if s.Binding == BindingLocal {
			locals = append(locals, s)
		} else {
			globals = append(globals, s)
		}
	}

	// Section lookup helper
	sectionIndex := func(target *SectionHeader) uint16 {
		if target == nil {
			return 0 // SHN_UNDEF
		}

		for i, sec := range sections {
			if sec == target {
				// +1 if section 0 is the implicit NULL section in your writer
				return uint16(i + 1)
			}
		}

		panic("elf.SymbolTable.Build() - Symbol section unknown")
	}

	// 2. Build sorted symbols slice: [UNDEF, locals..., globals...]
	var rawData []byte

	appendSym := func(name uint32, info uint8, shndx uint16, val, size uint64) {
		rawData = binary.LittleEndian.AppendUint32(rawData, name)
		rawData = append(rawData, info, 0) // info, other
		rawData = binary.LittleEndian.AppendUint16(rawData, shndx)
		rawData = binary.LittleEndian.AppendUint64(rawData, val)
		rawData = binary.LittleEndian.AppendUint64(rawData, size)
	}

	// Index 0: STN_UNDEF
	appendSym(0, 0, 0, 0, 0)

	currIndex := uint32(1)

	processSym := func(sym *Symbol) {
		nameOffset := addString(sym.Name)
		if sym.Name != "" {
			symIndices[sym.Name] = currIndex
		}

		info := (uint8(sym.Binding) << 4) | (uint8(sym.Type) & 0x0f)
		appendSym(nameOffset, info, sectionIndex(sym.Section), sym.Value, sym.Size)
		currIndex++
	}

	for _, sym := range locals {
		processSym(sym)
	}

	firstGlobalIndex := currIndex

	for _, sym := range globals {
		processSym(sym)
	}

	strtabSec = &SectionHeader{
		Name:         ".strtab",
		Type:         ShtStringTable,
		AddressAlign: 8,
		Data:         strtab,
	}

	symtabSec = &SectionHeader{
		Name:         ".symtab",
		Type:         ShtSymbolTable,
		AddressAlign: 8,
		Link:         strtabSec,
		Info:         firstGlobalIndex,
		EntrySize:    24,
		Data:         rawData,
	}

	return
}
