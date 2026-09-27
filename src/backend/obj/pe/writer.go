package pe

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"

	"fireball/backend/obj"
)

// Missing COFF specification constants not exported by debug/pe.
//
//goland:noinspection GoUnusedConst,GoSnakeCaseUsage
const (
	// AMD64 Relocation Types
	imageRelAMD64Addr64  = 0x0001
	imageRelAMD64Addr32  = 0x0002
	imageRelAMD64Rel32   = 0x0004
	imageRelAMD64Rel32_1 = 0x0005
	imageRelAMD64Rel32_2 = 0x0006
	imageRelAMD64Rel32_3 = 0x0007
	imageRelAMD64Rel32_4 = 0x0008
	imageRelAMD64Rel32_5 = 0x0009

	// Symbol Storage Classes
	imageSymClassExternal = 2
	imageSymClassStatic   = 3

	// Symbol Derived Type: Function (0x20 = 2 << 4)
	imageSymDTypeFunction = 0x0020
)

type section struct {
	objSec *obj.Section
	header pe.SectionHeader32
	name   string
	data   []byte
	relocs []pe.Reloc
}

type writer struct {
	file  *obj.File
	order binary.ByteOrder

	sections   []*section
	sectionMap map[*obj.Section]int16 // 1-based index (0 is UNDEF in COFF)

	symbols       []*obj.Symbol
	symbolIndices map[*obj.Symbol]uint32

	coffSymbols []pe.COFFSymbol
	strtab      []byte
	strOffsets  map[string]uint32

	symTableOff uint64
}

func Write(f *obj.File, w io.Writer) error {
	if f.Arch != obj.AMD64 {
		return fmt.Errorf("pe: unsupported architecture %v", f.Arch)
	}

	wr := &writer{
		file:          f,
		order:         binary.LittleEndian,
		sectionMap:    make(map[*obj.Section]int16),
		symbolIndices: make(map[*obj.Symbol]uint32),
		strOffsets:    make(map[string]uint32),
		strtab:        make([]byte, 4),
	}

	if err := wr.CollectSections(); err != nil {
		return err
	}
	if err := wr.CollectSymbols(); err != nil {
		return err
	}
	// Build symbols FIRST so symbols receive their exact COFF symbol table index
	if err := wr.BuildCOFFSymbols(); err != nil {
		return err
	}
	// Relocations now resolve to the correct symbol indices
	if err := wr.BakeAddendsAndBuildRelocations(); err != nil {
		return err
	}
	wr.FinalizeStringTable()
	if err := wr.Layout(); err != nil {
		return err
	}

	return wr.Emit(w)
}

