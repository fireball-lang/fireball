package elf

import (
	"cmp"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"slices"
)

type segmentMeta struct {
	offset uint64
	filesz uint64
	memsz  uint64
}

type writer struct {
	f *File

	w io.Writer
	n uint64

	order binary.ByteOrder

	ehSize    uint16
	phEntSize uint16
	shEntSize uint16

	sectionNameTableSection *SectionHeader
	nameOffsets             map[*SectionHeader]uint32

	sections                     []*SectionHeader
	sectionIndices               map[*SectionHeader]uint32
	sectionNameTableSectionIndex uint16

	sectionOffsets map[*SectionHeader]uint64
	segmentLayout  map[*ProgramHeader]segmentMeta
	shOff          uint64
}

func (f *File) WriteTo(path string) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	return f.Write(file)
}

func (f *File) Write(w io.Writer) error {
	wr := &writer{
		f: f,
		w: w,
	}

	switch f.Endianness {
	case Little:
		wr.order = binary.LittleEndian
	case Big:
		wr.order = binary.BigEndian
	default:
		panic("elf.File.Write() - Invalid endianness")
	}

	switch f.Class {
	case Bit32:
		wr.ehSize = 52
		wr.phEntSize = 32
		wr.shEntSize = 40

	case Bit64:
		wr.ehSize = 64
		wr.phEntSize = 56
		wr.shEntSize = 64

	default:
		panic("elf.File.Write() - Invalid class")
	}

	wr.BuildSectionNameTable()
	wr.CollectSections()
	wr.PrepareLayout()

	if err := wr.WriteElfHeader(); err != nil {
		return err
	}

	if err := wr.WriteProgramHeaders(); err != nil {
		return err
	}

	if err := wr.WritePayloads(); err != nil {
		return err
	}

	if err := wr.PadTo(wr.shOff); err != nil {
		return err
	}

	if err := wr.WriteSectionHeaders(); err != nil {
		return err
	}

	return nil
}

func (w *writer) BuildSectionNameTable() {
	data := []byte{0x00}
	strOffsets := make(map[string]uint32)
	w.nameOffsets = make(map[*SectionHeader]uint32)

	intern := func(s string) uint32 {
		if s == "" {
			return 0
		}
		if off, ok := strOffsets[s]; ok {
			return off
		}
		off := uint32(len(data))
		data = append(data, []byte(s)...)
		data = append(data, 0x00)
		strOffsets[s] = off
		return off
	}

	for _, header := range w.f.Headers {
		for _, section := range header.Sections {
			w.nameOffsets[section] = intern(section.Name)
		}
	}

	for _, section := range w.f.Sections {
		w.nameOffsets[section] = intern(section.Name)
	}

	w.sectionNameTableSection = &SectionHeader{
		Name:         ".shstrtab",
		Type:         ShtStringTable,
		AddressAlign: 1,
	}

	w.nameOffsets[w.sectionNameTableSection] = intern(".shstrtab")
	w.sectionNameTableSection.Data = data
}

func (w *writer) CollectSections() {
	nullSection := &SectionHeader{
		Name: "",
		Type: ShtNull,
	}

	w.sections = make([]*SectionHeader, 0, len(w.f.Sections)+len(w.f.Headers)+2)
	w.sections = append(w.sections, nullSection)

	seen := make(map[*SectionHeader]bool)
	seen[nullSection] = true

	add := func(sec *SectionHeader) {
		if sec == nil || seen[sec] {
			return
		}

		seen[sec] = true
		w.sections = append(w.sections, sec)
	}

	for _, ph := range w.f.Headers {
		for _, section := range ph.Sections {
			add(section)
		}
	}

	for _, section := range w.f.Sections {
		add(section)
	}

	if w.sectionNameTableSection != nil {
		w.sections = append(w.sections, w.sectionNameTableSection)
		w.sectionNameTableSectionIndex = uint16(len(w.sections) - 1)
	}

	// Cache section indices for fast Link lookups
	w.sectionIndices = make(map[*SectionHeader]uint32, len(w.sections))

	for idx, sec := range w.sections {
		w.sectionIndices[sec] = uint32(idx)
	}
}

