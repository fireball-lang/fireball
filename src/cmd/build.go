package main

import (
	"fireball/codegen"
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

func build(path string, opt uint8) (string, error) {
	start := time.Now()

	defer func() {
		duration := time.Now().Sub(start)
		fmt.Printf("  Took %s\n", duration)
		fmt.Println()
	}()

	// Open project
	proj, err := project.OpenProject(path)
	if err != nil {
		return "", err
	}

	// Analyze all files in src folder
	entries, err := os.ReadDir(filepath.Join(proj.AbsolutePath, "src"))
	if err != nil {
		return "", err
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

	proj.Analyze()

	// Report diagnostics
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

	// Compile
	if !hasError {
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

		// Compile
		cmd := exec.Command("clang", fmt.Sprintf("-O%d", opt), "-o", proj.Config.Name)
		cmd.Dir = filepath.Join(proj.AbsolutePath, "out")
		cmd.Stderr = os.Stderr

		for file := range proj.Files() {
			path := file.SrcRelativePath()
			path = strings.ReplaceAll(path, "/", "-")
			path = strings.TrimSuffix(path, ".fb") + ".ll"

			cmd.Args = append(cmd.Args, path)
		}

		err = cmd.Run()
		if err != nil {
			return "", err
		}

		fmt.Println()
		color.Green("Build successful")

		return filepath.Join(proj.AbsolutePath, "out", proj.Config.Name), nil
	}

	fmt.Println()
	color.Red("Build failed")

	return "<errors>", nil
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
