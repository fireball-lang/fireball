package lsp

import (
	"bytes"
	"context"
	"fireball/ast"
	"fireball/cfg"
	"fireball/core"
	"fireball/project"
	"iter"
	"log/slog"
	"path"
	"slices"
	"sync"

	"github.com/fireball-lang/protocol"
	"go.lsp.dev/uri"
)

type Server struct {
	Logger *slog.Logger
	Client protocol.Client

	Env cfg.Env

	nativeWatcher *NativeWatcher

	workspacesMutex sync.Mutex
	workspaces      []*Workspace

	fullSemanticTokens    bool
	definitionLinkSupport bool
}

func (s *Server) getWorkspaces() []*Workspace {
	s.workspacesMutex.Lock()
	defer s.workspacesMutex.Unlock()

	return slices.Clone(s.workspaces)
}

// Lifecycle

func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	s.info(ctx, "Starting")

	if opts, ok := params.InitializationOptions.(map[string]any); ok {
		if targetOs, ok := opts["target_os"].(string); ok {
			switch targetOs {
			case "windows":
				s.Env.TargetOs = ast.WindowsOs
			case "linux":
				s.Env.TargetOs = ast.Linux
			case "macos":
				s.Env.TargetOs = ast.MacOS
			}

			s.Env.ComputeDerived()
		}

		if fullSemanticTokens, ok := opts["full_semantic_tokens"].(bool); ok {
			s.fullSemanticTokens = fullSemanticTokens
		}
	}

	var filters []protocol.FileOperationFilter

	if params.Capabilities.Workspace != nil && params.Capabilities.Workspace.FileOperations != nil && params.Capabilities.Workspace.FileOperations.DidCreate && params.Capabilities.Workspace.FileOperations.DidDelete && params.Capabilities.Workspace.FileOperations.DidRename {
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
		s.openWorkspace(ctx, uri.URI(folder.URI).Filename())
	}

	if params.Capabilities.Workspace != nil && params.Capabilities.Workspace.FileOperations != nil {
		s.info(ctx, "%#v", params.Capabilities.Workspace.FileOperations)
	}

	s.publishDiagnostics(ctx)

	if params.Capabilities.TextDocument != nil && params.Capabilities.TextDocument.Definition != nil {
		s.definitionLinkSupport = params.Capabilities.TextDocument.Definition.LinkSupport
	}

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
						protocol.SemanticTokenComment,
					},
					TokenModifiers: []protocol.SemanticTokenModifiers{
						protocol.SemanticTokenModifierReadonly,
					},
				},
				Full: &protocol.SemanticTokensFull{},
			},
			DocumentSymbolProvider: &protocol.DocumentSymbolOptions{
				Label: "Fireball",
			},
			WorkspaceSymbolProvider: &protocol.WorkspaceSymbolOptions{},
			DefinitionProvider:      &protocol.DefinitionOptions{},
			ReferencesProvider:      &protocol.ReferencesOptions{},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{".", ":"},
			},
			RenameProvider: &protocol.RenameOptions{
				PrepareProvider: true,
			},
			SignatureHelpProvider: &protocol.SignatureHelpOptions{
				TriggerCharacters:   []string{"("},
				RetriggerCharacters: []string{","},
			},
			HoverProvider: &protocol.HoverOptions{},
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
		folderPath := uri.URI(folder.URI).Filename()

		s.workspacesMutex.Lock()

		index := slices.IndexFunc(s.workspaces, func(workspace *Workspace) bool {
			return workspace.path == folderPath
		})

		if index == -1 {
			s.workspacesMutex.Unlock()

			s.warn(ctx, "Workspace not found: '%s'", folder.URI)
			continue
		}

		workspace := s.workspaces[index]
		s.workspaces = slices.Delete(s.workspaces, index, index+1)

		s.workspacesMutex.Unlock()

		workspace.mutex.Lock()
		workspace.removeWatchers(s)
		workspace.mutex.Unlock()
	}

	for _, folder := range params.Event.Added {
		s.openWorkspace(ctx, uri.URI(folder.URI).Filename())
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

		fullPath := uri.URI(fileCreate.URI).Filename()
		proj := s.getProject(fullPath)

		if proj == nil {
			s.warn(ctx, "Failed to find project for file: '%s'", fileCreate.URI)
			continue
		}

		s.parseAndPublish(ctx, s.getWorkspaceForProject(proj), func() []*project.File {
			file := proj.AddFile(fullPath)

			if file == nil {
				s.warn(ctx, "Failed to add file to project: '%s'", fileCreate.URI)
				return nil
			}

			file.Data = &Document{}

			return []*project.File{file}
		})
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
			fullPath := uri.URI(file.URI).Filename()

			if path.Ext(fullPath) != ".fb" && path.Base(fullPath) != "project.toml" {
				for _, workspace := range s.getWorkspaces() {
					workspace.mutex.RLock()

					for _, proj := range workspace.projMap {
						if core.IsFilepathInside(fullPath, path.Join(proj.Path, "project.toml")) {
							if !yield(string(uri.File(path.Join(proj.Path, "project.toml")))) {
								workspace.mutex.RUnlock()
								return
							}
						}
					}

					workspace.mutex.RUnlock()
				}
			}
		}
	})

	for _, fileDelete := range params.Files {
		fullPath := uri.URI(fileDelete.URI).Filename()

		if path.Ext(fullPath) != ".fb" && path.Base(fullPath) != "project.toml" {
			s.deleteFilesUnder(ctx, fullPath)
			continue
		}

		proj := s.getProject(fullPath)

		if proj == nil {
			s.warn(ctx, "Failed to find project for file: '%s'", fileDelete.URI)
			continue
		}

		file, _ := s.getFile(fullPath)
		workspace := s.getWorkspaceForProject(proj)

		s.parseAndPublish(ctx, workspace, func() []*project.File {
			if !proj.RemoveFile(fullPath) {
				s.warn(ctx, "Failed to remove file from project: '%s'", fileDelete.URI)
				return nil
			}

			return nil
		})

		if file != nil {
			s.clearFileDiagnostics(ctx, file)
		}
	}

	return nil
}

