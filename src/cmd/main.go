package main

import (
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
)

func buildCommand() *cobra.Command {
	opt := uint8(0)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := build(".", opt)
			return err
		},
	}

	cmd.Flags().Uint8VarP(&opt, "opt", "O", 0, "Optimization level. [-O0, -O1, -O2, or -O3] (default = '-O0')")

	return cmd
}

func runCommand() *cobra.Command {
	opt := uint8(0)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the project.",
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := build(".", opt)
			if err != nil {
				return err
			}
			if path == "<errors>" {
				return nil
			}

			cmd := exec.Command(path)
			cmd.Stdout = os.Stdout

			return cmd.Run()
		},
	}

	cmd.Flags().Uint8VarP(&opt, "opt", "O", 0, "Optimization level. [-O0, -O1, -O2, or -O3] (default = '-O0')")

	return cmd
}

func main() {
	root := cobra.Command{
		Use:     "fireball",
		Short:   "Tooling for the Fireball programming language",
		Version: "0.1.0",
	}

	root.AddCommand(
		buildCommand(),
		runCommand(),
	)

	if err := root.Execute(); err != nil {
		log.Fatalln(err.Error())
	}
}
