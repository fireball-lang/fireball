package project

import (
	"io/fs"
	"path/filepath"
)

type Project struct {
	Path string

	Config Config
	Files  []*File
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
	}

	if err := filepath.WalkDir(filepath.Join(path, "src"), func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && filepath.Ext(path) == ".fb" {
			project.Files = append(project.Files, newFile(path))
		}

		return err
	}); err != nil {
		return nil, err
	}

	return project, nil
}

func (p *Project) Parse() {
	for _, file := range p.Files {
		file.parse()
	}

	for _, file := range p.Files {
		file.analyze()
	}
}
