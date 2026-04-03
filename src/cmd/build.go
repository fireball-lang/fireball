package main

import (
	"time"

	"github.com/spf13/cobra"
)

func getBuildCmd() *cobra.Command {
	var profileName string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Builds a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()

			// Parse
			proj, err := parseProject(start)
			if err != nil {
				return err
			}
			if proj == nil {
				return nil
			}

			// Build
			_, err = buildProject(proj, profileName, start, normalEntrypointProvider)

			return err
		},
	}

	cmd.Flags().StringVarP(&profileName, "profile", "p", "debug", "profile to build the project with")

	return cmd
}
