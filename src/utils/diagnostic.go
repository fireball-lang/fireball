package utils

import (
	"fireball/lexer"
	"fmt"
)

type DiagnosticKind uint8

const (
	Warning DiagnosticKind = iota
	Error
)

func (d DiagnosticKind) String() string {
	if d == Warning {
		return "Warning"
	}

	return "Error"
}

type Diagnostic struct {
	Kind    DiagnosticKind
	Message string
	Range   lexer.Range
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("[%s] %s - %s", d.Kind, d.Range.Start, d.Message)
}
