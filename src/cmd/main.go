package main

import (
	"errors"
	"fireball/core"
	"os"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
)

func main() {
	core.StartProfiler()
	scope := core.Scope()

	root := &cobra.Command{
		Use:           "fireball",
		Short:         "An all-in-one binary tooling for the Fireball language",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cc.Init(&cc.Config{
		RootCmd:         root,
		Headings:        cc.HiCyan + cc.Bold + cc.Underline,
		Commands:        cc.HiYellow + cc.Bold,
		Example:         cc.Italic,
		ExecName:        cc.Bold,
		Flags:           cc.Bold,
		NoExtraNewlines: true,
	})

	root.AddCommand(getInitCmd())
	root.AddCommand(getNewCmd())
	root.AddCommand(getCheckCmd())
	root.AddCommand(getBuildCmd())
	root.AddCommand(getRunCmd())
	root.AddCommand(getTestCmd())
	root.AddCommand(getLspCmd())

	cmd, err := root.ExecuteC()

	scope()
	core.EndProfiler()

	if err != nil {
		if rtErr, ok := errors.AsType[*runtimeError](err); ok {
			printError(rtErr.err)
		} else {
			printUsageError(cmd, err)
		}

		os.Exit(1)
	}
}
