package lsp

import (
	"bytes"
	"context"
	"fireball/project"
	"log/slog"
	"slices"
	"time"

	"github.com/fireball-lang/protocol"
)

type Server struct {
	Logger *slog.Logger
	Client protocol.Client

	workspaces []*Workspace

	changed chan *project.File
	stop    chan any

	definitionLinkSupport bool
}

// Lifecycle

func (s *Server) parseWorker() {
	var workspaces []*Workspace
	var files []*project.File

	timer := time.NewTimer(time.Hour)
	timer.Stop()

	for {
		select {
		case file := <-s.changed:
			// Queue file to be parsed
			workspace := s.getWorkspace(file)

			if !slices.Contains(workspaces, workspace) {
				workspace.mutex.Lock()
				workspaces = append(workspaces, workspace)
			}

			if !slices.Contains(files, file) {
				files = append(files, file)
			}

			timer.Reset(time.Millisecond * 250)

		case <-timer.C:
			// Parse queued files and analyze the workspace they are in
			for _, workspace := range workspaces {
				workspace.parseFiles(files)
				workspace.mutex.Unlock()
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			s.publishDiagnostics(ctx)
			cancel()

			workspaces = workspaces[0:0]
			files = files[0:0]

		case <-s.stop:
			// Stop
			return
		}
	}
}

func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	s.info(ctx, "Starting")

	for _, folder := range params.WorkspaceFolders {
		s.openWorkspace(ctx, uriPath(protocol.DocumentURI(folder.URI)))
	}

	s.publishDiagnostics(ctx)

	s.changed = make(chan *project.File, 8)
	s.stop = make(chan any, 1)

	s.definitionLinkSupport = params.Capabilities.TextDocument.Definition.LinkSupport

	go s.parseWorker()

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindIncremental,
			},
			SemanticTokensProvider: &protocol.SemanticTokensOptions{
				Legend: protocol.SemanticTokensLegend{
					TokenTypes: []protocol.SemanticTokenTypes{
						protocol.SemanticTokenFunction,
						protocol.SemanticTokenParameter,
						protocol.SemanticTokenVariable,
						protocol.SemanticTokenType,
						protocol.SemanticTokenClass,
						protocol.SemanticTokenEnum,
						protocol.SemanticTokenProperty,
						protocol.SemanticTokenEnumMember,
						protocol.SemanticTokenNamespace,
						protocol.SemanticTokenInterface,
						protocol.SemanticTokenTypeParameter,
						protocol.SemanticTokenKeyword,
					},
					TokenModifiers: []protocol.SemanticTokenModifiers{},
				},
				Full: &protocol.SemanticTokensFull{},
			},
			DefinitionProvider: &protocol.DefinitionOptions{},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "fireball",
			Version: "0.1.0",
		},
	}, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.info(ctx, "Stopping")

	s.stop <- nil

	return nil
}

// Text document sync

func (s *Server) DidOpen(_ context.Context, params *protocol.DidOpenTextDocumentParams) error {
	// Get file
	file, _ := s.getFile(uriPath(params.TextDocument.URI))
	if file == nil {
		return nil
	}

	// Set text contents
	if s, ok := file.Source.(*Source); ok {
		s.Apply(protocol.TextDocumentContentChangeEvent{Text: params.TextDocument.Text})
	} else {
		file.Source = &Source{
			lines: bytes.SplitAfter([]byte(params.TextDocument.Text), []byte{'\n'}),
		}
	}

	// Set document version
	file.Data.(*Document).Version = params.TextDocument.Version

	return nil
}

func (s *Server) DidChange(_ context.Context, params *protocol.DidChangeTextDocumentParams) error {
	// Get file
	file, _ := s.getFile(uriPath(params.TextDocument.URI))
	if file == nil {
		return nil
	}

	// Create file source
	if _, ok := file.Source.(*Source); !ok {
		file.Source = NewSource(file.Source)
	}

	// Apply changes
	source := file.Source.(*Source)

	for _, change := range params.ContentChanges {
		source.Apply(change)
	}

	// Set document version
	file.Data.(*Document).Version = params.TextDocument.Version

	// Mark file as changed
	s.changed <- file

	return nil
}

func (s *Server) DidClose(_ context.Context, _ *protocol.DidCloseTextDocumentParams) error {
	return nil
}