func (w *writer) PrepareLayout() {
	w.sectionOffsets = make(map[*SectionHeader]uint64)
	w.segmentLayout = make(map[*ProgramHeader]segmentMeta)

	phCount := uint64(len(w.f.Headers))
	currentOffset := uint64(w.ehSize) + phCount*uint64(w.phEntSize)

	// Step 1: Lay out PT_LOAD segments first to establish file offsets and page congruence.
	for _, ph := range w.f.Headers {
		if ph.Type != PtLoad || len(ph.Sections) == 0 {
			continue
		}

		if ph.Align > 1 {
			targetMod := ph.VirtAddr % ph.Align
			currentMod := currentOffset % ph.Align

			if currentMod <= targetMod {
				currentOffset += targetMod - currentMod
			} else {
				currentOffset += (ph.Align - currentMod) + targetMod
			}
		}

		segStartOffset := currentOffset
		segStartVAddr := ph.VirtAddr

		for _, sec := range ph.Sections {
			if _, ok := w.sectionOffsets[sec]; ok {
				continue
			}

			if sec.AddressAlign > 1 {
				currentOffset = alignUp(currentOffset, sec.AddressAlign)
			}

			if sec.Address > segStartVAddr && segStartVAddr > 0 {
				expectedOff := segStartOffset + (sec.Address - segStartVAddr)
				if expectedOff > currentOffset {
					currentOffset = expectedOff
				}
			}

			w.sectionOffsets[sec] = currentOffset

			if sec.Type != ShtNoBits {
				currentOffset += uint64(len(sec.Data))
			}
		}
	}

	// Step 2: Lay out any remaining sections (non-PT_LOAD headers, unmapped sections, .shstrtab).
	for _, sec := range w.sections {
		if sec.Type == ShtNull {
			continue
		}
		if _, ok := w.sectionOffsets[sec]; ok {
			continue
		}

		if sec.AddressAlign > 1 {
			currentOffset = alignUp(currentOffset, sec.AddressAlign)
		}

		w.sectionOffsets[sec] = currentOffset

		if sec.Type != ShtNoBits {
			currentOffset += uint64(len(sec.Data))
		}
	}

	var shAlign uint64 = 4
	if w.f.Class == Bit64 {
		shAlign = 8
	}

	currentOffset = alignUp(currentOffset, shAlign)

	w.shOff = currentOffset

	// Step 3: Compute segment bounds now that all section offsets are fixed.
	for _, ph := range w.f.Headers {
		if ph.Type == PtPhdr {
			phOff := w.PhOff()
			phSize := phCount * uint64(w.phEntSize)
			w.segmentLayout[ph] = segmentMeta{
				offset: phOff,
				filesz: phSize,
				memsz:  phSize,
			}
			continue
		}

		if len(ph.Sections) == 0 {
			continue
		}

		// Find starting offset
		firstSec := ph.Sections[0]
		segStartOffset := w.sectionOffsets[firstSec]

		if ph.Type == PtLoad && ph.VirtAddr > 0 && firstSec.Address >= ph.VirtAddr {
			segStartOffset -= firstSec.Address - ph.VirtAddr
		} else {
			for _, sec := range ph.Sections {
				if off := w.sectionOffsets[sec]; off < segStartOffset {
					segStartOffset = off
				}
			}
		}

		var maxFileEnd, maxMemEnd uint64

		for _, sec := range ph.Sections {
			secOff := w.sectionOffsets[sec]

			if sec.Type != ShtNoBits {
				fileEnd := (secOff - segStartOffset) + uint64(len(sec.Data))
				if fileEnd > maxFileEnd {
					maxFileEnd = fileEnd
				}
			}

			var memEnd uint64
			if sec.Address >= ph.VirtAddr && ph.VirtAddr > 0 {
				memEnd = (sec.Address - ph.VirtAddr) + sec.ContentSize()
			} else {
				memEnd = (secOff - segStartOffset) + sec.ContentSize()
			}

			if memEnd > maxMemEnd {
				maxMemEnd = memEnd
			}
		}

		w.segmentLayout[ph] = segmentMeta{
			offset: segStartOffset,
			filesz: maxFileEnd,
			memsz:  maxMemEnd,
		}
	}
}

func (w *writer) WriteElfHeader() error {
	ident := [16]byte{
		0x7F, 'E', 'L', 'F',
		byte(w.f.Class),
		byte(w.f.Endianness),
		1,
		byte(w.f.OsAbi),
		0,
	}

	phNum := uint16(len(w.f.Headers))

	switch w.f.Class {
	case Bit32:
		hdr := elf.Header32{
			Ident:     ident,
			Type:      uint16(w.f.Type),
			Machine:   uint16(w.f.Machine),
			Version:   1,
			Entry:     uint32(w.f.Entry),
			Phoff:     uint32(w.PhOff()),
			Shoff:     uint32(w.shOff),
			Flags:     0,
			Ehsize:    w.ehSize,
			Phentsize: w.phEntSize,
			Phnum:     phNum,
			Shentsize: w.shEntSize,
			Shnum:     uint16(len(w.sections)),
			Shstrndx:  w.sectionNameTableSectionIndex,
		}

		return binary.Write(w, w.order, &hdr)

	case Bit64:
		hdr := elf.Header64{
			Ident:     ident,
			Type:      uint16(w.f.Type),
			Machine:   uint16(w.f.Machine),
			Version:   1,
			Entry:     w.f.Entry,
			Phoff:     w.PhOff(),
			Shoff:     w.shOff,
			Flags:     0,
			Ehsize:    w.ehSize,
			Phentsize: w.phEntSize,
			Phnum:     phNum,
			Shentsize: w.shEntSize,
			Shnum:     uint16(len(w.sections)),
			Shstrndx:  w.sectionNameTableSectionIndex,
		}

		return binary.Write(w, w.order, &hdr)

	default:
		panic("elf.writer.WriteElfHeader() - Invalid class")
	}
}

