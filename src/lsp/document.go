package lsp

import (
	"context"
	"fireball/core"
	"fireball/project"

	"github.com/fireball-lang/protocol"
)

type Document struct {
	Version        int32
	HasDiagnostics bool
}

// Diagnostics

func (s *Server) publishDiagnostics(ctx context.Context) {
	for _, workspace := range s.workspaces {
		workspace.mutex.RLock()

		for _, proj := range workspace.projMap {
			for _, file := range proj.Files {
				s.publishFileDiagnostics(ctx, file)
			}
		}

		workspace.mutex.RUnlock()
	}
}

//goland:noinspection GoPreferNilSlice
var emptyDiagnostics = []protocol.Diagnostic{}

func (s *Server) publishFileDiagnostics(ctx context.Context, file *project.File) {
	document := file.Data.(*Document)
	diagnostics := emptyDiagnostics

	for diagnostic := range file.Diagnostics() {
		var severity protocol.DiagnosticSeverity

		switch diagnostic.Kind {
		case core.Warning:
			severity = protocol.DiagnosticSeverityWarning
		case core.Error:
			severity = protocol.DiagnosticSeverityError

		default:
			panic("lsp.Handler.publishFileDiagnostics() - Invalid diagnostic kind")
		}

		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range:    toLspRange(diagnostic.Range),
			Severity: severity,
			Message:  diagnostic.Message,
		})
	}

	if len(diagnostics) > 0 || document.HasDiagnostics {
		_ = s.Client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
			URI:         protocol.DocumentURI("file://" + file.Path),
			Version:     uint32(document.Version),
			Diagnostics: diagnostics,
		})

		document.HasDiagnostics = len(diagnostics) > 0
	}
}
