package main

import (
	"errors"
	"fireball/ast"
	"fireball/codegen"
	"fireball/llvm"
	"fireball/project"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os/exec"
	"strings"
)

var testFunctions []*ast.Func

func testCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Runs tests in the projects.",
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := build(".", "test", 0, createTestModule)
			if err != nil {
				return err
			}
			if path == "<errors>" {
				return nil
			}

			var buffer strings.Builder

			cmd := exec.Command(path)
			cmd.Stdout = &buffer

			if err := cmd.Run(); err != nil {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) {
					return err
				}
			}

			displayTestResults(buffer.String())
			return nil
		},
	}

	return cmd
}

func displayTestResults(result string) {
	failed := 0
	total := 0

	nameStyle := color.New(color.Underline)

	for i, ch := range result {
		name := testFunctions[i].Name()

		if str := testFunctions[i].GetAttribute("test").Param; str != "" {
			name = str
		}

		if ch == '0' {
			_, _ = nameStyle.Print(name)
			color.Red(" failed")

			failed++
		}

		total++
	}

	if failed == 0 {
		_, _ = color.New(color.FgGreen).Printf("%d tests passed\n", total)
	} else {
		fmt.Println()
		_, _ = color.New(color.FgYellow).Printf("%d out of %d tests failed", failed, total)
	}
}

func createTestModule(proj *project.Project) *llvm.Module {
	m := llvm.NewModule("", "", "")

	testType := m.NewFunctionType(m.NewIntegerType(false, 1), nil, false)

	var tests []*llvm.ExternFunction

	for file := range proj.Files() {
		for _, decl := range file.Ast().Decls {
			if f, ok := decl.(*ast.Func); ok && f.GetAttribute("test") != nil {
				testFunctions = append(testFunctions, f)

				tests = append(tests, m.NewExternFunction(codegen.GetLinkName(f), testType))
			}
		}
	}

	i32 := m.NewIntegerType(true, 32)
	puts := m.NewExternFunction("putchar", m.NewFunctionType(i32, []llvm.Type{i32}, false))

	main := m.NewFunction("main", "main", m.NewFunctionType(i32, nil, false), nil)
	main.Block(llvm.NamedIdentifier("entry"))

	var counter llvm.Value = llvm.Int(i32, 0)

	for _, test := range tests {
		ok := llvm.Call(main, test, "").End()
		v := llvm.Select(main, ok, llvm.Int(i32, '1'), llvm.Int(i32, '0'), "")

		call := llvm.Call(main, puts, "")
		llvm.Arg(&call, v)
		call.End()

		oki32 := llvm.Ext(main, ok, i32, "")
		counter = llvm.Add(main, counter, oki32, "")
	}

	llvm.RetValue(main, counter)
	main.End()

	return m
}
