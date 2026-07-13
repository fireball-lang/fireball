package build

import (
	"fireball/project"
	"path/filepath"
	"strings"
)

var nameReplacer = strings.NewReplacer(" ", "_", "-", "_")

func getBuildFileName(file *project.File) string {
	name, err := filepath.Rel(filepath.Join(file.Proj.Path, "src"), file.Path)
	if err != nil {
		panic("build.getBuildFileName() - Failed to get src relative file path: " + err.Error())
	}

	if index := strings.IndexRune(name, '.'); index != -1 {
		name = name[:index]
	}

	name = strings.ReplaceAll(name, string(filepath.Separator), "___")

	return nameReplacer.Replace(name)
}
