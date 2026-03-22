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

type Diagnostic struct {
	Kind DiagnosticKind

	Path    string
	Range   Range
	Message string
}
