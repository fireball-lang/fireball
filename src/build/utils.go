package build

import (
	"fireball/project"
	"path/filepath"
	"strings"
)

var nameReplacer = strings.NewReplacer(" ", "_", "-", "_")

func getBuildFileName(file *project.File) string {
	name := filepath.Base(file.Path)

	if index := strings.IndexRune(name, '.'); index != -1 {
		name = name[:index]
	}

	return nameReplacer.Replace(name)
}
