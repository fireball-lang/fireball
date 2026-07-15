package lsp

import (
	"bytes"
	"context"
	"fireball/core"
	"fireball/project"
	"iter"
	"log/slog"
	"path"
	"slices"

	"github.com/fireball-lang/protocol"
	"go.lsp.dev/uri"
)

type Server struct {
	Logger *slog.Logger
	Client protocol.Client

	nativeWatcher *NativeWatcher

	workspaces []*Workspace

	definitionLinkSupport bool
}

// Lifecycle

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

	s.definitionLinkSupport = params.Capabilities.TextDocument.Definition.LinkSupport

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

		s.parseAndPublish(ctx, s.getWorkspace(file), []*project.File{file})
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

			// A deleted directory may contain a project.toml
			fullPath := uriPath(protocol.DocumentURI(file.URI))

			if path.Ext(fullPath) != ".fb" && path.Base(fullPath) != "project.toml" {
				for _, workspace := range s.workspaces {
					for _, proj := range workspace.projMap {
						if core.IsFilepathInside(fullPath, path.Join(proj.Path, "project.toml")) {
							if !yield(string(uri.File(path.Join(proj.Path, "project.toml")))) {
								return
							}
						}
					}
				}
			}
		}
	})

	for _, fileDelete := range params.Files {
		fullPath := uriPath(protocol.DocumentURI(fileDelete.URI))

		if path.Ext(fullPath) != ".fb" && path.Base(fullPath) != "project.toml" {
			s.deleteFilesUnder(ctx, fullPath)
			continue
		}

		proj := s.getProject(fullPath)

		if proj == nil {
			s.warn(ctx, "Failed to find project for file: '%s'", fileDelete.URI)
			continue
		}

		if !proj.RemoveFile(fullPath) {
			s.warn(ctx, "Failed to remove file from project: '%s'", fileDelete.URI)
			continue
		}

		workspace := s.getWorkspaceForProject(proj)
		s.parseAndPublish(ctx, workspace, nil)
	}

	return nil
}

func (s *Server) deleteFilesUnder(ctx context.Context, dir string) {
	for _, workspace := range s.workspaces {
		for _, proj := range workspace.projMap {
			var removed []*project.File

			for _, file := range proj.Files {
				if core.IsFilepathInside(dir, file.Path) {
					removed = append(removed, file)
				}
			}

			for _, file := range removed {
				proj.RemoveFile(file.Path)
			}

			if len(removed) > 0 {
				s.parseAndPublish(ctx, workspace, nil)
			}
		}
	}
}

func (s *Server) reloadWorkspacesIfProjectConfigChanged(ctx context.Context, it iter.Seq[string]) {
	var workspaces []*Workspace

	for uri_ := range it {
		if path.Base(uri_) == "project.toml" {
			workspace := s.getWorkspaceForProjectConfig(uriPath(protocol.DocumentURI(uri_)))

			if workspace == nil {
				s.warn(ctx, "failed to find workspace for project config file: '%s'", uri_)
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

func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	// Get file and its workspace
	file, _ := s.getFile(uriPath(params.TextDocument.URI))
	if file == nil {
		return nil
	}

	workspace := s.getWorkspace(file)

	// Acquire write lock
	workspace.mutex.Lock()

	// Set text contents
	if src, ok := file.Source.(*Source); ok {
		src.Apply(protocol.TextDocumentContentChangeEvent{Text: params.TextDocument.Text})
	} else {
		file.Source = &Source{
			lines: bytes.SplitAfter([]byte(params.TextDocument.Text), []byte{'\n'}),
		}
	}

	// Set document version
	file.Data.(*Document).Version = params.TextDocument.Version

	// Parse and release write lock
	workspace.parseFiles([]*project.File{file})
	workspace.mutex.Unlock()

	s.publishDiagnostics(ctx)

	return nil
}

func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	// Get file and its workspace
	file, _ := s.getFile(uriPath(params.TextDocument.URI))
	if file == nil {
		return nil
	}

	workspace := s.getWorkspace(file)

	// Acquire write lock
	workspace.mutex.Lock()

	// Create file source if needed
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

	// Parse and release write lock
	workspace.parseFiles([]*project.File{file})
	workspace.mutex.Unlock()

	s.publishDiagnostics(ctx)

	return nil
}

func (s *Server) DidClose(_ context.Context, _ *protocol.DidCloseTextDocumentParams) error {
	return nil
}

func (s *Server) parseAndPublish(ctx context.Context, workspace *Workspace, files []*project.File) {
	workspace.mutex.Lock()
	workspace.parseFiles(files)
	workspace.mutex.Unlock()

	s.publishDiagnostics(ctx)
}