func (s *Server) deleteFilesUnder(ctx context.Context, dir string) {
	for _, workspace := range s.getWorkspaces() {
		for _, proj := range workspace.projects() {
			var removed []*project.File

			workspace.mutex.RLock()

			for _, file := range proj.Files {
				if core.IsFilepathInside(dir, file.Path) {
					removed = append(removed, file)
				}
			}

			workspace.mutex.RUnlock()

			if len(removed) == 0 {
				continue
			}

			s.parseAndPublish(ctx, workspace, func() []*project.File {
				for _, file := range removed {
					proj.RemoveFile(file.Path)
				}

				return nil
			})

			for _, file := range removed {
				s.clearFileDiagnostics(ctx, file)
			}
		}
	}
}

func (s *Server) reloadWorkspacesIfProjectConfigChanged(ctx context.Context, it iter.Seq[string]) {
	var workspaces []*Workspace

	for uri_ := range it {
		if path.Base(uri_) == "project.toml" {
			workspace := s.getWorkspaceForProjectConfig(uri.URI(uri_).Filename())

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
	file, _ := s.getFile(params.TextDocument.URI.Filename())
	if file == nil {
		return nil
	}

	workspace := s.getWorkspace(file)

	s.parseAndPublish(ctx, workspace, func() []*project.File {
		// Set text contents
		if src, ok := file.Source.(*Source); ok {
			src.Apply(protocol.TextDocumentContentChangeEvent{Text: params.TextDocument.Text})
		} else {
			file.Source = &Source{
				lines: bytes.SplitAfter([]byte(params.TextDocument.Text), []byte{'\n'}),
			}
		}

		// Set document version
		document := file.Data.(*Document)
		document.mu.Lock()
		document.Version = params.TextDocument.Version
		document.mu.Unlock()

		return []*project.File{file}
	})

	return nil
}

func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	// Get file and its workspace
	file, _ := s.getFile(params.TextDocument.URI.Filename())
	if file == nil {
		return nil
	}

	workspace := s.getWorkspace(file)

	s.parseAndPublish(ctx, workspace, func() []*project.File {
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
		document := file.Data.(*Document)
		document.mu.Lock()
		document.Version = params.TextDocument.Version
		document.mu.Unlock()

		return []*project.File{file}
	})

	return nil
}

func (s *Server) DidClose(_ context.Context, _ *protocol.DidCloseTextDocumentParams) error {
	return nil
}

func (s *Server) parseAndPublish(ctx context.Context, workspace *Workspace, mutate func() []*project.File) {
	workspace.mutex.Lock()

	var files []*project.File

	if mutate != nil {
		files = mutate()
	}

	workspace.parseFiles(files)
	workspace.mutex.Unlock()

	s.publishDiagnostics(ctx)
}
