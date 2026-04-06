package project

import (
	"io/fs"
	"path/filepath"
)

type Project struct {
	Path string

	Config Config
	Files  []*File

	Module *Module
}

func Open(path string) (*Project, error) {
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
	p.Module = &Module{Name: p.Config.Name}

	for _, file := range p.Files {
		file.parse()
		p.assignFileToModule(file)
	}

	for _, file := range p.Files {
		file.resolve()
	}

	for _, file := range p.Files {
		file.analyze()
	}
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
