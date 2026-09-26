package obj

type SectionKind uint8

const (
	SecText SectionKind = iota
	SecData
	SecReadOnly
	SecBSS
)

type SymbolScope uint8

const (
	ScopeLocal SymbolScope = iota
	ScopeGlobal
	ScopeWeak
)

type SymbolKind uint8

const (
	SymNone SymbolKind = iota
	SymFunction
	SymData
	SymSection
)

type RelocationKind uint8

const (
	RelocAbs64 RelocationKind = iota
	RelocAbs32
	RelocSigned32
	RelocPC32
)

type Arch uint8

const (
	AMD64 Arch = iota
)

type Symbol struct {
	Name string

	Kind  SymbolKind
	Scope SymbolScope

	Section *Section

	Value uint64
	Size  uint64
}

type Relocation struct {
	Target *Symbol

	Kind   RelocationKind
	Offset int

	Addend        int64
	TrailingBytes int
}

type Section struct {
	Name string
	Kind SectionKind

	Align uint64

	Data []byte
	Size uint64

	Relocations []Relocation
}

type File struct {
	Arch Arch

	Sections []*Section
	Symbols  []*Symbol
}

func (f *File) AddSection(section *Section) *Section {
	f.Sections = append(f.Sections, section)
	return section
}

func (f *File) AddSymbol(symbol *Symbol) *Symbol {
	f.Symbols = append(f.Symbols, symbol)
	return symbol
}
