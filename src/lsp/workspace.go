package lsp

import (
	"cmp"
	"context"
	"fireball/core"
	"fireball/project"
	"fireball/sema"
	"fireball/types"
	"path/filepath"
	"slices"
	"sync"
)

type Workspace struct {
	server *Server

	path  string
	mutex sync.RWMutex

	projMap map[string]*project.Project
	depMap  map[project.Dependency]*project.Project
}

func (s *Server) openWorkspace(ctx context.Context, path string) {
	// Create workspace
	workspace := &Workspace{
		server: s,
		path:   path,
	}

	if !workspace.reload(s, ctx) {
		return
	}

	// Add workspace
	s.workspaces = append(s.workspaces, workspace)

	s.info(ctx, "Opened workspace at: %s", path)
}

func (w *Workspace) reload(s *Server, ctx context.Context) bool {
	// Open project
	proj, err := project.Open(w.path)
	if err != nil {
		s.warn(ctx, "Failed to open project: %s", err.Error())
		return false
	}

	// Load hierarchy
	projMap, depMap, err := project.LoadHierarchy(proj)
	if err != nil {
		s.error(ctx, "Failed to load hierarchy: %s", err.Error())
		return false
	}

	// Update native watcher
	if s.nativeWatcher != nil {
		w.removeWatchers(s)

		for _, proj := range projMap {
			s.nativeWatcher.AddRecursive(filepath.Join(proj.Path, "src"))
		}
	}

	// Lock the workspace mutex
	w.mutex.Lock()

	// Set maps
	w.projMap = projMap
	w.depMap = depMap

	// Create documents
	for _, proj := range projMap {
		for _, file := range proj.Files {
			file.Data = &Document{}
		}
	}

	// Initial parse
	w.parseFiles(nil)
	w.mutex.Unlock()

	s.publishDiagnostics(ctx)

	return true
}

func (w *Workspace) removeWatchers(s *Server) {
	if s.nativeWatcher != nil {
		for _, proj := range w.projMap {
			s.nativeWatcher.Remove(filepath.Join(proj.Path, "src"))
		}
	}
}

func (w *Workspace) parseFiles(files []*project.File) {
	ordered := project.OrderProjects(w.projMap, w.depMap)

	for _, proj := range ordered {
		proj.Parse(files, w.server.Env)
	}

	instantiations := types.NewInstantiationCache()
	typeEnv := sema.NewTypeEnvironment(instantiations)

	for _, proj := range ordered {
		proj.Resolve(w.depMap, instantiations, typeEnv)
	}

	for _, proj := range ordered {
		proj.Analyze(w.depMap, instantiations, typeEnv)
	}
}

func (w *Workspace) getProject(file string) *project.Project {
	projects := make([]*project.Project, 0, len(w.projMap))

	for _, proj := range w.projMap {
		projects = append(projects, proj)
	}

	slices.SortFunc(projects, func(a, b *project.Project) int {
		return cmp.Compare(len(b.Path), len(a.Path))
	})

	for _, proj := range projects {
		if core.IsFilepathInside(proj.Path, file) {
			return proj
		}
	}

	return nil
}

func (s *Server) getFile(path string) (*project.File, sync.Locker) {
	for _, workspace := range s.workspaces {
		for _, proj := range workspace.projMap {
			for _, file := range proj.Files {
				if file.Path == path {
					return file, workspace.mutex.RLocker()
				}
			}
		}
	}

	return nil, nil
}

func (s *Server) getProject(file string) *project.Project {
	workspaces := append([]*Workspace{}, s.workspaces...)

	slices.SortFunc(workspaces, func(a, b *Workspace) int {
		return cmp.Compare(len(b.path), len(a.path))
	})

	for _, workspace := range workspaces {
		if core.IsFilepathInside(workspace.path, file) {
			return workspace.getProject(file)
		}
	}

	return nil
}

func (s *Server) getWorkspaceForProject(proj *project.Project) *Workspace {
	for _, workspace := range s.workspaces {
		for _, p := range workspace.projMap {
			if p == proj {
				return workspace
			}
		}
	}

	panic("lsp.Handler.getWorkspaceForProject() - Project doesn't belong to any workspace")
}

func (s *Server) getWorkspace(file *project.File) *Workspace {
	for _, workspace := range s.workspaces {
		for _, proj := range workspace.projMap {
			for _, f := range proj.Files {
				if f == file {
					return workspace
				}
			}
		}
	}

	panic("lsp.Handler.getWorkspace() - File doesn't belong to any workspace")
}

func (s *Server) getWorkspaceForProjectConfig(path string) *Workspace {
	for _, workspace := range s.workspaces {
		for _, proj := range workspace.projMap {
			if filepath.Join(proj.Path, "project.toml") == path {
				return workspace
			}
		}
	}

	return nil
}
