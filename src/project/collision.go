package project

import (
	"fireball/core"
	"fireball/sema"
	"path/filepath"
)

func CheckTypeLocalCollisions(projMap map[string]*Project, env *sema.TypeEnvironment) {
	defer core.Scope()()

	env.CheckCollisions(func(diagnostic core.Diagnostic) {
		file := getFile(projMap, diagnostic.Path)
		file.resolveDiagnostics = append(file.resolveDiagnostics, diagnostic)
	})
}

func getFile(projMap map[string]*Project, path string) *File {
	for _, project := range projMap {
		src := filepath.Join(project.Path, "src")

		if core.IsFilepathInside(src, path) {
			for _, file := range project.Files {
				if file.Path == path {
					return file
				}
			}
		}
	}

	panic("project.getFile() - Failed to find file for path '" + path + "'")
}