func (w *writer) WriteProgramHeaders() error {
	for _, ph := range w.f.Headers {
		meta := w.segmentLayout[ph]

		switch w.f.Class {
		case Bit32:
			prog := elf.Prog32{
				Type:   uint32(ph.Type),
				Off:    uint32(meta.offset),
				Vaddr:  uint32(ph.VirtAddr),
				Paddr:  uint32(ph.PhysAddr),
				Filesz: uint32(meta.filesz),
				Memsz:  uint32(meta.memsz),
				Flags:  uint32(ph.Flags),
				Align:  uint32(ph.Align),
			}

			if err := binary.Write(w, w.order, &prog); err != nil {
				return err
			}

		case Bit64:
			prog := elf.Prog64{
				Type:   uint32(ph.Type),
				Flags:  uint32(ph.Flags),
				Off:    meta.offset,
				Vaddr:  ph.VirtAddr,
				Paddr:  ph.PhysAddr,
				Filesz: meta.filesz,
				Memsz:  meta.memsz,
				Align:  ph.Align,
			}

			if err := binary.Write(w, w.order, &prog); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *writer) WritePayloads() error {
	type secOffset struct {
		sec    *SectionHeader
		offset uint64
	}

	var ordered []secOffset

	for _, sec := range w.sections {
		if sec.Type == ShtNull || sec.Type == ShtNoBits || len(sec.Data) == 0 {
			continue
		}

		ordered = append(ordered, secOffset{sec: sec, offset: w.sectionOffsets[sec]})
	}

	slices.SortFunc(ordered, func(a, b secOffset) int {
		return cmp.Compare(a.offset, b.offset)
	})

	for _, item := range ordered {
		if err := w.PadTo(item.offset); err != nil {
			return err
		}

		if _, err := w.Write(item.sec.Data); err != nil {
			return err
		}
	}

	return nil
}

func (w *writer) WriteSectionHeaders() error {
	for _, sec := range w.sections {
		var offset uint64
		if sec.Type != ShtNull {
			offset = w.sectionOffsets[sec]
		}

		var linkIdx uint32
		if sec.Link != nil {
			linkIdx = w.sectionIndices[sec.Link]
		}

		nameOff := w.nameOffsets[sec]

		switch w.f.Class {
		case Bit32:
			shdr := elf.Section32{
				Name:      nameOff,
				Type:      uint32(sec.Type),
				Flags:     uint32(sec.Flags),
				Addr:      uint32(sec.Address),
				Off:       uint32(offset),
				Size:      uint32(sec.ContentSize()),
				Link:      linkIdx,
				Info:      sec.Info,
				Addralign: uint32(sec.AddressAlign),
				Entsize:   uint32(sec.EntrySize),
			}

			if err := binary.Write(w, w.order, &shdr); err != nil {
				return err
			}

		case Bit64:
			shdr := elf.Section64{
				Name:      nameOff,
				Type:      uint32(sec.Type),
				Flags:     uint64(sec.Flags),
				Addr:      sec.Address,
				Off:       offset,
				Size:      sec.ContentSize(),
				Link:      linkIdx,
				Info:      sec.Info,
				Addralign: sec.AddressAlign,
				Entsize:   sec.EntrySize,
			}

			if err := binary.Write(w, w.order, &shdr); err != nil {
				return err
			}
		}
	}

	return nil
}

// Utils

func (w *writer) PhOff() uint64 {
	if len(w.f.Headers) > 0 {
		return uint64(w.ehSize)
	}

	return 0
}

func (w *writer) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += uint64(n)
	return n, err
}

var zeroChunk [4096]byte

func (w *writer) PadTo(target uint64) error {
	if target < w.n {
		return fmt.Errorf("cannot pad backwards: at %d, target %d", w.n, target)
	}

	for w.n < target {
		toWrite := min(target-w.n, uint64(len(zeroChunk)))

		if _, err := w.Write(zeroChunk[:toWrite]); err != nil {
			return err
		}
	}

	return nil
}

func alignUp(val, align uint64) uint64 {
	if align <= 1 {
		return val
	}

	rem := val % align
	if rem == 0 {
		return val
	}

	return val + (align - rem)
}
