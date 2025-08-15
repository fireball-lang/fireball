package build

import (
	"fireball/ir"
	"fireball/profiler"
	"fireball/project"
	"fmt"
	"os"
	"path/filepath"
)

type LtoKind uint8

const (
	None LtoKind = iota
	Thin
)

type Profile struct {
	Name string
	Opt  uint
	Lto  LtoKind
}

type EntrypointFunc func(proj *project.Project, m *ir.Module, main *ir.Function, summary bool) string

func Build(proj *project.Project, profile Profile, entrypointFunc EntrypointFunc) (string, error) {
	defer profiler.Event()()

	// Get target
	target, err := getTarget()
	if err != nil {
		return "", err
	}

	// Create output folder
	outPath := filepath.Join(proj.AbsolutePath, "out", profile.Name+"-"+target.Name)

	if err := os.MkdirAll(outPath, 0750); err != nil {
		return "", fmt.Errorf("failed to create output folder: %w", err)
	}

	// Run codegen for project files
	irPaths, err := runCodegenProject(proj, profile, target, outPath)
	if err != nil {
		return "", err
	}

	// Run codegen for entrypoint
	entrypointIrPath, entrypointName, err := runCodegenEntrypoint(proj, profile, target, outPath, entrypointFunc)
	if err != nil {
		return "", err
	}

	irPaths = append(irPaths, entrypointIrPath)

	// Compile
	var linkPaths []string

	{
		end := profiler.EventNamed("Compile")

		if profile.Lto == None {
			// Compile IR to OBJ
			linkPaths, err = runForEach(compileParams{profile, target}, irPaths, compileToObj)
			if err != nil {
				return "", err
			}
		} else {
			// Compile IR to BC
			linkPaths, err = runForEach(0, irPaths, compileToBc)
			if err != nil {
				return "", err
			}
		}

		end()
	}

	// Link binary
	binaryName := proj.Config.Name
	if entrypointName != "" {
		binaryName += "_" + entrypointName
	}
	binaryName += target.ExecutableFileExtension

	return linkBinary(profile, target, outPath, linkPaths, binaryName)
}
