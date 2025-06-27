package lsp

import (
	"cmp"
	"context"
	"fireball/project"
	"fireball/utils"
	"github.com/MineGame159/protocol"
	"github.com/fsnotify/fsnotify"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type server struct {
	logger *zap.Logger
	client protocol.Client

	projects []*project.Project

	astMutex sync.RWMutex
}

func newServer() *server {
	return &server{}
}

func (s *server) Initialize(_ context.Context, params *protocol.InitializeParams) (result *protocol.InitializeResult, err error) {
	defer stop(start(s, "Initialize"))

	// Open projects
	for _, folder := range params.WorkspaceFolders {
		s.openProject(folder)
	}

	// Return server info
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			Workspace: &protocol.ServerCapabilitiesWorkspace{
				WorkspaceFolders: &protocol.ServerCapabilitiesWorkspaceFolders{
					Supported:           true,
					ChangeNotifications: true,
				},
			},
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				Change: protocol.TextDocumentSyncKindFull,
			},
			SemanticTokensProvider: &SemanticTokensOptions{
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
				Full: &SemanticTokensFull{},
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

	// Analyze
	go s.analyze(true)

	return nil
}

func (s *server) Shutdown(_ context.Context) (err error) {
	defer stop(start(s, "Shutdown"))

	for _, proj := range s.projects {
		data := proj.Data.(*projectData)

		_ = data.watcher.Close()
	}

	return nil
}

func (s *server) Exit(_ context.Context) (err error) {
	defer stop(start(s, "Exit"))

	return nil
}

func (s *server) DidChangeWorkspaceFolders(_ context.Context, params *protocol.DidChangeWorkspaceFoldersParams) (err error) {
	defer stop(start(s, "DidChangeWorkspaceFolders"))

	// Add projects
	for _, folder := range params.Event.Added {
		s.openProject(folder)
	}

	// Remove projects
	for _, folder := range params.Event.Removed {
		uri_, err := uri.Parse(folder.URI)
		if err != nil {
			continue
		}

		path, err := filepath.Abs(uri_.Filename())
		if err != nil {
			continue
		}

		for i, proj := range s.projects {
			if proj.AbsolutePath == path {
				s.projects = slices.Delete(s.projects, i, i+1)
				s.logger.Info("Closed project", zap.String("path", proj.AbsolutePath))

				break
			}
		}
	}

	// Analyze
	go s.analyze(false)

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
	go s.analyze(false)

	return nil
}

// Analyze

func (s *server) analyze(forceWithoutParse bool) {
	defer stop(start(s, "analyze"))

	s.astMutex.Lock()
	defer s.astMutex.Unlock()

	for _, proj := range s.projects {
		s.analyzeProject(proj, forceWithoutParse)
	}
}

func (s *server) analyzeProject(proj *project.Project, forceWithoutParse bool) {
	data := proj.Data.(*projectData)

	data.filesMutex.Lock()
	defer data.filesMutex.Unlock()

	// Analyze
	proj.Analyze(forceWithoutParse)

	// Publish diagnostics
	for file := range proj.Files() {
		doc := file.Provider().(*document)
		diagnostics := file.Diagnostics()

		if !doc.hasPublishedDiagnostics && len(diagnostics) == 0 {
			continue
		}

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

		doc.hasPublishedDiagnostics = len(diagnostics) > 0
	}
}

// Utils

func (s *server) getFile(uri uri.URI) *project.File {
	uriPath, err := filepath.Abs(uri.Filename())
	if err != nil {
		return nil
	}

	if !strings.HasSuffix(uriPath, ".fb") {
		return nil
	}

	for _, proj := range s.projects {
		if file := s.getFileFromProject(proj, uriPath); file != nil {
			return file
		}
	}

	return nil
}

func (s *server) getFileFromProject(proj *project.Project, path string) *project.File {
	data := proj.Data.(*projectData)

	data.filesMutex.Lock()
	defer data.filesMutex.Unlock()

	// Check src folder
	if filepath.Dir(path) != filepath.Join(proj.AbsolutePath, "src") {
		return nil
	}

	// Find file
	for file := range proj.Files() {
		if file.AbsolutePath() == path {
			return file
		}
	}

	// Create file
	doc := newDocument(path)
	return proj.AddFile(doc)
}

type projectData struct {
	watcher    *fsnotify.Watcher
	filesMutex sync.Mutex
}

func (s *server) openProject(folder protocol.WorkspaceFolder) {
	// Parse URI
	uri_, err := uri.Parse(folder.URI)
	if err != nil {
		return
	}

	// Open project
	proj, err := project.OpenProject(uri_.Filename())
	if err != nil {
		return
	}

	data := &projectData{}
	proj.Data = data

	// Load source files
	s.scanProjectFiles(proj)

	// Setup file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	if err := watcher.Add(filepath.Join(proj.AbsolutePath, "src")); err != nil {
		_ = watcher.Close()
		return
	}

	data.watcher = watcher

	go func() {
		for {
			select {
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				s.logger.Error("Watcher error", zap.Error(err))

			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}

				if ev.Has(fsnotify.Create) || ev.Has(fsnotify.Rename) || ev.Has(fsnotify.Remove) {
					if s.scanProjectFiles(proj) {
						go s.analyze(true)
					}
				}
			}
		}
	}()

	// Add project
	s.projects = append(s.projects, proj)

	slices.SortFunc(s.projects, func(a, b *project.Project) int {
		return cmp.Compare(len(b.AbsolutePath), len(a.AbsolutePath))
	})

	s.logger.Info("Opened project", zap.String("path", proj.AbsolutePath))
}

func (s *server) scanProjectFiles(proj *project.Project) (changed bool) {
	data := proj.Data.(*projectData)

	data.filesMutex.Lock()
	defer data.filesMutex.Unlock()

	entries, err := os.ReadDir(filepath.Join(proj.AbsolutePath, "src"))
	if err != nil {
		return
	}

	// New files
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".fb") {
			path := filepath.Join(proj.AbsolutePath, "src", entry.Name())

			if !proj.HasFile(path) {
				doc := newDocument(path)
				proj.AddFile(doc)

				changed = true
			}
		}
	}

	// Deleted files
	var filesToRemove []string

	for file := range proj.Files() {
		exists := slices.ContainsFunc(entries, func(entry os.DirEntry) bool {
			return entry.Name() == file.SrcRelativePath()
		})

		if !exists {
			filesToRemove = append(filesToRemove, file.AbsolutePath())
		}
	}

	for _, path := range filesToRemove {
		proj.RemoveFile(path)
		changed = true
	}

	return
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
