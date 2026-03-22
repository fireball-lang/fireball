package core

type DiagnosticKind uint8

const (
	Warning DiagnosticKind = iota
	Error
)

type Diagnostic struct {
	Kind DiagnosticKind

	Path    string
	Range   Range
	Message string
}
