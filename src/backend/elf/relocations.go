package elf

import (
	"encoding/binary"
	"fireball/backend/amd64/asm"
)

func BuildRelaSection(targetName string, targetIndex uint32, symtab *SectionHeader, relocations []asm.Relocation) *SectionHeader {
	data := make([]uint8, 0, len(relocations)*24)

	for _, r := range relocations {
		var symIdx uint32
		if r.Symbol == "text" || r.Symbol == "msg" {
			symIdx = 1 // refers to symbol 'msg'
		}

		rType := uint32(2) // R_X86_64_PC32
		info := (uint64(symIdx) << 32) | uint64(rType)

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
