package main

import (
	"errors"
	"fireball/ast"
	"fireball/build"
	"fireball/codegen"
	"fireball/core"
	"fireball/ir"
	"fireball/project"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func getTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Tests a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()

			// Parse
			proj, err := parseProject(start)
			if err != nil {
				return err
			}
			if proj == nil {
				return nil
			}

			// Get tests
			testFuncs := getTestFuncs(proj)

			// Build
			proj.Config.LibC = true

			exePath, err := buildProject(proj, "debug", start, func(proj *project.Project) (build.EntrypointFn, error) {
				return func(module *ir.Module, fun *ir.Function) {
					testEntrypoint(module, fun, testFuncs)
				}, nil
			})
			if err != nil {
				return err
			}

			fmt.Println()

			// Run
			exeCmd := exec.Command(exePath)

			tw := testWriter{testFuncs: testFuncs}
			exeCmd.Stdout = &tw

			start = time.Now()
			err = exeCmd.Run()
			duration := time.Since(start)

			var exitError *exec.ExitError
			if err != nil && !errors.As(err, &exitError) {
				return err
			}

			if tw.index != len(testFuncs) {
				fmt.Println()

				_, _ = color.New(color.FgRed, color.Bold, color.Underline).Print("CRASHED")
				_, _ = color.New(color.FgWhite, color.Bold).Println("", testFuncs[tw.index].GetTestName())

				return nil
			}

			// Print info
			fg := color.FgGreen

			if tw.successful != len(testFuncs) {
				fmt.Println()
				fg = color.FgYellow
			}

			word := "tests"
			if tw.successful == 1 {
				word = "test"
			}

			_, _ = color.New(fg, color.Bold).Printf("%d / %d %s succeeded\n", tw.successful, len(testFuncs), word)
			color.White("  took %s", duration)

			if tw.successful != len(testFuncs) {
				os.Exit(1)
			}

			return nil
		},
	}

	return cmd
}

type testWriter struct {
	testFuncs  []*ast.Func
	index      int
	successful int
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		f := w.testFuncs[w.index]
		w.index++

		if b == '1' {
			w.successful++
		} else {
			_, _ = color.New(color.FgRed, color.Bold).Print("FAILED ")
			_, _ = color.New(color.FgWhite, color.Bold).Println(f.GetTestName())
		}
	}

	return len(p), nil
}

func getTestFuncs(proj *project.Project) []*ast.Func {
	var tests []*ast.Func

	for _, file := range proj.Files {
		for _, decl := range file.Decls {
			if f, ok := decl.(*ast.Func); ok && f.GetTestName() != "" {
				tests = append(tests, f)
			}
		}
	}

	return tests
}

var irCharOne = &ir.Integer{Typ: ir.I32, Value: core.Unsigned(false, '1')}
var irCharZero = &ir.Integer{Typ: ir.I32, Value: core.Unsigned(false, '0')}

var irOne = &ir.Integer{Typ: ir.I32, Value: core.Unsigned(false, 1)}
var irZero = &ir.Integer{Typ: ir.I32, Value: core.Unsigned(false, 0)}

func testEntrypoint(module *ir.Module, fun *ir.Function, testFuncs []*ast.Func) {
	// Test pointers variable
	tests := make([]ir.Value, len(testFuncs))

	testSignature := &ir.Signature{Returns: ir.I8}

	for i, test := range testFuncs {
		fun := module.NewFunction(codegen.FuncLinkName(test), testSignature, nil)
		fun.Flags = ir.Declare

		tests[i] = fun
	}

	testPointers := module.NewGlobalVar("__test_pointers", &ir.ArrayType{Length: uint32(len(tests)), Element: ir.Pointer})
	testPointers.Flags = ir.Private | ir.UnnamedAddr | ir.Constant
	testPointers.Initializer = &ir.Array{Elements: tests}

	// Lib C
	putchar := module.NewFunction("putchar", &ir.Signature{Returns: ir.I32, Params: []ir.Type{ir.I32}}, []string{"char"})
	putchar.Flags = ir.Declare

	fflush := module.NewFunction("fflush", &ir.Signature{Returns: ir.I32, Params: []ir.Type{ir.Pointer}}, []string{"file"})
	fflush.Flags = ir.Declare

	stdoutPtr := module.NewGlobalVar("stdout", ir.Pointer)
	stdoutPtr.Flags = ir.External

	// Entrypoint
	emitter := ir.Emitter{Module: module}
	emitter.Begin(fun.NewBlock("fun.entry"))

	indexPtr := emitter.Alloca(ir.I32, 1)
	indexPtr.SetName("index")
	emitter.Store(irZero, indexPtr)

	failedPtr := emitter.Alloca(ir.I32, 1)
	failedPtr.SetName("failed")
	emitter.Store(irZero, failedPtr)

	bCondition := fun.NewBlock("condition")
	bBody := fun.NewBlock("body")
	bExit := fun.NewBlock("exit")

	emitter.Br(bCondition)

	// Condition
	{
		emitter.Begin(bCondition)

		index := emitter.Load(ir.I32, indexPtr)
		ok := emitter.ICmp(ir.Lt, false, index, &ir.Integer{Typ: ir.I32, Value: core.Signed(int64(len(tests)))})
		emitter.BrCond(ok, bBody, bExit)
	}

	// Body
	{
		emitter.Begin(bBody)

		index := emitter.Load(ir.I32, indexPtr)

		testPtr := emitter.GetElementPtrDyn(testPointers.Typ, testPointers, irZero, index)
		test := emitter.Load(ir.Pointer, testPtr)

		success := emitter.Call(testSignature, test, nil)
		success = emitter.Trunc(success, ir.I1)

		char := emitter.Select(success, irCharOne, irCharZero)
		emitter.Call(putchar.Signature, putchar, []ir.Value{char})

		stdout := emitter.Load(ir.Pointer, stdoutPtr)
		emitter.Call(fflush.Signature, fflush, []ir.Value{stdout})

		failed := emitter.Load(ir.I32, failedPtr)
		value := emitter.Select(success, irZero, irOne)
		failed = emitter.Add(failed, value)
		emitter.Store(failed, failedPtr)

		index = emitter.Add(index, irOne)
		emitter.Store(index, indexPtr)

		emitter.Br(bCondition)
	}

	// Exit
	{
		emitter.Begin(bExit)

		failed := emitter.Load(ir.I32, failedPtr)
		emitter.Ret(failed)
	}
}
