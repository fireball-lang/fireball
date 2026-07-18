package lsp

import (
	"context"
	"fireball/core"
	"fireball/project"
	"reflect"
	"sync"

	"github.com/fireball-lang/protocol"
	"go.lsp.dev/uri"
)

type Document struct {
	mu sync.Mutex

	Version int32

	lastDiagnostics []protocol.Diagnostic
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

	document.mu.Lock()
	defer document.mu.Unlock()

	if document.lastDiagnostics == nil || !reflect.DeepEqual(document.lastDiagnostics, diagnostics) {
		_ = s.Client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
			URI:         uri.File(file.Path),
			Version:     uint32(document.Version),
			Diagnostics: diagnostics,
		})

		document.lastDiagnostics = diagnostics
	}
}

func (s *Server) clearFileDiagnostics(ctx context.Context, file *project.File) {
	document := file.Data.(*Document)

	document.mu.Lock()
	defer document.mu.Unlock()

	_ = s.Client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         uri.File(file.Path),
		Version:     uint32(document.Version),
		Diagnostics: emptyDiagnostics,
	})

	document.lastDiagnostics = emptyDiagnostics
}
