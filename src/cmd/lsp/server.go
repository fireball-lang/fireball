package lsp

import (
	"context"
	"fireball/project"
	"fireball/utils"
	"github.com/MineGame159/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type server struct {
	logger *zap.Logger
	client protocol.Client

	projects []*project.Project

	analyzing sync.Mutex
}

func newServer() *server {
	return &server{}
}

func (s *server) Initialize(_ context.Context, params *protocol.InitializeParams) (result *protocol.InitializeResult, err error) {
	defer stop(start(s, "Initialize"))

	// Open projects
	for _, folder := range params.WorkspaceFolders {
		// Parse URI
		uri_, err := uri.Parse(folder.URI)
		if err != nil {
			continue
		}

		// Open project
		proj, err := project.OpenProject(uri_.Filename())
		if err != nil {
			continue
		}

		// Add project
		s.projects = append(s.projects, proj)
		s.logger.Info("Opened project", zap.String("path", proj.AbsolutePath))
	}

	// Return server info
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				Change: protocol.TextDocumentSyncKindFull,
			},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "fireball",
			Version: "0.1.0",
		},
	}, nil
}

func (s *server) Initialized(_ context.Context, _ *protocol.InitializedParams) (err error) {
	defer stop(start(s, "Initialized"))

	// Load initial project files
	for _, proj := range s.projects {
		entries, err := os.ReadDir(filepath.Join(proj.AbsolutePath, "src"))
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".fb") {
				path := filepath.Join(proj.AbsolutePath, "src", entry.Name())

				doc, err := newDocument(path)
				if err != nil {
					continue
				}

				proj.AddFile(doc)
			}
		}
	}

	// Analyze
	go s.analyze()

	return nil
}

func (s *server) Shutdown(_ context.Context) (err error) {
	defer stop(start(s, "Shutdown"))

	return nil
}

func (s *server) Exit(_ context.Context) (err error) {
	defer stop(start(s, "Exit"))

	return nil
}

func (s *server) DidChange(_ context.Context, params *protocol.DidChangeTextDocumentParams) (err error) {
	defer stop(start(s, "DidChange"))

	// Get file
	file := s.getFile(params.TextDocument.URI)
	if file == nil {
		return nil
	}

	// Update document
	doc := file.Provider().(*document)

	doc.contents = params.ContentChanges[0].Text
	doc.changed = true

	doc.version = params.TextDocument.Version

	// Analyze
	go s.analyze()

	return nil
}

// Analyze

func (s *server) analyze() {
	defer stop(start(s, "analyze"))

	s.analyzing.Lock()
	defer s.analyzing.Unlock()

	for _, proj := range s.projects {
		// Analyze
		proj.Analyze()

		// Publish diagnostics
		for file := range proj.Files() {
			doc := file.Provider().(*document)

			diagnostics := file.Diagnostics()
			lspDiagnostics := make([]protocol.Diagnostic, len(diagnostics))

			for i, diagnostic := range diagnostics {
				severity := protocol.DiagnosticSeverityError
				if diagnostic.Kind == utils.Warning {
					severity = protocol.DiagnosticSeverityWarning
				}

				lspDiagnostics[i] = protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      diagnostic.Range.Start.Line - 1,
							Character: diagnostic.Range.Start.Column,
						},
						End: protocol.Position{
							Line:      diagnostic.Range.End.Line - 1,
							Character: diagnostic.Range.End.Column,
						},
					},
					Severity: severity,
					Source:   "fireball",
					Message:  diagnostic.Message,
				}
			}

			_ = s.client.PublishDiagnostics(context.Background(), &protocol.PublishDiagnosticsParams{
				URI:         doc.uri,
				Version:     uint32(doc.version),
				Diagnostics: lspDiagnostics,
			})
		}
	}
}

// Utils

func (s *server) getFile(uri uri.URI) *project.File {
	uriPath := uri.Filename()

	for _, proj := range s.projects {
		// Find project
		_, err := filepath.Rel(proj.AbsolutePath, uriPath)
		if err != nil {
			continue
		}

		// Find file
		for file := range proj.Files() {
			if file.AbsolutePath() == uriPath {
				return file
			}
		}
	}

	return nil
}

func (s *server) error(ctx context.Context, msg string) {
	_ = s.client.ShowMessage(ctx, &protocol.ShowMessageParams{
		Message: msg,
		Type:    protocol.MessageTypeError,
	})
}

// Start / Stop

type request struct {
	s     *server
	name  string
	start time.Time
}

func start(s *server, name string) request {
	return request{
		s:     s,
		name:  name,
		start: time.Now(),
	}
}

func stop(req request) {
	duration := time.Now().Sub(req.start)
	req.s.logger.Debug(req.name, zap.Duration("duration", duration))
}
