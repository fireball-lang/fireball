package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func getNewCmd() *cobra.Command {
	var name string
	var initGit bool

	cmd := &cobra.Command{
		Use:   "new <path>",
		Short: "Initializes a project in a new directory",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			// Get path
			path, err := os.Getwd()
			if err != nil {
				return err
			}

			if filepath.IsAbs(args[0]) {
				path = args[0]
			} else {
				path = filepath.Join(path, args[0])
			}

			// Check directory
			_, err = os.Stat(path)

			if err == nil {
				return fmt.Errorf("directory already exists: '%s'", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			// Create directory
			if err := os.MkdirAll(path, 0750); err != nil {
				return err
			}

			// Generate
			if name == "" {
				name = filepath.Base(path)
			}

			return generateProjectTemplate(path, name, initGit)
		}),
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "name of the project, default is directory name")
	cmd.Flags().BoolVarP(&initGit, "git", "g", false, "initializes a Git repository inside the project")

	return cmd
}
