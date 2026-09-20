package lsp

import (
	"cmp"
	"context"
	"fireball/ast"
	"fireball/codegen"
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
	mutex *reentrantRWMutex

	projMap map[string]*project.Project
	depMap  map[project.Dependency]*project.Project

	typeEnv        *sema.TypeEnvironment
	instantiations *types.InstantiationCache
}

func (s *Server) openWorkspace(ctx context.Context, path string) {
	// Create workspace
	workspace := &Workspace{
		server: s,
		path:   path,
		mutex:  newReentrantRWMutex(),
	}

	if !workspace.reload(s, ctx) {
		return
	}

	// Add workspace
	s.workspacesMutex.Lock()
	s.workspaces = append(s.workspaces, workspace)
	s.workspacesMutex.Unlock()

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

	// Lock the workspace mutex
	w.mutex.Lock()

	// Update native watcher
	w.removeWatchers(s)

	for _, proj := range projMap {
		if s.nativeWatcher != nil {
			s.nativeWatcher.AddRecursive(filepath.Join(proj.Path, "src"))
		}
	}

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

func (w *Workspace) projects() []*project.Project {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	projects := make([]*project.Project, 0, len(w.projMap))

	for _, proj := range w.projMap {
		projects = append(projects, proj)
	}

	return projects
}

func (w *Workspace) parseFiles(files []*project.File) {
	defer core.Scope()()

	// Order projects by dependency (dependencies before dependents)
	ordered := project.OrderProjects(w.projMap, w.depMap)

	// Parse
	for _, proj := range ordered {
		for _, file := range proj.Files {
			if len(files) == 0 || slices.Contains(files, file) {
				file.Parse(w.server.Env)
			}
		}
	}

	for _, proj := range ordered {
		proj.AssignFilesToModules()
	}

	for _, proj := range ordered {
		proj.Module.CheckCollisions()
	}

	// Resolve
	builtins := project.GetBuiltins(w.projMap)
	instantiations := types.NewInstantiationCache()
	typeEnv := sema.NewTypeEnvironment(instantiations, builtins)

	for _, proj := range ordered {
		proj.Resolve(w.depMap, instantiations, typeEnv, builtins)
	}

	instantiations.SubstituteDependentTypes()

	// Check type-local collisions
	project.CheckTypeLocalCollisions(w.projMap, typeEnv)

	// Analyze
	for _, proj := range ordered {
		proj.Analyze(w.depMap, instantiations, typeEnv, builtins)
	}

	// Evaluate comp-time
	fileDataMap := make(map[*ast.File]codegen.FileData)

	for _, proj := range ordered {
		for _, file := range proj.Files {
			fileDataMap[file.Ast] = codegen.FileData{
				ExprInfos:   file.ExprInfos,
				NodeTypes:   file.NodeTypes,
				Evaluations: file.Evaluations,
			}
		}
	}

	for _, proj := range ordered {
		proj.EvalCompTime(instantiations, typeEnv, fileDataMap, builtins)
	}

	w.typeEnv = typeEnv
	w.instantiations = instantiations
}

func (w *Workspace) getProject(file string) *project.Project {
	projects := w.projects()

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
	for _, workspace := range s.getWorkspaces() {
		workspace.mutex.RLock()

		for _, proj := range workspace.projMap {
			for _, file := range proj.Files {
				if file.Path == path {
					workspace.mutex.RUnlock()

					return file, workspace.mutex.RLocker()
				}
			}
		}

		workspace.mutex.RUnlock()
	}

	return nil, nil
}

func (s *Server) getProject(file string) *project.Project {
	workspaces := s.getWorkspaces()

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
	for _, workspace := range s.getWorkspaces() {
		if slices.Contains(workspace.projects(), proj) {
			return workspace
		}
	}

	panic("lsp.Handler.getWorkspaceForProject() - Project doesn't belong to any workspace")
}

func (s *Server) getWorkspace(file *project.File) *Workspace {
	for _, workspace := range s.getWorkspaces() {
		workspace.mutex.RLock()

		found := false

		for _, proj := range workspace.projMap {
			if slices.Contains(proj.Files, file) {
				found = true
			}

			if found {
				break
			}
		}

		workspace.mutex.RUnlock()

		if found {
			return workspace
		}
	}

	panic("lsp.Handler.getWorkspace() - File doesn't belong to any workspace")
}

func (s *Server) getWorkspaceForProjectConfig(path string) *Workspace {
	for _, workspace := range s.getWorkspaces() {
		for _, proj := range workspace.projects() {
			if filepath.Join(proj.Path, "project.toml") == path {
				return workspace
			}
		}
	}

	return nil
}
