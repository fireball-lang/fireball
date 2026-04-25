package lsp

import (
	"context"
	"fireball/core"
	"fireball/project"

	"github.com/owenrumney/go-lsp/lsp"
)

type Document struct {
	Version        int
	HasDiagnostics bool
}

// Diagnostics

func (h *Handler) publishDiagnostics(ctx context.Context) {
	for _, workspace := range h.workspaces {
		workspace.mutex.RLock()

		for _, proj := range workspace.projMap {
			for _, file := range proj.Files {
				h.publishFileDiagnostics(ctx, file)
			}
		}

		workspace.mutex.RUnlock()
	}
}

//goland:noinspection GoPreferNilSlice
var emptyDiagnostics = []lsp.Diagnostic{}

func (h *Handler) publishFileDiagnostics(ctx context.Context, file *project.File) {
	document := file.Data.(*Document)
	diagnostics := emptyDiagnostics

	for diagnostic := range file.Diagnostics() {
		var severity lsp.DiagnosticSeverity

		switch diagnostic.Kind {
		case core.Warning:
			severity = lsp.SeverityWarning
		case core.Error:
			severity = lsp.SeverityError

		default:
			panic("lsp.Handler.publishFileDiagnostics() - Invalid diagnostic kind")
		}

		diagnostics = append(diagnostics, lsp.Diagnostic{
			Range:    toLspRange(diagnostic.Range),
			Severity: &severity,
			Message:  diagnostic.Message,
		})
	}

	if len(diagnostics) > 0 || document.HasDiagnostics {
		_ = h.Client.PublishDiagnostics(ctx, &lsp.PublishDiagnosticsParams{
			URI:         lsp.DocumentURI("file://" + file.Path),
			Version:     &document.Version,
			Diagnostics: diagnostics,
		})

		document.HasDiagnostics = len(diagnostics) > 0
	}
}