func WriteTo(f *obj.File, path string) (err error) {
	var file *os.File

	file, err = os.Create(path)
	if err != nil {
		return err
	}

	defer func() {
		var closeErr error

		if closeErr = file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	return Write(f, file)
}

func (w *writer) InternString(s string) uint32 {
	if off, ok := w.strOffsets[s]; ok {
		return off
	}

	off := uint32(len(w.strtab))
	w.strtab = append(w.strtab, s...)
	w.strtab = append(w.strtab, 0x00)

	w.strOffsets[s] = off
	return off
}

func (w *writer) CollectSections() error {
	for i, sec := range w.file.Sections {
		name := sec.Name
		if name == "" {
			name = DefaultSectionName(sec.Kind)
		}

		var flags uint32
		switch sec.Kind {
		case obj.SecText:
			flags = pe.IMAGE_SCN_CNT_CODE | pe.IMAGE_SCN_MEM_EXECUTE | pe.IMAGE_SCN_MEM_READ
		case obj.SecData:
			flags = pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ | pe.IMAGE_SCN_MEM_WRITE
		case obj.SecReadOnly:
			flags = pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ
		case obj.SecBSS:
			flags = pe.IMAGE_SCN_CNT_UNINITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ | pe.IMAGE_SCN_MEM_WRITE
		default:
			return fmt.Errorf("pe.writer.CollectSections() - unknown section kind: %v", sec.Kind)
		}

		flags |= sectionAlignFlag(sec.Align)

		if sec.Deduplicate {
			flags |= pe.IMAGE_SCN_LNK_COMDAT
		}

		var dataCopy []byte
		if sec.Kind != obj.SecBSS && len(sec.Data) > 0 {
			dataCopy = make([]byte, len(sec.Data))
			copy(dataCopy, sec.Data)
		}

		var rawSize uint32
		if sec.Kind == obj.SecBSS {
			if sec.Size > 0 {
				rawSize = uint32(sec.Size)
			} else {
				rawSize = uint32(len(sec.Data))
			}
		} else {
			rawSize = uint32(len(dataCopy))
		}

		var nameBytes [8]byte
		if len(name) <= 8 {
			copy(nameBytes[:], name)
		} else {
			off := w.InternString(name)
			s := fmt.Sprintf("/%d", off)
			if len(s) > 8 {
				return fmt.Errorf("pe.writer.CollectSections() - string table offset %d exceeds 7 decimal digits", off)
			}
			copy(nameBytes[:], s)
		}

		peSec := &section{
			objSec: sec,
			name:   name,
			data:   dataCopy,
			header: pe.SectionHeader32{
				Name:            nameBytes,
				SizeOfRawData:   rawSize,
				Characteristics: flags,
			},
		}

		w.sections = append(w.sections, peSec)
		w.sectionMap[sec] = int16(i + 1)
	}

	return nil
}

func (w *writer) CollectSymbols() error {
	seen := make(map[*obj.Symbol]bool)

	addSymbol := func(sym *obj.Symbol) {
		if sym == nil || seen[sym] {
			return
		}

		seen[sym] = true
		w.symbols = append(w.symbols, sym)
	}

	for _, sym := range w.file.Symbols {
		addSymbol(sym)
	}

	for _, sec := range w.file.Sections {
		for _, rel := range sec.Relocations {
			if rel.Target == nil {
				return fmt.Errorf("pe.writer.CollectSymbols() - relocation in section %q has nil target", sec.Name)
			}

			addSymbol(rel.Target)
		}
	}

	return nil
}

func (w *writer) BuildCOFFSymbols() error {
	// 1. Emit section symbol + aux symbol for COMDAT sections
	for _, sec := range w.sections {
		if !sec.objSec.Deduplicate {
			continue
		}

		secNum := w.sectionMap[sec.objSec]

		// Static section definition symbol
		w.coffSymbols = append(w.coffSymbols, pe.COFFSymbol{
			Name:               sec.header.Name,
			Value:              0,
			SectionNumber:      secNum,
			Type:               0,
			StorageClass:       imageSymClassStatic,
			NumberOfAuxSymbols: 1,
		})

		// 18-byte IMAGE_AUX_SECTION_DEFINITION
		var aux [pe.COFFSymbolSize]byte
		w.order.PutUint32(aux[0:4], sec.header.SizeOfRawData)
		w.order.PutUint16(aux[4:6], uint16(len(sec.objSec.Relocations)))
		w.order.PutUint16(aux[12:14], uint16(secNum))
		aux[14] = pe.IMAGE_COMDAT_SELECT_ANY

		var auxSym pe.COFFSymbol
		if err := binary.Read(bytes.NewReader(aux[:]), w.order, &auxSym); err != nil {
			return err
		}

		w.coffSymbols = append(w.coffSymbols, auxSym)
	}

	// 2. Emit regular user symbols and record their true symbol indices
	for _, sym := range w.symbols {
		if len(sym.Name) == 0 {
			return fmt.Errorf("pe.writer.BuildCOFFSymbols() - symbol cannot have empty name")
		}

		var name [8]byte
		if len(sym.Name) <= 8 {
			copy(name[:], sym.Name)
		} else {
			off := w.InternString(sym.Name)
			w.order.PutUint32(name[4:8], off)
		}

		var secNum int16
		if sym.Section != nil {
			num, ok := w.sectionMap[sym.Section]
			if !ok {
				return fmt.Errorf("pe.writer.BuildCOFFSymbols() - symbol %q references unknown section", sym.Name)
			}
			secNum = num
		}

		var storageClass uint8
		switch sym.Scope {
		case obj.ScopeLocal:
			storageClass = imageSymClassStatic
		case obj.ScopeGlobal:
			storageClass = imageSymClassExternal
		default:
			return fmt.Errorf("pe.writer.BuildCOFFSymbols() - unknown symbol scope: %v", sym.Scope)
		}

		var symType uint16
		if sym.Kind == obj.SymFunction {
			symType = imageSymDTypeFunction
		}

		w.symbolIndices[sym] = uint32(len(w.coffSymbols))
		w.coffSymbols = append(w.coffSymbols, pe.COFFSymbol{
			Name:               name,
			Value:              uint32(sym.Value),
			SectionNumber:      secNum,
			Type:               symType,
			StorageClass:       storageClass,
			NumberOfAuxSymbols: 0,
		})
	}

	return nil
}

func (w *writer) BakeAddendsAndBuildRelocations() error {
	for _, sec := range w.sections {
		if len(sec.objSec.Relocations) == 0 {
			continue
		}

		if len(sec.objSec.Relocations) > math.MaxUint16 {
			return fmt.Errorf("pe.writer.BakeAddendsAndBuildRelocations() - section %q exceeds max 16-bit relocation count (%d)", sec.name, len(sec.objSec.Relocations))
		}

		for _, r := range sec.objSec.Relocations {
			symIdx, ok := w.symbolIndices[r.Target]
			if !ok {
				return fmt.Errorf("pe.writer.BakeAddendsAndBuildRelocations() - unknown symbol %q in relocation", r.Target.Name)
			}

			var rType uint16
			switch r.Kind {
			case obj.RelocAbs64:
				rType = imageRelAMD64Addr64

				if r.Offset+8 > len(sec.data) {
					return fmt.Errorf("pe.writer.BakeAddendsAndBuildRelocations() - relocation offset %d out of bounds for section %q (len %d)", r.Offset, sec.name, len(sec.data))
				}

				existing := int64(w.order.Uint64(sec.data[r.Offset : r.Offset+8]))
				w.order.PutUint64(sec.data[r.Offset:r.Offset+8], uint64(existing+r.Addend))

			case obj.RelocAbs32, obj.RelocSigned32:
				rType = imageRelAMD64Addr32

				if r.Offset+4 > len(sec.data) {
					return fmt.Errorf("pe.writer.BakeAddendsAndBuildRelocations() - relocation offset %d out of bounds for section %q (len %d)", r.Offset, sec.name, len(sec.data))
				}

				existing := int32(w.order.Uint32(sec.data[r.Offset : r.Offset+4]))
				w.order.PutUint32(sec.data[r.Offset:r.Offset+4], uint32(existing+int32(r.Addend)))

			case obj.RelocPC32:
				if r.TrailingBytes > 5 {
					return fmt.Errorf("pe.writer.BakeAddendsAndBuildRelocations() - trailing bytes %d exceeds max supported 5 for AMD64 REL32", r.TrailingBytes)
				}

				rType = imageRelAMD64Rel32 + uint16(r.TrailingBytes)

				if r.Offset+4 > len(sec.data) {
					return fmt.Errorf("pe.writer.BakeAddendsAndBuildRelocations() - relocation offset %d out of bounds for section %q (len %d)", r.Offset, sec.name, len(sec.data))
				}

				existing := int32(w.order.Uint32(sec.data[r.Offset : r.Offset+4]))
				w.order.PutUint32(sec.data[r.Offset:r.Offset+4], uint32(existing+int32(r.Addend)))

			default:
				return fmt.Errorf("pe.writer.BakeAddendsAndBuildRelocations() - unsupported relocation kind %v", r.Kind)
			}

			sec.relocs = append(sec.relocs, pe.Reloc{
				VirtualAddress:   uint32(r.Offset),
				SymbolTableIndex: symIdx,
				Type:             rType,
			})
		}
	}

	return nil
}

func (w *writer) FinalizeStringTable() {
	w.order.PutUint32(w.strtab[0:4], uint32(len(w.strtab)))
}

func (w *writer) Layout() error {
	const fileHdrSize = 20
	const secHdrSize = 40

	offset := uint64(fileHdrSize + len(w.sections)*secHdrSize)

	// Layout section raw data
	for _, sec := range w.sections {
		if sec.objSec.Kind == obj.SecBSS {
			sec.header.PointerToRawData = 0
			continue
		}

		if len(sec.data) == 0 {
			sec.header.PointerToRawData = 0
			sec.header.SizeOfRawData = 0
			continue
		}

		align := max(sec.objSec.Align, 4)
		offset = obj.AlignUp(offset, align)

		sec.header.PointerToRawData = uint32(offset)
		sec.header.SizeOfRawData = uint32(len(sec.data))
		offset += uint64(len(sec.data))
	}

	// Layout relocations
	for _, sec := range w.sections {
		if len(sec.relocs) == 0 {
			continue
		}

		offset = obj.AlignUp(offset, 4)
		sec.header.PointerToRelocations = uint32(offset)
		sec.header.NumberOfRelocations = uint16(len(sec.relocs))
		offset += uint64(len(sec.relocs) * 10)
	}

	// Layout symbol table & string table
	if len(w.coffSymbols) > 0 || len(w.strtab) > 4 {
		offset = obj.AlignUp(offset, 4)
		w.symTableOff = offset
		offset += uint64(len(w.coffSymbols) * pe.COFFSymbolSize)
	}

	return nil
}

func (w *writer) Emit(out io.Writer) error {
	bw := &obj.CountedWriter{Out: out}

	hdr := pe.FileHeader{
		Machine:              pe.IMAGE_FILE_MACHINE_AMD64,
		NumberOfSections:     uint16(len(w.sections)),
		TimeDateStamp:        0,
		PointerToSymbolTable: uint32(w.symTableOff),
		NumberOfSymbols:      uint32(len(w.coffSymbols)),
		SizeOfOptionalHeader: 0,
		Characteristics:      0,
	}

	if err := binary.Write(bw, w.order, &hdr); err != nil {
		return err
	}

	for _, sec := range w.sections {
		if err := binary.Write(bw, w.order, &sec.header); err != nil {
			return err
		}
	}

	// Write section raw data
	for _, sec := range w.sections {
		if sec.header.PointerToRawData == 0 || len(sec.data) == 0 {
			continue
		}

		if err := bw.PadTo(uint64(sec.header.PointerToRawData)); err != nil {
			return err
		}

		if _, err := bw.Write(sec.data); err != nil {
			return err
		}
	}

	// Write per-section relocations
	for _, sec := range w.sections {
		if len(sec.relocs) == 0 {
			continue
		}

		if err := bw.PadTo(uint64(sec.header.PointerToRelocations)); err != nil {
			return err
		}

		for _, rel := range sec.relocs {
			if err := binary.Write(bw, w.order, &rel); err != nil {
				return err
			}
		}
	}

	// Write symbol table & string table
	if w.symTableOff > 0 {
		if err := bw.PadTo(w.symTableOff); err != nil {
			return err
		}

		for _, sym := range w.coffSymbols {
			if err := binary.Write(bw, w.order, &sym); err != nil {
				return err
			}
		}

		// Write string table immediately following symbols
		if _, err := bw.Write(w.strtab); err != nil {
			return err
		}
	}

	return nil
}

func DefaultSectionName(kind obj.SectionKind) string {
	switch kind {
	case obj.SecText:
		return ".text"
	case obj.SecData:
		return ".data"
	case obj.SecReadOnly:
		return ".rdata"
	case obj.SecBSS:
		return ".bss"
	default:
		return ".data"
	}
}

func sectionAlignFlag(align uint64) uint32 {
	if align <= 1 {
		return 0x00100000 // IMAGE_SCN_ALIGN_1BYTES
	}

	tz := bits.TrailingZeros64(align)

	// Max standard alignment: 8192 bytes (IMAGE_SCN_ALIGN_8192BYTES)
	tz = min(tz, 13)

	return uint32(tz+1) << 20
}
