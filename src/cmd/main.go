package main

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/cmd/lsp"
	"fireball/codegen"
	"fireball/llvm"
	"fireball/project"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
)

func main() {
	root := cobra.Command{
		Use:     "fireball",
		Short:   "Tooling for the Fireball programming language",
		Version: "0.1.0",
	}

	root.AddCommand(
		buildCommand(),
		runCommand(),
		testCommand(),
		lsp.Command(),
	)

	if err := root.Execute(); err != nil {
		log.Fatalln(err.Error())
	}
}

func buildCommand() *cobra.Command {
	opt := uint8(0)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := buildExe(opt)
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
			path, err := buildExe(opt)
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

func buildExe(opt uint8) (string, error) {
	return build(".", "", opt, func(proj *project.Project) *llvm.Module {
		m := llvm.NewModule("", "", "")
		types := codegen.TypeCache{Arch: abi.AMD64, Module: m}

		mainType := types.Get(&ast.SimpleFuncType{Returns: &ast.PrimitiveType{Kind: ast.I32}})
		fbMain := m.NewExternFunction("fb$main", mainType)

		main := m.NewFunction("main", "main", mainType, nil)
		main.Block(llvm.NamedIdentifier("entry"))

		ret := llvm.Call(main, fbMain, "").End()
		llvm.RetValue(main, ret)
		main.End()

		return m
	})
}
