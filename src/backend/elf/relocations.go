package elf

import (
	"encoding/binary"
	"fireball/backend/amd64/asm"
)

func BuildRelaSection(targetName string, targetIndex uint32, symtab *SectionHeader, symIndices map[string]uint32, relocations []asm.Relocation) *SectionHeader {
	data := make([]uint8, 0, len(relocations)*24)

	for _, r := range relocations {
		idx, ok := symIndices[r.Symbol]
		if !ok {
			panic("elf.BuildRelaSection() - Unknown symbol: " + r.Symbol)
		}

		var rType uint32

		switch r.Type {
		case asm.RelocPC32:
			rType = 2 // R_X86_64_PC32
		case asm.RelocAbs64:
			rType = 1 // R_X86_64_64
		case asm.RelocAbs32:
			rType = 10 // R_X86_64_32
		case asm.RelocSigned32:
			rType = 11 // R_X86_64_32S
		default:
			panic("elf.BuildRelaSection() - Invalid RelocType")
		}

		info := (uint64(idx) << 32) | uint64(rType)

		data = binary.LittleEndian.AppendUint64(data, uint64(r.Offset))
		data = binary.LittleEndian.AppendUint64(data, info)
		data = binary.LittleEndian.AppendUint64(data, uint64(r.ElfAddend()))
	}

	return &SectionHeader{
		Name:      ".rela" + targetName,
		Type:      ShtRelocationsAddends,
		Flags:     ShfInfoLink,
		Link:      symtab,
		Info:      targetIndex,
		EntrySize: 24,
		Data:      data,
	}
}
