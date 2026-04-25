package main

import (
	"fireball/core"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	core.StartProfiler()
	scope := core.Scope()

	root := &cobra.Command{
		Use:   "fireball",
		Short: "An all-in-one binary tooling for the Fireball language",
	}

	root.AddCommand(getBuildCmd())
	root.AddCommand(getRunCmd())
	root.AddCommand(getTestCmd())
	root.AddCommand(getLspCmd())

	if err := root.Execute(); err != nil {
		scope()
		os.Exit(1)
	}

	scope()
	core.EndProfiler()
}
