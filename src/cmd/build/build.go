package build

import (
	"fireball/ir"
	"fireball/project"
	"fmt"
	"os"
	"path/filepath"
)

type Profile struct {
	Name string
	Opt  uint
}

type EntrypointFunc func(proj *project.Project, m *ir.Module, main *ir.Function) string

func Build(proj *project.Project, profile Profile, entrypointFunc EntrypointFunc) (string, error) {
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
	irPaths, err := runCodegenProject(proj, target, outPath)
	if err != nil {
		return "", err
	}

	// Run codegen for entrypoint
	entrypointIrPath, entrypointName, err := runCodegenEntrypoint(proj, target, outPath, entrypointFunc)
	if err != nil {
		return "", err
	}

	irPaths = append(irPaths, entrypointIrPath)

	// Compile IR to OBJ
	objPaths, err := runForEach(profile, irPaths, compile)
	if err != nil {
		return "", err
	}

	// Link binary
	binaryName := proj.Config.Name
	if entrypointName != "" {
		binaryName += "_" + entrypointName
	}

	return linkBinary(profile, outPath, objPaths, binaryName)
}
