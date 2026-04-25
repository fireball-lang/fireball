package lsp

import (
	"bytes"
	"context"
	"fireball/project"
	"slices"
	"time"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
)

type Handler struct {
	Client *server.Client

	workspaces []*Workspace

	changed chan *project.File
	stop    chan any
}

// Client

func (h *Handler) SetClient(client *server.Client) {
	h.Client = client
}

// Lifecycle

func (h *Handler) parseWorker() {
	var workspaces []*Workspace
	var files []*project.File

	timer := time.NewTimer(time.Hour)
	timer.Stop()

	for {
		select {
		case file := <-h.changed:
			// Queue file to be parsed
			workspace := h.getWorkspace(file)

			if !slices.Contains(workspaces, workspace) {
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
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			h.publishDiagnostics(ctx)
			cancel()

			workspaces = workspaces[0:0]
			files = files[0:0]

		case <-h.stop:
			// Stop
			return
		}
	}
}

func (h *Handler) Initialize(ctx context.Context, params *lsp.InitializeParams) (*lsp.InitializeResult, error) {
	h.info(ctx, "Starting")

	for _, folder := range params.WorkspaceFolders {
		h.openWorkspace(ctx, uriPath(folder.URI))
	}

	h.publishDiagnostics(ctx)

	h.changed = make(chan *project.File, 8)
	h.stop = make(chan any, 1)

	go h.parseWorker()

	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptions{
				OpenClose: new(true),
				Change:    lsp.SyncIncremental,
			},
		},
		ServerInfo: &lsp.ServerInfo{
			Name:    "fireball",
			Version: "0.1.0",
		},
	}, nil
}

func (h *Handler) Shutdown(ctx context.Context) error {
	h.info(ctx, "Stopping")

	h.stop <- nil

	return nil
}

// Text document sync

func (h *Handler) DidOpen(_ context.Context, params *lsp.DidOpenTextDocumentParams) error {
	// Get file
	file := h.getFile(uriPath(params.TextDocument.URI))
	if file == nil {
		return nil
	}

	// Set text contents
	if s, ok := file.Source.(*Source); ok {
		s.Apply(lsp.TextDocumentContentChangeEvent{Text: params.TextDocument.Text})
	} else {
		file.Source = &Source{
			lines: bytes.SplitAfter([]byte(params.TextDocument.Text), []byte{'\n'}),
		}
	}

	// Set document version
	file.Data.(*Document).Version = params.TextDocument.Version

	return nil
}

func (h *Handler) DidChange(_ context.Context, params *lsp.DidChangeTextDocumentParams) error {
	// Get file
	file := h.getFile(uriPath(params.TextDocument.URI))
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
	h.changed <- file

	return nil
}

func (h *Handler) DidClose(_ context.Context, _ *lsp.DidCloseTextDocumentParams) error {
	return nil
}
