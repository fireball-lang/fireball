package build

import (
	"fireball/codegen"
	"fireball/ir"
	"fireball/ir/llvm"
	"fireball/project"
	"os"
	"path/filepath"
	"strings"
)

func runCodegenProject(proj *project.Project, profile Profile, target Target, outPath string) ([]string, error) {
	var irPaths []string

	for file := range proj.Files() {
		path := file.SrcRelativePath()
		path = strings.ReplaceAll(path, string(filepath.Separator), "-")
		path = strings.TrimSuffix(path, ".fb") + ".ll"
		path = filepath.Join(outPath, path)

		m := codegen.Emit(file.Ast(), file.AbsolutePath(), target.Arch, target.CallConv, profile.Lto == Thin)

		m.DataLayout = target.DataLayout
		m.Triple = target.Triple

		if err := writeModule(m, path); err != nil {
			return nil, err
		}

		irPaths = append(irPaths, path)
	}

	return irPaths, nil
}

func runCodegenEntrypoint(proj *project.Project, profile Profile, target Target, outPath string, entrypointFunc EntrypointFunc) (string, string, error) {
	// Create IR module
	m := ir.NewModule()

	m.Path = "__entrypoint.ll"
	m.DataLayout = target.DataLayout
	m.Triple = target.Triple

	codegen.AddModuleMetaFlags(m, profile.Lto == Thin)

	mainTyp := &ir.FunctionType{Returns: ir.I32}
	main := m.NewFunction("main", mainTyp, nil)

	name := entrypointFunc(proj, m, main, profile.Lto == Thin)

	// Run codegen
	entrypointName := "_"
	if name != "" {
		entrypointName += "_" + name
	}
	entrypointName += "_entrypoint.ll"

	path := filepath.Join(outPath, entrypointName)

	if err := writeModule(m, path); err != nil {
		return "", "", err
	}

	return path, name, nil
}

func writeModule(m *ir.Module, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	err = llvm.Write(m, f)
	_ = f.Close()

	return err
}
