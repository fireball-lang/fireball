package core

type DiagnosticKind uint8

const (
	Warning DiagnosticKind = iota
	Error
)

func (d DiagnosticKind) String() string {
	switch d {
	case Warning:
		return "Warning"
	case Error:
		return "Error"
	default:
		panic("core.DiagnosticKind.String() - Invalid kind")
	}
}

type DiagnosticFlags uint8

const (
	Deprecated DiagnosticFlags = 1 << iota
)

type Diagnostic struct {
	Kind  DiagnosticKind
	Flags DiagnosticFlags

	Path    string
	Range   Range
	Message string
}
