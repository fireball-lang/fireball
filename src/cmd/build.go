package main

import (
	"fireball/codegen"
	"fireball/llvm"
	"fireball/project"
	"fireball/utils"
	"fmt"
	"github.com/fatih/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func build(path string, suffix string, opt uint8, entrypointCb func(proj *project.Project) *llvm.Module) (string, error) {
	start := time.Now()

	defer func() {
		duration := time.Now().Sub(start)
		fmt.Printf("  Took %s\n", duration)
		fmt.Println()
	}()

	// Open project
	proj, err := openAndAnalyzeProject(path)
	if err != nil {
		return "", err
	}

	// Report diagnostics
	hasError := reportDiagnostics(proj)

	// Compile
	if !hasError {
		path, err := compile(proj, suffix, opt, entrypointCb)

		fmt.Println()
		color.Green("Build successful")

		return path, err
	}

	// Build failed
	fmt.Println()
	color.Red("Build failed")

	return "<errors>", nil
}

func openAndAnalyzeProject(path string) (*project.Project, error) {
	// Open project
	proj, err := project.OpenProject(path)
	if err != nil {
		return nil, err
	}

	// Analyze all files in src folder
	entries, err := os.ReadDir(filepath.Join(proj.AbsolutePath, "src"))
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".fb") {
			path := filepath.Join(proj.AbsolutePath, "src", entry.Name())

			proj.AddFile(&simpleFileContentsProvider{
				path:    path,
				changed: true,
			})
		}
	}

	proj.Analyze(true)

	return proj, nil
}

func reportDiagnostics(proj *project.Project) bool {
	hasError := false

	for file := range proj.Files() {
		diagnostics := file.Diagnostics()

		if len(diagnostics) > 0 {
			fmt.Print(file.SrcRelativePath())
			fmt.Println(":")

			for _, diagnostic := range diagnostics {
				fmt.Print("   ")
				fmt.Println(diagnostic)

				if diagnostic.Kind == utils.Error {
					hasError = true
				}
			}
		}
	}

	return hasError
}

func compile(proj *project.Project, suffix string, opt uint8, entrypointCb func(proj *project.Project) *llvm.Module) (string, error) {
	// Codegen
	err := os.MkdirAll(filepath.Join(proj.AbsolutePath, "out"), 0750)
	if err != nil {
		return "", err
	}

	for file := range proj.Files() {
		path := file.SrcRelativePath()
		path = strings.ReplaceAll(path, "/", "-")
		path = strings.TrimSuffix(path, ".fb") + ".ll"
		path = filepath.Join(proj.AbsolutePath, "out", path)

		f, err := os.Create(path)
		if err != nil {
			return "", err
		}

		m := codegen.Gen(file.Ast(), file.AbsolutePath())
		err = m.Write(f)

		_ = f.Close()

		if err != nil {
			return "", err
		}
	}

	exeName := proj.Config.Name
	if suffix != "" {
		exeName += "_" + suffix
	}

	entrypoint := entrypointCb(proj)

	entrypointName := "_"
	if suffix != "" {
		entrypointName += "_" + suffix
	}
	entrypointName += "_entrypoint.ll"

	{
		path := filepath.Join(proj.AbsolutePath, "out", entrypointName)

		f, err := os.Create(path)
		if err != nil {
			return "", err
		}

		err = entrypoint.Write(f)

		_ = f.Close()

		if err != nil {
			return "", err
		}
	}

	// Compile
	cmd := exec.Command("clang", fmt.Sprintf("-O%d", opt), "-o", exeName)
	cmd.Dir = filepath.Join(proj.AbsolutePath, "out")
	cmd.Stderr = os.Stderr

	for file := range proj.Files() {
		path := file.SrcRelativePath()
		path = strings.ReplaceAll(path, "/", "-")
		path = strings.TrimSuffix(path, ".fb") + ".ll"

		cmd.Args = append(cmd.Args, path)
	}

	cmd.Args = append(cmd.Args, entrypointName)

	err = cmd.Run()
	if err != nil {
		return "", err
	}

	return filepath.Join(proj.AbsolutePath, "out", exeName), nil
}

type simpleFileContentsProvider struct {
	path    string
	changed bool
}

func (s *simpleFileContentsProvider) AbsolutePath() string {
	return s.path
}

func (s *simpleFileContentsProvider) Contents() (string, bool) {
	if !s.changed {
		return "", false
	}

	bytes, err := os.ReadFile(s.path)
	if err != nil {
		panic(err.Error())
	}

	return string(bytes), true
}
