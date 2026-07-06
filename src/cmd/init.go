package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/spf13/cobra"
)

func getInitCmd() *cobra.Command {
	var name string
	var initGit bool

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initializes a project in an existing directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			// Get path
			path, err := os.Getwd()
			if err != nil {
				return err
			}

			if len(args) > 0 {
				if filepath.IsAbs(args[0]) {
					path = args[0]
				} else {
					path = filepath.Join(path, args[0])
				}
			}

			// Check directory
			info, err := os.Stat(path)

			if err != nil {
				return fmt.Errorf("path does not exist: '%s'", path)
			} else if !info.IsDir() {
				return fmt.Errorf("path is not a directory: '%s'", path)
			}

			// Check existing project
			_, err = os.Stat(filepath.Join(path, "project.toml"))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}

			if err == nil {
				return fmt.Errorf("project already exists at: '%s'", path)
			}

			// Generate
			if name == "" {
				name = info.Name()
			}

			return generateProjectTemplate(path, name, initGit)
		}),
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "name of the project, default is directory name")
	cmd.Flags().BoolVarP(&initGit, "git", "g", false, "initializes a Git repository inside the project")

	return cmd
}

const (
	templateProjectToml = `name = "{name}"
lib-c = true
`

	templateMainFb = `mod {name};

func main() {
	// code
}
`
)

func generateProjectTemplate(path string, name string, initGit bool) error {
	// Generate
	if err := generateFile(filepath.Join(path, "project.toml"), templateProjectToml, name); err != nil {
		return err
	}
	if err := generateFile(filepath.Join(path, "src", "main.fb"), templateMainFb, name); err != nil {
		return err
	}

	// Git
	if initGit {
		repo, err := git.PlainInit(path, false)
		if err != nil {
			return err
		}

		_ = repo.Close()
	}

	return nil
}

func generateFile(path string, contents string, name string) error {
	// Make parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	// Write file
	data := []byte(strings.ReplaceAll(contents, "{name}", name))
	return os.WriteFile(path, data, 0666)
}
