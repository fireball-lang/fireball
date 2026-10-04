package elf

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fireball/core"
	"fmt"
	"io"
	"os"

	"fireball/backend/obj"
)

const (
	grpCOMDAT = 0x1
)

type section struct {
	header elf.Section64
	name   string
	data   []byte
}

type groupInfo struct {
	section *section
	target  *obj.Section
}

type writer struct {
	file  *obj.File
	order binary.ByteOrder

	sections   []*section
	sectionMap map[*obj.Section]uint32

	symbols       []*obj.Symbol
	symbolIndices map[*obj.Symbol]uint32
	groups        map[*obj.Section]*groupInfo

	symtabSectionIndex   uint32
	strtabSectionIndex   uint32
	shstrtabSectionIndex uint16

	shOff uint64
}

func Write(f *obj.File, w io.Writer) error {
	defer core.Scope()()

	if f.Arch != obj.AMD64 {
		return fmt.Errorf("elf: unsupported architecture %v", f.Arch)
	}

	wr := &writer{
		file:   f,
		order:  binary.LittleEndian,
		groups: make(map[*obj.Section]*groupInfo),
	}

	if err := wr.CollectSections(); err != nil {
		return err
	}
	if err := wr.BuildSymbolAndStringTable(); err != nil {
		return err
	}
	if err := wr.BuildRelocations(); err != nil {
		return err
	}
	wr.BuildSectionNameTable()
	wr.Layout()

	return wr.Emit(w)
}

