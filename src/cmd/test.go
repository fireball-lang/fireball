package main

import (
	"errors"
	"fireball/ast"
	"fireball/codegen"
	"fireball/ir"
	"fireball/project"
	"fireball/utils"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"atomicgo.dev/cursor"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var testFunctions []*ast.Func

type testState uint8

const (
	failed testState = iota
	running
	passed
)

type testOutputWriter struct {
	index  int
	failed int
}

func (t *testOutputWriter) Begin() {
	t.index = 0
	writeTestState(testFunctions[0], running)
}

func (t *testOutputWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		cursor.ClearLine()
		cursor.StartOfLine()

		state := utils.Ternary(b == '0', failed, passed)
		writeTestState(testFunctions[t.index], state)

		if state == failed {
			fmt.Println()
			t.failed++
		}

		t.index++
	}

	if t.index < len(testFunctions) {
		cursor.ClearLine()
		cursor.StartOfLine()

		writeTestState(testFunctions[t.index], running)
	}

	return len(p), nil
}

func (t *testOutputWriter) End() {
	cursor.ClearLine()
	cursor.StartOfLine()

	total := len(testFunctions)

	if t.failed == 0 {
		_, _ = color.New(color.FgGreen).Printf("%d tests passed\n", total)
	} else {
		fmt.Println()
		_, _ = color.New(color.FgYellow).Printf("%d out of %d tests failed", t.failed, total)

		os.Exit(1)
	}
}

func writeTestState(fun *ast.Func, state testState) {
	// Module
	mod := ast.Root(fun).ModulePath()
	color.Set(color.FgHiBlack)

	for _, segment := range mod.Segments {
		fmt.Print(segment.Token.Text)
		fmt.Print(":")
	}

	color.Unset()

	// Name
	name := fun.Name()

	if str := fun.GetAttribute("test").Param; str != "" {
		name = str
	}

	_, _ = color.New(color.Underline).Print(name)

	// State
	switch state {
	case failed:
		fmt.Print(color.RedString(" failed"))
	case running:
		fmt.Print(color.YellowString(" running"))
	case passed:
		fmt.Print(color.GreenString(" passed"))
	}
}

func testCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Runs tests in the projects.",
		RunE: func(_ *cobra.Command, args []string) error {
			profile := getProfile(false)
			path, err := buildPath(".", profile, buildTestEntrypoint)

			if err != nil {
				return err
			}
			if path == "<errors>" {
				return nil
			}

			out := testOutputWriter{}
			out.Begin()

			cmd := exec.Command(path)
			cmd.Stdout = &out

			if err := cmd.Run(); err != nil {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) {
					return err
				}
			}

			out.End()
			return nil
		},
	}

	return cmd
}

func buildTestEntrypoint(proj *project.Project, m *ir.Module, main *ir.Function) string {
	testTyp := &ir.FunctionType{Returns: ir.I8}
	var tests []*ir.Function

	for file := range proj.Files() {
		for _, decl := range file.Ast().Decls {
			if f, ok := decl.(*ast.Func); ok && f.GetAttribute("test") != nil {
				testFunctions = append(testFunctions, f)

				test := m.NewFunction(codegen.GetFuncLinkName(f), testTyp, nil)
				test.Flags = ir.Declare | ir.DsoLocal

				tests = append(tests, test)
			}
		}
	}

	putsTyp := &ir.FunctionType{Returns: ir.I32, Params: []ir.Type{ir.I32}}
	puts := m.NewFunction("putchar", putsTyp, []string{"char"})
	puts.Flags = ir.Declare

	var stdoutVar *ir.GlobalVar
	var flush *ir.Function

	if runtime.GOOS != "windows" {
		stdoutVar = m.NewGlobalVar("stdout", ir.Pointer)
		stdoutVar.Flags = ir.External

		flushTyp := &ir.FunctionType{Returns: ir.I32, Params: []ir.Type{ir.Pointer}}
		flush = m.NewFunction("fflush", flushTyp, []string{"file"})
		flush.Flags = ir.Declare
	}

	emitter := ir.Emitter{Module: m}
	emitter.Begin(main.NewBlock("func.entry"))

	var stdout ir.Value

	if runtime.GOOS != "windows" {
		stdout = emitter.Load(ir.Pointer, stdoutVar)
	}

	counter := i32Value(0)

	for _, test := range tests {
		okI8 := emitter.Call(testTyp, test, nil)
		okI1 := emitter.Trunc(okI8, ir.I1)

		v := emitter.Select(okI1, i32Value('1'), i32Value('0'))
		emitter.Call(putsTyp, puts, []ir.Value{v})

		if runtime.GOOS != "windows" {
			emitter.Call(flush.Typ, flush, []ir.Value{stdout})
		}

		okI32 := emitter.Ext(ir.Unsigned, okI1, ir.I32)
		counter = emitter.Add(counter, okI32)
	}

	emitter.Ret(counter)

	return "test"
}

func i32Value(value int64) ir.Value {
	return &ir.Integer{
		Typ:   ir.I32,
		Value: utils.Signed(value),
	}
}
