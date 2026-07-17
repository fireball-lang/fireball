package main

import (
	"fireball/cfg"

	"github.com/spf13/cobra"
)

func getCheckCmd() *cobra.Command {
	targetOs := TargetOsValue{Value: cfg.GetHost().TargetOs}

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Checks the project for errors",
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			env := cfg.GetHost()
			env.TargetOs = targetOs.Value
			env.ComputeDerived()

			_, _, err := parseProject(env, nil)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.Flags().VarP(&targetOs, "target", "t", "override the target OS")

	return cmd
}
