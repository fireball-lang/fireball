package main

import (
	"fireball/cmd/build"
	"fireball/project"
	"fireball/utils"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
)

func buildPath(path string, profile build.Profile, entrypointFunc build.EntrypointFunc) (string, error) {
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

	// Build
	if !hasError {
		binaryPath, err := build.Build(proj, profile, entrypointFunc)

		fmt.Println()
		color.Green("Build successful")

		return binaryPath, err
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
	err = filepath.WalkDir(filepath.Join(proj.AbsolutePath, "src"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.HasSuffix(path, ".fb") {
			proj.AddFile(&simpleFileContentsProvider{
				path:    path,
				changed: true,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
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
