package main

import (
	"fireball/core"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

func getRunCmd() *cobra.Command {
	var profileName string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Builds and runs a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()

			// Parse
			proj, projMap, err := parseProject(start)
			if err != nil {
				return err
			}
			if proj == nil {
				return nil
			}

			// Build
			exePath, err := buildProject(proj, projMap, profileName, start, normalEntrypointProvider)
			fmt.Println()

			if err != nil {
				return err
			}

			// Run
			return runProgram(exePath)
		},
	}

	cmd.Flags().StringVarP(&profileName, "profile", "p", "debug", "profile to build the project with")

	return cmd
}

func runProgram(path string) error {
	defer core.Scope()()

	exeCmd := exec.Command(path)

	exeCmd.Stdin = os.Stdin
	exeCmd.Stdout = os.Stdout
	exeCmd.Stderr = os.Stderr

	err := exeCmd.Run()

	if exeCmd.ProcessState.ExitCode() != 0 {
		return nil
	}

	return err
}
