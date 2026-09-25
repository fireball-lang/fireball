package elf

type Class uint8

const (
	Bit32 Class = 1
	Bit64 Class = 2
)

type Endianness uint8

const (
	Little Endianness = 1
	Big    Endianness = 2
)

type OsAbi uint8

const (
	SystemV OsAbi = 0x00
	Linux   OsAbi = 0x03
)

type Type uint16

const (
	Relocatable Type = 0x0001
	Executable  Type = 0x0002
	Dynamic     Type = 0x0003
	Core        Type = 0x0004
)

type Machine uint16

const (
	x86   Machine = 0x0003
	Arm   Machine = 0x0028
	Amd64 Machine = 0x003E
	Arm64 Machine = 0x00B7
)

type ProgramHeaderType uint32

const (
	PtNull      ProgramHeaderType = 0x00000000
	PtLoad      ProgramHeaderType = 0x00000001
	PtDynamic   ProgramHeaderType = 0x00000002
	PtInterpret ProgramHeaderType = 0x00000003
	PtNote      ProgramHeaderType = 0x00000004
	PtPhdr      ProgramHeaderType = 0x00000006
	PtTls       ProgramHeaderType = 0x00000007
)

type ProgramHeaderFlags uint32

const (
	PfExecute ProgramHeaderFlags = 0x00000001
	PfWrite   ProgramHeaderFlags = 0x00000002
	PfRead    ProgramHeaderFlags = 0x00000004
)

type SectionHeaderType uint32

const (
	ShtNull                     SectionHeaderType = 0x00000000
	ShtProgram                  SectionHeaderType = 0x00000001
	ShtSymbolTable              SectionHeaderType = 0x00000002
	ShtStringTable              SectionHeaderType = 0x00000003
	ShtRelocationsAddends       SectionHeaderType = 0x00000004
	ShtHashTable                SectionHeaderType = 0x00000005
	ShtDynamic                  SectionHeaderType = 0x00000006
	ShtNote                     SectionHeaderType = 0x00000007
	ShtNoBits                   SectionHeaderType = 0x00000008
	ShtRelocationsNoAddends     SectionHeaderType = 0x00000009
	ShtDynamicLinkerSymbolTable SectionHeaderType = 0x0000000B
	ShtInitArray                SectionHeaderType = 0x0000000E
	ShtFiniArray                SectionHeaderType = 0x0000000F
	ShtPreInitArray             SectionHeaderType = 0x00000010
	ShtGroup                    SectionHeaderType = 0x00000011
	ShtExtSectionIndices        SectionHeaderType = 0x00000012
)

type SectionHeaderFlags uint32

const (
	ShfWrite           SectionHeaderFlags = 0x1
	ShfAllocate        SectionHeaderFlags = 0x2
	ShfExecute         SectionHeaderFlags = 0x4
	ShfMerge           SectionHeaderFlags = 0x10
	ShfStrings         SectionHeaderFlags = 0x20
	ShfInfoLink        SectionHeaderFlags = 0x40
	ShfLinkOrder       SectionHeaderFlags = 0x80
	ShfOsNonConforming SectionHeaderFlags = 0x100
	ShfGroup           SectionHeaderFlags = 0x200
	ShfTls             SectionHeaderFlags = 0x400
	ShfMaskOs          SectionHeaderFlags = 0x0FF00000
	ShfMaskProc        SectionHeaderFlags = 0xF0000000
	ShfOrdered         SectionHeaderFlags = 0x4000000
)

type SectionHeader struct {
	Name string

	Type  SectionHeaderType
	Flags SectionHeaderFlags

	Address      uint64
	AddressAlign uint64

	Link      *SectionHeader
	Info      uint32
	EntrySize uint64

	Size uint64
	Data []uint8
}

func (s *SectionHeader) ContentSize() uint64 {
	if s.Type == ShtNoBits {
		return s.Size
	}

	if len(s.Data) == 0 && s.Size > 0 {
		return s.Size
	}

	return uint64(len(s.Data))
}

type ProgramHeader struct {
	Type  ProgramHeaderType
	Flags ProgramHeaderFlags

	VirtAddr uint64
	PhysAddr uint64
	Align    uint64

	Sections []*SectionHeader
}

func (p *ProgramHeader) FileSize() uint64 {
	var size uint64

	for _, s := range p.Sections {
		if s.Type != ShtNoBits {
			size += uint64(len(s.Data))
		}
	}

	return size
}

func (p *ProgramHeader) MemSize() uint64 {
	var size uint64

	for _, s := range p.Sections {
		size += s.ContentSize()
	}

	return size
}

type File struct {
	Class      Class
	Endianness Endianness
	OsAbi      OsAbi
	Type       Type
	Machine    Machine

	Entry uint64

	Headers  []*ProgramHeader
	Sections []*SectionHeader
}
