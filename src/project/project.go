package project

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
	"io/fs"
	"path/filepath"
)

type Project struct {
	Path string

	Config Config
	Files  []*File

	Module *Module
}

type Method struct {
	Ast  *ast.File
	Type *types.Func
}

func Open(path string) (*Project, error) {
	defer core.Scope()()

	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	config, err := readConfig(filepath.Join(path, "project.toml"))
	if err != nil {
		return nil, err
	}

	project := &Project{
		Path:   path,
		Config: config,
		Files:  nil,
		Module: nil,
	}

	if err := filepath.WalkDir(filepath.Join(path, "src"), func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && filepath.Ext(path) == ".fb" {
			project.Files = append(project.Files, newFile(project, path))
		}

		return err
	}); err != nil {
		return nil, err
	}

	return project, nil
}

func (p *Project) Parse() {
	defer core.Scope()()

	p.Module = &Module{Name: p.Config.Name}

	for _, file := range p.Files {
		file.parse()
		p.assignFileToModule(file)
	}
}

func (p *Project) Resolve(depMap map[Dependency]*Project, methodTable symbols.MethodTable) {
	defer core.Scope()()

	root := p.getRootScope(depMap)

	for _, file := range p.Files {
		file.resolve(&root, methodTable)
	}
}

func (p *Project) Analyze(projMap map[Dependency]*Project, methodTable symbols.MethodTable) {
	defer core.Scope()()

	root := p.getRootScope(projMap)

	for _, file := range p.Files {
		file.analyze(&root, methodTable)
	}
}

func (p *Project) getRootScope(depMap map[Dependency]*Project) rootScope {
	modules := make([]*Module, 0, 1+len(p.Config.Dependencies))
	modules = append(modules, p.Module)

	for _, dep := range p.Config.Dependencies {
		proj, ok := depMap[dep]
		if !ok {
			panic("project.Project.getRootScope() - Missing dependency project")
		}

		modules = append(modules, proj.Module)
	}

	return rootScope{modules}
}

func (p *Project) assignFileToModule(file *File) {
	// Get ast.Mod
	path := file.Ast.Mod.Path.Entries

	if len(path) == 0 || path[0].Token.Text != p.Config.Name {
		return
	}

	// Assign to module
	mod := p.Module

	for _, leaf := range path[1:] {
		mod = mod.getOrCreateChild(leaf.Token.Text)
	}

	mod.Files = append(mod.Files, file)
}

// rootScope

type rootScope struct {
	modules []*Module
}

func (r *rootScope) GetScope(name string) (symbols.Scope, bool) {
	for _, module := range r.modules {
		if module.Name == name {
			return module, true
		}
	}

	return nil, false
}

func (r *rootScope) GetSymbol(_ string) (symbols.Symbol, bool) {
	return symbols.Symbol{}, false
}
