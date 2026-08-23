package main

import (
	"fireball/cfg"
	"fireball/project"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func getBuildCmd() *cobra.Command {
	var profileName string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Builds a project",
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			start := time.Now()

			// Parse
			proj, projMap, err := parseProject(cfg.GetHost(), &start)
			if err != nil {
				return err
			}

			if proj == nil {
				os.Exit(1)
			}

			// Build
			entrypointProvider := normalEntrypointProvider
			if proj.Config.Kind != project.Executable {
				entrypointProvider = nil
			}

			_, err = buildProject(proj, projMap, profileName, start, entrypointProvider)

			return err
		}),
	}

	cmd.Flags().StringVarP(&profileName, "profile", "p", "debug", "profile to build the project with")

	return cmd
}
