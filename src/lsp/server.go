package lsp

import (
	"bytes"
	"context"
	"fireball/project"
	"iter"
	"log/slog"
	"path"
	"slices"
	"time"

	"github.com/fireball-lang/protocol"
)

type Server struct {
	Logger *slog.Logger
	Client protocol.Client

	nativeWatcher *NativeWatcher

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

	var filters []protocol.FileOperationFilter

	if params.Capabilities.Workspace.FileOperations.DidCreate && params.Capabilities.Workspace.FileOperations.DidDelete && params.Capabilities.Workspace.FileOperations.DidRename {
		filters = []protocol.FileOperationFilter{
			{
				Scheme: "file",
				Pattern: protocol.FileOperationPattern{
					Glob:    "**/*.fb",
					Matches: protocol.FileOperationPatternKindFile,
				},
			},
			{
				Scheme: "file",
				Pattern: protocol.FileOperationPattern{
					Glob:    "**/project.toml",
					Matches: protocol.FileOperationPatternKindFile,
				},
			},
		}

		s.info(ctx, "Using LSP file operation notifications")
	} else {
		filters = []protocol.FileOperationFilter{}

		s.nativeWatcher = NewNativeWatcher(s.Logger)

		s.nativeWatcher.Create = s.DidCreateFiles
		s.nativeWatcher.Delete = s.DidDeleteFiles

		s.info(ctx, "Using native OS file watchers")
	}

	for _, folder := range params.WorkspaceFolders {
		s.openWorkspace(ctx, uriPath(protocol.DocumentURI(folder.URI)))
	}

	s.info(ctx, "%#v", params.Capabilities.Workspace.FileOperations)

	s.publishDiagnostics(ctx)

	s.changed = make(chan *project.File, 8)
	s.stop = make(chan any, 1)

	s.definitionLinkSupport = params.Capabilities.TextDocument.Definition.LinkSupport

	go s.parseWorker()

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			Workspace: &protocol.ServerCapabilitiesWorkspace{
				WorkspaceFolders: &protocol.ServerCapabilitiesWorkspaceFolders{
					Supported:           true,
					ChangeNotifications: true,
				},
				FileOperations: &protocol.ServerCapabilitiesWorkspaceFileOperations{
					DidCreate: &protocol.FileOperationRegistrationOptions{Filters: filters},
					DidRename: &protocol.FileOperationRegistrationOptions{Filters: filters},
					DidDelete: &protocol.FileOperationRegistrationOptions{Filters: filters},
				},
			},
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
			DocumentSymbolProvider: &protocol.DocumentSymbolOptions{
				Label: "Fireball",
			},
			WorkspaceSymbolProvider: &protocol.WorkspaceSymbolOptions{},
			DefinitionProvider:      &protocol.DefinitionOptions{},
			SignatureHelpProvider: &protocol.SignatureHelpOptions{
				TriggerCharacters:   []string{"("},
				RetriggerCharacters: []string{","},
			},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "fireball",
			Version: "0.1.0",
		},
	}, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.info(ctx, "Stopping")

	if s.nativeWatcher != nil {
		s.nativeWatcher.Close()
	}

	s.stop <- nil

	return nil
}

// Workspace

func (s *Server) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) error {
	for _, folder := range params.Event.Removed {
		folderPath := uriPath(protocol.DocumentURI(folder.URI))

		index := slices.IndexFunc(s.workspaces, func(workspace *Workspace) bool {
			return workspace.path == folderPath
		})

		if index == -1 {
			s.warn(ctx, "Workspace not found: '%s'", folder.URI)
			continue
		}

		s.workspaces[index].removeWatchers(s)

		s.workspaces = slices.Delete(s.workspaces, index, index+1)
	}

	for _, folder := range params.Event.Added {
		s.openWorkspace(ctx, uriPath(protocol.DocumentURI(folder.URI)))
	}

	return nil
}

func (s *Server) DidCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) error {
	s.reloadWorkspacesIfProjectConfigChanged(ctx, func(yield func(string) bool) {
		for _, file := range params.Files {
			if !yield(file.URI) {
				return
			}
		}
	})

	for _, fileCreate := range params.Files {
		if path.Ext(fileCreate.URI) != ".fb" {
			continue
		}

		fullPath := uriPath(protocol.DocumentURI(fileCreate.URI))
		proj := s.getProject(fullPath)

		if proj == nil {
			s.warn(ctx, "Failed to find project for file: '%s'", fileCreate.URI)
			continue
		}

		file := proj.AddFile(fullPath)

		if file == nil {
			s.warn(ctx, "Failed to add file to project: '%s'", fileCreate.URI)
			continue
		}

		file.Data = &Document{}

		s.changed <- file
	}

	return nil
}

func (s *Server) DidRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) error {
	var deleted []protocol.FileDelete
	var created []protocol.FileCreate

	for _, file := range params.Files {
		deleted = append(deleted, protocol.FileDelete{URI: file.OldURI})
		created = append(created, protocol.FileCreate{URI: file.NewURI})
	}

	_ = s.DidDeleteFiles(ctx, &protocol.DeleteFilesParams{Files: deleted})
	_ = s.DidCreateFiles(ctx, &protocol.CreateFilesParams{Files: created})

	return nil
}

func (s *Server) DidDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) error {
	s.reloadWorkspacesIfProjectConfigChanged(ctx, func(yield func(string) bool) {
		for _, file := range params.Files {
			if !yield(file.URI) {
				return
			}
		}
	})

	for _, fileDelete := range params.Files {
		if path.Ext(fileDelete.URI) != ".fb" {
			continue
		}

		fullPath := uriPath(protocol.DocumentURI(fileDelete.URI))
		proj := s.getProject(fullPath)

		if proj == nil {
			s.warn(ctx, "Failed to find project for file: '%s'", fileDelete.URI)
			continue
		}

		if !proj.RemoveFile(fullPath) {
			s.warn(ctx, "Failed to remove file from project: '%s'", fileDelete.URI)
			continue
		}

		// workaround to re-check the project
		if len(proj.Files) > 0 {
			s.changed <- proj.Files[0]
		}
	}

	return nil
}

func (s *Server) reloadWorkspacesIfProjectConfigChanged(ctx context.Context, it iter.Seq[string]) {
	var workspaces []*Workspace

	for uri := range it {
		if path.Base(uri) == "project.toml" {
			workspace := s.getWorkspaceForProjectConfig(uriPath(protocol.DocumentURI(uri)))

			if workspace == nil {
				s.warn(ctx, "failed to find workspace for project config file: '%s'", uri)
				continue
			}

			if !slices.Contains(workspaces, workspace) {
				workspaces = append(workspaces, workspace)
			}
		}
	}

	for _, workspace := range workspaces {
		workspace.reload(s, ctx)
	}
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
