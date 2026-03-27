package main

import (
	"fireball/core"
	"fireball/project"
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func getBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Builds a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := project.Open(".")
			if err != nil {
				return err
			}

			proj.Parse()

			for _, file := range proj.Files {
				path, err := filepath.Rel(proj.Path, file.Path)
				if err != nil {
					panic(err)
				}

				for diag := range file.Diagnostics() {
					printDiagnostic(path, diag)
				}
			}

			return nil
		},
	}
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
