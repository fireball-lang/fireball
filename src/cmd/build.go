package main

import (
	"fireball/build"
	"fireball/core"
	"fireball/project"
	"fireball/toolchain"
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func getBuildCmd() *cobra.Command {
	var profileName string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Builds a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := project.Open(".")
			if err != nil {
				return err
			}

			// Parse
			proj.Parse()

			hasErrors := false

			for _, file := range proj.Files {
				path, err := filepath.Rel(proj.Path, file.Path)
				if err != nil {
					panic(err)
				}

				for diag := range file.Diagnostics() {
					printDiagnostic(path, diag)

					if diag.Kind == core.Error {
						hasErrors = true
					}
				}
			}

			if hasErrors {
				return nil
			}

			// Build
			if err := toolchain.Validate(); err != nil {
				return err
			}

			target, err := toolchain.GetTarget()
			if err != nil {
				return err
			}

			profile, ok := proj.Config.Profiles[profileName]
			if !ok {
				return fmt.Errorf("unknown profile: '%s'", profileName)
			}

			_, err = build.Build(proj, target, profile)

			return err
		},
	}

	cmd.Flags().StringVarP(&profileName, "profile", "p", "debug", "profile to build the project with")

	return cmd
}

func printDiagnostic(filePath string, diag core.Diagnostic) {
	switch diag.Kind {
	case core.Warning:
		_, _ = color.New(color.FgYellow, color.Bold).Print("warning")
	case core.Error:
		_, _ = color.New(color.FgRed, color.Bold).Print("error")
	}

	_, _ = color.New(color.Bold).Printf(": %s\n", diag.Message)
	_, _ = color.New(color.FgBlue, color.Bold).Print("  --> ")

	fmt.Printf("%s:%d:%d\n", filePath, diag.Range.Start.Line, diag.Range.Start.Column)
}