func WriteTo(f *obj.File, path string) (err error) {
	defer core.Scope()()

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

func (w *writer) CollectSections() error {
	// Section 0 is always SHN_UNDEF
	w.sections = append(w.sections, &section{
		header: elf.Section64{Type: uint32(elf.SHT_NULL)},
	})

	w.sectionMap = make(map[*obj.Section]uint32)

	for _, sec := range w.file.Sections {
		name := sec.Name
		if name == "" {
			name = DefaultSectionName(sec.Kind)
		}

		shType := uint32(elf.SHT_PROGBITS)
		var flags uint64

		switch sec.Kind {
		case obj.SecText:
			flags = uint64(elf.SHF_ALLOC | elf.SHF_EXECINSTR)
		case obj.SecData:
			flags = uint64(elf.SHF_ALLOC | elf.SHF_WRITE)
		case obj.SecReadOnly:
			flags = uint64(elf.SHF_ALLOC)
		case obj.SecBSS:
			shType = uint32(elf.SHT_NOBITS)
			flags = uint64(elf.SHF_ALLOC | elf.SHF_WRITE)
		default:
			return fmt.Errorf("elf.writer.CollectSections() - unknown section kind: %v", sec.Kind)
		}

		var groupSec *section
		if sec.Deduplicate {
			flags |= uint64(elf.SHF_GROUP)

			groupSec = &section{
				name: ".group",
				header: elf.Section64{
					Type:      uint32(elf.SHT_GROUP),
					Addralign: 4,
					Entsize:   4,
				},
			}

			w.sections = append(w.sections, groupSec)
		}

		align := sec.Align
		if align == 0 {
			align = 1
		}

		size := uint64(len(sec.Data))
		if sec.Kind == obj.SecBSS && sec.Size > 0 {
			size = sec.Size
		}

		elfSec := &section{
			name: name,
			data: sec.Data,
			header: elf.Section64{
				Type:      shType,
				Flags:     flags,
				Size:      size,
				Addralign: align,
			},
		}

		idx := uint32(len(w.sections))
		w.sectionMap[sec] = idx
		w.sections = append(w.sections, elfSec)

		if groupSec != nil {
			groupData := make([]byte, 8)
			w.order.PutUint32(groupData[0:4], grpCOMDAT)
			w.order.PutUint32(groupData[4:8], idx)

			groupSec.data = groupData
			groupSec.header.Size = 8

			w.groups[sec] = &groupInfo{
				section: groupSec,
				target:  sec,
			}
		}
	}

	return nil
}

func (w *writer) BuildSymbolAndStringTable() error {
	seen := make(map[*obj.Symbol]bool)

	var locals []*obj.Symbol
	var globals []*obj.Symbol

	addSymbol := func(sym *obj.Symbol) {
		if sym == nil || seen[sym] {
			return
		}

		seen[sym] = true
		w.symbols = append(w.symbols, sym)

		if sym.Scope == obj.ScopeLocal {
			locals = append(locals, sym)
		} else {
			globals = append(globals, sym)
		}
	}

	for _, s := range w.file.Symbols {
		addSymbol(s)
	}

	for _, sec := range w.file.Sections {
		for _, rel := range sec.Relocations {
			if rel.Target == nil {
				return fmt.Errorf("elf.writer.BuildSymbolAndStringTable() - relocation has a nil symbol target")
			}

			addSymbol(rel.Target)
		}
	}

	strtab := []byte{0x00}
	strOffsets := make(map[string]uint32)

	internString := func(s string) uint32 {
		if s == "" {
			return 0
		}

		if off, ok := strOffsets[s]; ok {
			return off
		}

		off := uint32(len(strtab))
		strtab = append(strtab, s...)
		strtab = append(strtab, 0x00)

		strOffsets[s] = off
		return off
	}

	symtabBuf := new(bytes.Buffer)
	w.symbolIndices = make(map[*obj.Symbol]uint32)

	// Symbol 0 is STN_UNDEF
	if err := binary.Write(symtabBuf, w.order, &elf.Sym64{}); err != nil {
		return err
	}

	currIdx := uint32(1)

	writeSym := func(sym *obj.Symbol) error {
		w.symbolIndices[sym] = currIdx
		currIdx++

		var binding uint8
		switch sym.Scope {
		case obj.ScopeLocal:
			binding = uint8(elf.STB_LOCAL)
		case obj.ScopeGlobal:
			binding = uint8(elf.STB_GLOBAL)
		}

		var symType uint8
		switch sym.Kind {
		case obj.SymNone:
			symType = uint8(elf.STT_NOTYPE)
		case obj.SymFunction:
			symType = uint8(elf.STT_FUNC)
		case obj.SymData:
			symType = uint8(elf.STT_OBJECT)
		case obj.SymSection:
			symType = uint8(elf.STT_SECTION)
		}

		var shndx uint16
		if sym.Section != nil {
			idx, ok := w.sectionMap[sym.Section]
			if !ok {
				return fmt.Errorf("elf.writer.BuildSymbolAndStringTable() - Symbol %q references unknown section", sym.Name)
			}
			shndx = uint16(idx)
		}

		entry := elf.Sym64{
			Name:  internString(sym.Name),
			Info:  (binding << 4) | (symType & 0x0F),
			Shndx: shndx,
			Value: sym.Value,
			Size:  sym.Size,
		}

		return binary.Write(symtabBuf, w.order, &entry)
	}

	for _, sym := range locals {
		if err := writeSym(sym); err != nil {
			return err
		}
	}

	firstGlobalIndex := currIdx

	for _, sym := range globals {
		if err := writeSym(sym); err != nil {
			return err
		}
	}

	strtabSec := &section{
		name: ".strtab",
		data: strtab,
		header: elf.Section64{
			Type:      uint32(elf.SHT_STRTAB),
			Addralign: 1,
			Size:      uint64(len(strtab)),
		},
	}

	symtabSec := &section{
		name: ".symtab",
		data: symtabBuf.Bytes(),
		header: elf.Section64{
			Type:      uint32(elf.SHT_SYMTAB),
			Info:      firstGlobalIndex,
			Addralign: 8,
			Entsize:   24,
			Size:      uint64(symtabBuf.Len()),
		},
	}

	w.symtabSectionIndex = uint32(len(w.sections))
	w.sections = append(w.sections, symtabSec)

	w.strtabSectionIndex = uint32(len(w.sections))
	w.sections = append(w.sections, strtabSec)

	symtabSec.header.Link = w.strtabSectionIndex

	for _, group := range w.groups {
		group.section.header.Link = w.symtabSectionIndex

		var sigSym *obj.Symbol
		for _, sym := range w.symbols {
			if sym.Section == group.target && sym.Scope == obj.ScopeGlobal {
				sigSym = sym
				break
			}
		}

		if sigSym == nil {
			return fmt.Errorf("elf.writer.BuildSymbolAndStringTable() - deduplicated section %q has no global signature symbol", group.target.Name)
		}

		group.section.header.Info = w.symbolIndices[sigSym]
	}

	return nil
}

func (w *writer) BuildRelocations() error {
	for _, sec := range w.file.Sections {
		if len(sec.Relocations) == 0 {
			continue
		}

		targetSecIdx := w.sectionMap[sec]
		secName := sec.Name
		if secName == "" {
			secName = DefaultSectionName(sec.Kind)
		}

		var relaBuf bytes.Buffer

		for _, r := range sec.Relocations {
			if r.Target == nil {
				return fmt.Errorf("elf.writer.BuildRelocations() - relocation has a nil symbol target")
			}

			symIdx, ok := w.symbolIndices[r.Target]
			if !ok {
				return fmt.Errorf("elf.writer.BuildRelocations() - invalid symbol: %s", r.Target.Name)
			}

			var rType uint32
			switch r.Kind {
			case obj.RelocAbs64:
				rType = uint32(elf.R_X86_64_64)
			case obj.RelocPC32:
				rType = uint32(elf.R_X86_64_PC32)
			case obj.RelocAbs32:
				rType = uint32(elf.R_X86_64_32)
			case obj.RelocSigned32:
				rType = uint32(elf.R_X86_64_32S)
			default:
				return fmt.Errorf("elf.writer.BuildRelocations() - invalid relocation kind: %d", r.Kind)
			}

			addend := r.Addend
			if r.Kind == obj.RelocPC32 {
				addend = r.Addend - 4 - int64(r.TrailingBytes)
			}

			rela := elf.Rela64{
				Off:    uint64(r.Offset),
				Info:   (uint64(symIdx) << 32) | uint64(rType),
				Addend: addend,
			}

			if err := binary.Write(&relaBuf, w.order, &rela); err != nil {
				return err
			}
		}

		flags := uint64(elf.SHF_INFO_LINK)
		if sec.Deduplicate {
			flags |= uint64(elf.SHF_GROUP)
		}

		relaSecIdx := uint32(len(w.sections))
		relaSec := &section{
			name: ".rela" + secName,
			data: relaBuf.Bytes(),
			header: elf.Section64{
				Type:      uint32(elf.SHT_RELA),
				Flags:     flags,
				Link:      w.symtabSectionIndex,
				Info:      targetSecIdx,
				Addralign: 8,
				Entsize:   24,
				Size:      uint64(relaBuf.Len()),
			},
		}

		w.sections = append(w.sections, relaSec)

		// Include the .rela section in the group payload
		if sec.Deduplicate {
			group := w.groups[sec]

			var idxBytes [4]byte
			w.order.PutUint32(idxBytes[:], relaSecIdx)
			group.section.data = append(group.section.data, idxBytes[:]...)
			group.section.header.Size = uint64(len(group.section.data))
		}
	}

	return nil
}

func (w *writer) BuildSectionNameTable() {
	shstrtab := []byte{0x00}
	strOffsets := make(map[string]uint32)

	intern := func(s string) uint32 {
		if s == "" {
			return 0
		}

		if off, ok := strOffsets[s]; ok {
			return off
		}

		off := uint32(len(shstrtab))
		shstrtab = append(shstrtab, s...)
		shstrtab = append(shstrtab, 0x00)

		strOffsets[s] = off
		return off
	}

	for _, sec := range w.sections {
		sec.header.Name = intern(sec.name)
	}

	shstrtabSec := &section{
		name: ".shstrtab",
		header: elf.Section64{
			Type:      uint32(elf.SHT_STRTAB),
			Addralign: 1,
		},
	}
	shstrtabSec.header.Name = intern(".shstrtab")

	w.shstrtabSectionIndex = uint16(len(w.sections))
	w.sections = append(w.sections, shstrtabSec)

	shstrtabSec.data = shstrtab
	shstrtabSec.header.Size = uint64(len(shstrtab))
}

func (w *writer) Layout() {
	const ehdrSize = 64
	offset := uint64(ehdrSize)

	for _, sec := range w.sections {
		if sec.header.Type == uint32(elf.SHT_NULL) {
			continue
		}

		if sec.header.Addralign > 1 {
			offset = obj.AlignUp(offset, sec.header.Addralign)
		}

		sec.header.Off = offset

		if sec.header.Type != uint32(elf.SHT_NOBITS) {
			offset += uint64(len(sec.data))
		}
	}

	offset = obj.AlignUp(offset, 8)
	w.shOff = offset
}

func (w *writer) Emit(out io.Writer) error {
	bw := &obj.CountedWriter{Out: out}

	hdr := elf.Header64{
		Ident: [16]byte{
			0x7F, 'E', 'L', 'F',
			byte(elf.ELFCLASS64),
			byte(elf.ELFDATA2LSB),
			byte(elf.EV_CURRENT),
			byte(elf.ELFOSABI_NONE),
			0,
		},
		Type:      uint16(elf.ET_REL),
		Machine:   uint16(elf.EM_X86_64),
		Version:   uint32(elf.EV_CURRENT),
		Shoff:     w.shOff,
		Ehsize:    64,
		Shentsize: 64,
		Shnum:     uint16(len(w.sections)),
		Shstrndx:  w.shstrtabSectionIndex,
	}

	if err := binary.Write(bw, w.order, &hdr); err != nil {
		return err
	}

	// Payloads
	for _, sec := range w.sections {
		if sec.header.Type == uint32(elf.SHT_NULL) || sec.header.Type == uint32(elf.SHT_NOBITS) || len(sec.data) == 0 {
			continue
		}

		if err := bw.PadTo(sec.header.Off); err != nil {
			return err
		}

		if _, err := bw.Write(sec.data); err != nil {
			return err
		}
	}

	// Pad to Section Header Table
	if err := bw.PadTo(w.shOff); err != nil {
		return err
	}

	// Section Headers Table directly serialized
	for _, sec := range w.sections {
		if err := binary.Write(bw, w.order, &sec.header); err != nil {
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
		return ".rodata"
	case obj.SecBSS:
		return ".bss"
	default:
		return ".data"
	}
}
