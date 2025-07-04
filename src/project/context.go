package project

import (
	"fireball/analyzer"
	"fireball/ast"
)

type context struct {
	modules []*Module
}

func (c *context) addFile(file *File) {
	filePath := file.Ast().ModulePath()

	if filePath.SegmentCount() == 0 {
		return
	}

	var module *Module

	for i := 1; i <= filePath.SegmentCount(); i++ {
		path := ast.StringPath{Segments: make([]string, i)}

		for j := 0; j < i; j++ {
			path.Segments[j] = filePath.SegmentAt(j)
		}

		module = c.getOrCreateModule(&path)
	}

	//goland:noinspection GoDfaNilDereference
	module.files = append(module.files, file)
}

func (c *context) getOrCreateModule(path *ast.StringPath) *Module {
	for _, module := range c.modules {
		if ast.PathEquals(module.path, path) {
			return module
		}
	}

	module := &Module{path: path}
	c.modules = append(c.modules, module)

	return module
}

// analyzer.Context

func (c *context) GetAbsoluteModule(path ast.PathLike) analyzer.Module {
	for _, module := range c.modules {
		if ast.PathEquals(module.path, path) {
			return module
		}
	}

	return nil
}
