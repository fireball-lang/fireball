package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "fireball",
		Short: "An all-in-one binary tooling for the Fireball language",
	}

	root.AddCommand(getBuildCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
