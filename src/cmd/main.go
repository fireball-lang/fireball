package main

import (
	"fireball/ast"
	"fireball/cmd/build"
	"fireball/cmd/lsp"
	"fireball/codegen"
	"fireball/ir"
	"fireball/profiler"
	"fireball/project"
	"log"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

const profile = false

func main() {
	if profile {
		file, err := os.Create("profile.json")
		if err != nil {
			log.Fatalln(err.Error())
		}

		//goland:noinspection GoUnhandledErrorResult
		defer file.Close()

		stop, err := profiler.Start(file)
		if err != nil {
			log.Fatalln(err.Error())
		}

		//goland:noinspection GoUnhandledErrorResult
		defer stop()
	}

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
	release := false

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile := getProfile(release)
			_, err := buildPath(".", profile, buildExecutableEntrypoint)

			return err
		},
	}

	cmd.Flags().BoolVarP(&release, "release", "r", false, "Build project using using the release profile.")

	return cmd
}

func runCommand() *cobra.Command {
	release := false

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the project.",
		RunE: func(_ *cobra.Command, args []string) error {
			profile := getProfile(release)
			path, err := buildPath(".", profile, buildExecutableEntrypoint)

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

	cmd.Flags().BoolVarP(&release, "release", "r", false, "Build project using using the release profile.")

	return cmd
}

func getProfile(release bool) build.Profile {
	if release {
		return build.Profile{
			Name: "release",
			Opt:  2,
			Lto:  build.Thin,
		}
	}

	return build.Profile{
		Name: "debug",
		Opt:  0,
		Lto:  build.None,
	}
}

func buildExecutableEntrypoint(proj *project.Project, m *ir.Module, main *ir.Function, summary bool) string {
	mainType := &ast.SimpleFuncType{Returns: &ast.PrimitiveType{Kind: ast.I32}}
	mainTyp := &ir.FunctionType{Returns: ir.I32}

	fbMainFunc := getMainFunc(proj, mainType)
	fbMain := m.NewFunction(codegen.GetFuncLinkName(fbMainFunc), mainTyp, nil)
	fbMain.Flags = ir.Declare | ir.DsoLocal

	emitter := ir.Emitter{Module: m}
	emitter.Begin(main.NewBlock("func.entry"))
	emitter.Ret(emitter.Call(mainTyp, fbMain, nil))

	if summary {
		moduleRef := m.AddSummary(&ir.ModuleSummary{
			Path: "[Regular LTO]",
			Hash: [5]uint32{},
		})

		fbMainRef := m.AddSummary(&ir.SymbolSummary{Name: fbMain.Name})

		m.AddSummary(&ir.FunctionSummary{
			Module: moduleRef,
			Name:   main.Name,
			LinkFlags: ir.LinkSummaryFlags{
				Linkage:             ir.LinkageExternal,
				Visibility:          ir.VisibilityDefault,
				NotEligibleToImport: false,
				Live:                false,
				DsoLocal:            true,
				CanAutoHide:         false,
				ImportType:          ir.ImportDefinition,
			},
			InstructionCount: emitter.Block().InstructionCount,
			Flags:            ir.FuncNoInline | ir.FuncNoUnwind,
			Calls:            []ir.FunctionSummaryCall{{Callee: fbMainRef}},
			Refs:             nil,
		})

		m.AddSummary(&ir.SimpleSummary{
			Name:  "flags",
			Value: 520,
		})

		m.AddSummary(&ir.SimpleSummary{
			Name:  "blockcount",
			Value: 0,
		})
	}

	return ""
}

func getMainFunc(proj *project.Project, type_ ast.FuncType) *ast.Func {
	for file := range proj.Files() {
		for _, decl := range file.Ast().Decls {
			if f, ok := decl.(*ast.Func); ok && f.Name() == "main" && f.Equals(type_) {
				return f
			}
		}
	}

	panic("no main function")
}
