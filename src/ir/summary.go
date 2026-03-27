package ir

type SummaryRef uint32

func (s SummaryRef) Valid() bool {
	return s != 0
}

func (s SummaryRef) Value() uint32 {
	if s == 0 {
		panic("ir.SummaryRef.Value() - Summary reference is not valid")
	}

	return uint32(s) - 1
}

type Summary interface {
	next() Summary
	setNext(node Summary)
}

type baseSummary struct {
	nextNode Summary
}

func (b *baseSummary) next() Summary {
	return b.nextNode
}

func (b *baseSummary) setNext(node Summary) {
	b.nextNode = node
}

// Summaries

type LinkageKind uint8

const (
	LinkageExternal LinkageKind = iota
	LinkageLinkOnce
	LinkagePrivate
)

func (l LinkageKind) String() string {
	switch l {
	case LinkageExternal:
		return "external"
	case LinkageLinkOnce:
		return "linkonce"
	case LinkagePrivate:
		return "private"
	default:
		panic("ir.LinkageKind.String() - Invalid kind")
	}
}

type VisibilityKind uint8

const (
	VisibilityDefault VisibilityKind = iota
	VisibilityHidden
	VisibilityProtected
)

func (v VisibilityKind) String() string {
	switch v {
	case VisibilityDefault:
		return "default"
	case VisibilityHidden:
		return "hidden"
	case VisibilityProtected:
		return "protected"
	default:
		panic("ir.VisibilityKind.String() - Invalid kind")
	}
}

type ImportTypeKind uint8

const (
	ImportDefinition ImportTypeKind = iota
	ImportDeclaration
)

func (i ImportTypeKind) String() string {
	switch i {
	case ImportDefinition:
		return "definition"
	case ImportDeclaration:
		return "declaration"
	default:
		panic("ir.ImportTypeKind.String() - Invalid kind")
	}
}

type LinkSummaryFlags struct {
	Linkage             LinkageKind
	Visibility          VisibilityKind
	NotEligibleToImport bool
	Live                bool
	DsoLocal            bool
	CanAutoHide         bool
	ImportType          ImportTypeKind
}

type ModuleSummary struct {
	baseSummary

	Path string
	Hash [5]uint32
}

type SymbolSummary struct {
	baseSummary

	Name string
}

type FunctionSummaryFlags uint16

const (
	FuncReadNone FunctionSummaryFlags = 1 << iota
	FuncReadOnly
	FuncNoRecurse
	FuncReturnDoesNotAlias
	FuncNoInline
	FuncAlwaysInline
	FuncNoUnwind
	FuncMayThrow
	FuncHasUnknownCall
	FuncMustBeUnreachable
)

type FunctionSummaryCall struct {
	Callee SummaryRef
}

type FunctionSummary struct {
	baseSummary

	Module           SummaryRef
	Name             string
	LinkFlags        LinkSummaryFlags
	InstructionCount uint32
	Flags            FunctionSummaryFlags
	Calls            []FunctionSummaryCall
	Refs             []SummaryRef
}

type VariableSummaryFlags uint8

const (
	VarReadOnly VariableSummaryFlags = 1 << iota
	VarWriteOnly
	VarConstant
)

type VariableSummary struct {
	baseSummary

	Module    SummaryRef
	Name      string
	LinkFlags LinkSummaryFlags
	Flags     VariableSummaryFlags
	Refs      []SummaryRef
}

type SimpleSummary struct {
	baseSummary

	Name  string
	Value uint32
}
