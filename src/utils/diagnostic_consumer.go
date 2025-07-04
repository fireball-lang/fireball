package utils

type DiagnosticConsumer interface {
	Add(diag Diagnostic)
}
