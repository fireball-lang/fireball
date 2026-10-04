package amd64

import (
	"encoding/binary"
	"fireball/backend/obj"
	"fireball/ir"
	"math"
	"strconv"
	"unicode/utf8"
)

func (b *backend) CreateGlobalVar(gVar *ir.GlobalVar) {
	// External symbol
	if gVar.Flags&ir.External != 0 {
		b.gVarSymbols[gVar] = b.file.AddSymbol(&obj.Symbol{
			Name:  gVar.Name,
			Kind:  obj.SymData,
			Scope: obj.ScopeGlobal,
		})

		return
	}

	// Get target section and byte slice
	info := gVar.Typ.Info()
	section := b.data

	var start uint64

	if gVar.Flags&ir.Constant != 0 {
		section = b.rodata
	}

	if gVar.Flags&ir.LinkOnce != 0 {
		section = b.file.AddSection(&obj.Section{
			Name:        section.Name,
			Kind:        section.Kind,
			Align:       uint64(info.Align),
			Deduplicate: true,
			Data:        make([]byte, info.Size),
		})
	} else {
		start = b.PrepareSectionData(section, info)
	}

	// Create symbol
	scope := obj.ScopeGlobal
	if gVar.Flags&ir.Private != 0 {
		scope = obj.ScopeLocal
	}

	b.gVarSymbols[gVar] = b.file.AddSymbol(&obj.Symbol{
		Name:    gVar.Name,
		Kind:    obj.SymData,
		Scope:   scope,
		Section: section,
		Value:   start,
		Size:    uint64(info.Size),
	})
}

func (b *backend) PrepareSectionData(section *obj.Section, info ir.TypeInfo) uint64 {
	start := obj.AlignUp(uint64(len(section.Data)), uint64(info.Align))
	end := start + uint64(info.Size)

	for uint64(len(section.Data)) < end {
		section.Data = append(section.Data, 0)
	}

	return start
}

func (b *backend) GenerateGlobalVar(gVar *ir.GlobalVar) {
	// Skip external
	if gVar.Flags&ir.External != 0 {
		return
	}

	// Fill in initializer
	symbol := b.gVarSymbols[gVar]
	data := symbol.Section.Data[symbol.Value : symbol.Value+symbol.Size]

	b.WriteIrValue(gVar.Initializer, symbol.Section, symbol.Value, data, 0)
}

func (b *backend) WriteIrValue(value ir.Value, section *obj.Section, start uint64, data []uint8, off uint32) uint32 {
	switch init := value.(type) {
	case *ir.ZeroInitializer, *ir.Null:
		// nop, already zero initialized
		return off + value.Type().Info().Size

	case *ir.Integer:
		size := value.Type().Info().Size
		val := init.Value.TwosComplement()

		switch size {
		case 1:
			data[off] = uint8(val & 0xFF)
		case 2:
			binary.LittleEndian.PutUint16(data[off:], uint16(val))
		case 4:
			binary.LittleEndian.PutUint32(data[off:], uint32(val))
		case 8:
			binary.LittleEndian.PutUint64(data[off:], val)
		default:
			panic("amd64.backend.WriteIrValue() - Invalid integer size: " + strconv.FormatUint(uint64(size), 10))
		}

		return off + size

	case *ir.FloatV:
		binary.LittleEndian.PutUint32(data[off:], math.Float32bits(init.Value))
		return off + 4

	case *ir.DoubleV:
		binary.LittleEndian.PutUint64(data[off:], math.Float64bits(init.Value))
		return off + 8

	case *ir.String:
		for _, ch := range init.Runes {
			bytes := utf8.EncodeRune(data[off:], ch)
			if bytes == -1 {
				panic("amd64.backend.WriteIrValue() - Invalid rune")
			}

			off += uint32(bytes)
		}

		return off

	case *ir.Vector:
		for _, element := range init.Elements {
			off = b.WriteIrValue(element, section, start, data, off)
		}

		return off

	case *ir.Array:
		for _, element := range init.Elements {
			off = b.WriteIrValue(element, section, start, data, off)
		}

		return off

	case *ir.Struct:
		offStart := off

		for i, range_ := range ir.GetStructFieldRanges(init.Typ) {
			off = b.WriteIrValue(init.Fields[i], section, start, data, offStart+uint32(range_.Offset))
		}

		return offStart + init.Typ.Info().Size

	case *ir.GlobalVar:
		section.Relocations = append(section.Relocations, obj.Relocation{
			Target: b.gVarSymbols[init],
			Kind:   obj.RelocAbs64,
			Offset: int(start + uint64(off)),
		})

		return off + 8

	case *ir.Function:
		section.Relocations = append(section.Relocations, obj.Relocation{
			Target: b.funSymbols[init],
			Kind:   obj.RelocAbs64,
			Offset: int(start + uint64(off)),
		})

		return off + 8

	default:
		panic("amd64.backend.WriteIrValue() - Invalid value")
	}
}
