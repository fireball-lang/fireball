package lsp

import (
	"context"
	"fireball/project"
	"fireball/symbols"
	"sync"
)

type Workspace struct {
	path  string
	mutex sync.RWMutex

	projMap map[string]*project.Project
	depMap  map[project.Dependency]*project.Project
}

func (h *Handler) openWorkspace(ctx context.Context, path string) {
	// Open project
	proj, err := project.Open(path)
	if err != nil {
		h.warning(ctx, "Failed to open project: %s", err.Error())
		return
	}

	// Load hierarchy
	projMap, depMap, err := project.LoadHierarchy(proj)
	if err != nil {
		h.error(ctx, "Failed to load hierarchy: %s", err.Error())
		return
	}

	// Create documents
	for _, proj := range projMap {
		for _, file := range proj.Files {
			file.Data = &Document{}
		}
	}

	// Create workspace
	workspace := &Workspace{
		path:    path,
		projMap: projMap,
		depMap:  depMap,
	}

	h.workspaces = append(h.workspaces, workspace)

	h.info(ctx, "Opened workspace at: %s", path)

	// Initial parse
	workspace.parseFiles(nil)
}

func (w *Workspace) parseFiles(files []*project.File) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	for _, proj := range w.projMap {
		proj.Parse(files)
	}

	methodTable := symbols.NewMethodTable()

	for _, proj := range w.projMap {
		proj.Resolve(w.depMap, methodTable)
	}

	for _, proj := range w.projMap {
		proj.Analyze(w.depMap, methodTable)
	}
}

func (h *Handler) getFile(path string) *project.File {
	for _, workspace := range h.workspaces {
		for _, proj := range workspace.projMap {
			for _, file := range proj.Files {
				if file.Path == path {
					return file
				}
			}
		}
	}

	return nil
}

func (h *Handler) getWorkspace(file *project.File) *Workspace {
	for _, workspace := range h.workspaces {
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
