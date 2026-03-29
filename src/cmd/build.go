package main

import (
	"fireball/abi"
	"fireball/codegen"
	"fireball/core"
	"fireball/ir/llvm"
	"fireball/project"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

			// Compile
			if err := os.MkdirAll(filepath.Join(proj.Path, "build"), 0750); err != nil {
				return err
			}

			for _, file := range proj.Files {
				module := codegen.Generate(file.Decls, abi.AMD64, file.ExprInfos, file.NodeTypes)

				file, err := os.Create(filepath.Join(proj.Path, "build", getBuildFileName(file)+".ll"))
				if err != nil {
					return err
				}

				if err := llvm.Write(module, file); err != nil {
					_ = file.Close()
					return err
				}

				_ = file.Close()
			}

			return nil
		},
	}
}

var nameReplacer = strings.NewReplacer(" ", "_", "-", "_")

func getBuildFileName(file *project.File) string {
	name := filepath.Base(file.Path)

	if index := strings.IndexRune(name, '.'); index != -1 {
		name = name[:index]
	}

	return nameReplacer.Replace(name)
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
