package main

import (
	"fireball/codegen"
	"fireball/project"
	"fireball/utils"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Open project
	proj, err := project.OpenProject("./example")
	if err != nil {
		log.Fatalln(err.Error())
	}

	// Analyze all files in src folder
	entries, err := os.ReadDir(filepath.Join(proj.AbsolutePath, "src"))
	if err != nil {
		log.Fatalln(err.Error())
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
			log.Fatalln(err.Error())
		}

		for file := range proj.Files() {
			path := file.SrcRelativePath()
			path = strings.ReplaceAll(path, "/", "-")
			path = strings.TrimSuffix(path, ".fb") + ".ll"
			path = filepath.Join(proj.AbsolutePath, "out", path)

			f, err := os.Create(path)
			if err != nil {
				log.Fatalln(err.Error())
			}

			m := codegen.Gen(file.Ast(), file.SrcRelativePath())
			err = m.Write(f)

			_ = f.Close()

			if err != nil {
				log.Fatalln(err.Error())
			}
		}

		// Compile
		cmd := exec.Command("clang", "-o", proj.Config.Name)
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
			log.Fatalln(err)
		}

		fmt.Println()

		// Run
		cmd = exec.Command(filepath.Join(proj.AbsolutePath, "out", proj.Config.Name))
		cmd.Stdout = os.Stdout

		err = cmd.Run()
		if err != nil {
			log.Fatalln(err)
		}
	}
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
