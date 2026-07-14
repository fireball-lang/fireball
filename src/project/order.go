package project

import (
	"sort"
)

const coreProjectName = "core"

// OrderProjects returns the projects in dependency order, ensuring that a
// project is always processed after the projects it depends on. The language
// core ("core") is treated as an implicit dependency of every project.
func OrderProjects(projMap map[string]*Project, depMap map[Dependency]*Project) []*Project {
	placed := make(map[string]bool)
	order := make([]*Project, 0, len(projMap))

	var visit func(name string)

	visit = func(name string) {
		if placed[name] {
			return
		}
		placed[name] = true

		proj := projMap[name]
		if proj == nil {
			return
		}

		depNames := make([]string, 0, len(proj.Config.Dependencies)+1)
		for _, dep := range proj.Config.Dependencies {
			if depProj, ok := depMap[dep]; ok {
				depNames = append(depNames, depProj.Config.Name)
			}
		}

		// Every project implicitly depends on the language core.
		if name != coreProjectName {
			depNames = append(depNames, coreProjectName)
		}

		sort.Strings(depNames)
		for _, depName := range depNames {
			visit(depName)
		}

		order = append(order, proj)
	}

	names := make([]string, 0, len(projMap))
	for name := range projMap {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		visit(name)
	}

	return order
}
