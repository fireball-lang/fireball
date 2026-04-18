package main

import (
	"errors"
	"fireball/ast"
	"fireball/build"
	"fireball/codegen"
	"fireball/core"
	"fireball/ir"
	"fireball/project"
	"fireball/symbols"
	"fireball/toolchain"
	"fireball/types"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
)

func buildProject(proj *project.Project, projMap map[string]*project.Project, profileName string, start time.Time, entrypointFnProvider func(project2 *project.Project) (build.EntrypointFn, error)) (string, error) {
	defer core.Scope()()

	// Build
	entrypointFn, err := entrypointFnProvider(proj)
	if err != nil {
		return "", err
	}

	if err := toolchain.Validate(); err != nil {
		return "", err
	}

	target, err := toolchain.GetTarget()
	if err != nil {
		return "", err
	}

	profile, ok := proj.Config.Profiles[profileName]
	if !ok {
		return "", fmt.Errorf("unknown profile: '%s'", profileName)
	}

	exePath, err := build.Build(proj, projMap, target, profile, entrypointFn)
	if err != nil {
		return "", err
	}

	// Print
	duration := time.Since(start)

	_, _ = color.New(color.FgGreen, color.Bold).Print("Build succeeded\n")
	color.White("  took %s", duration)

	return exePath, nil
}

func parseProject(start time.Time) (*project.Project, map[string]*project.Project, error) {
	defer core.Scope()()

	main, err := project.Open(".")
	if err != nil {
		return nil, nil, err
	}

	err = os.MkdirAll(filepath.Join(main.Path, "build"), 0750)
	if err != nil {
		return nil, nil, err
	}

	timings, err := os.Create(filepath.Join(main.Path, "build", "timings.json"))
	if err != nil {
		return nil, nil, err
	}

	core.SetProfilerOutput(timings)

	projMap, depMap, err := project.LoadHierarchy(main)
	if err != nil {
		return nil, nil, err
	}

	// Parse
	for _, proj := range projMap {
		proj.Parse()
	}

	// Resolve
	methodTable := symbols.NewMethodTable()

	for _, proj := range projMap {
		proj.Resolve(depMap, methodTable)
	}

	// Analyze
	for _, proj := range projMap {
		proj.Analyze(depMap, methodTable)
	}

	// Print diagnostics
	hasErrors := false

	for _, proj := range projMap {
		for _, file := range proj.Files {
			path, err := filepath.Rel(proj.Path, file.Path)
			if err != nil {
				panic(err)
			}

			for diag := range file.Diagnostics() {
				printDiagnostic(path, diag)

				if diag.Kind == core.Error {
					hasErrors = true
				}
			}
		}
	}

	if hasErrors {
		duration := time.Since(start)

		fmt.Println()
		_, _ = color.New(color.FgRed, color.Bold).Print("Build failed\n")
		color.White("  took %s", duration)

		return nil, nil, nil
	}

	return main, projMap, nil
}

func normalEntrypointProvider(proj *project.Project) (build.EntrypointFn, error) {
	var mainFunc *ast.Func
	var mainFuncTyp *types.Func

outer:
	for _, file := range proj.Files {
		for _, decl := range file.Ast.Decls {
			if f, ok := decl.(*ast.Func); ok && f.Name() == "main" && len(f.Params) == 0 && !f.VarArgs {
				typ := file.NodeTypes[f].(*types.Func)

				if p, ok := typ.Returns.(*types.Primitive); (ok && types.IsInteger(p.Kind)) || typ.Returns == types.PrimitiveVoid {
					mainFunc = f
					mainFuncTyp = typ
					break outer
				}
			}
		}
	}

	if mainFunc == nil {
		return nil, errors.New("main function not found")
	}

	return func(module *ir.Module, fun *ir.Function) {
		typeCache := codegen.TypeCache{Module: module}

		mainFun := module.NewFunction(codegen.FuncLinkName(mainFunc), &ir.Signature{Returns: typeCache.Get(mainFuncTyp.Returns)}, nil)
		mainFun.Flags = ir.Declare

		emitter := ir.Emitter{Module: module}
		emitter.Begin(fun.NewBlock("fun.entry"))

		var value ir.Value

		if mainFuncTyp.Returns == types.PrimitiveVoid {
			emitter.Call(mainFun.Signature, mainFun, nil)
			value = &ir.Integer{Typ: ir.I32, Value: core.Signed(0)}
		} else {
			value = emitter.Call(mainFun.Signature, mainFun, nil)

			switch mainFuncTyp.Returns.(*types.Primitive).Kind {
			case types.U8, types.U16, types.I8, types.I16:
				value = emitter.Ext(ir.Unsigned, value, ir.I32)
			case types.U32, types.I32:
				// noop
			case types.U64, types.I64:
				value = emitter.Trunc(value, ir.I32)

			default:
				panic("cmd.build.normalEntrypoint() - Invalid main function return type")
			}
		}

		emitter.Ret(value)

		// Summary

		moduleRef := module.AddSummary(&ir.ModuleSummary{
			Path: module.Path,
			Hash: [5]uint32{},
		})

		fbMainRef := module.AddSummary(&ir.SymbolSummary{Name: mainFun.Name})

		module.AddSummary(&ir.FunctionSummary{
			Module: moduleRef,
			Name:   fun.Name,
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

		module.AddSummary(&ir.SimpleSummary{
			Name:  "flags",
			Value: 520,
		})

		module.AddSummary(&ir.SimpleSummary{
			Name:  "blockcount",
			Value: 0,
		})
	}, nil
}

func printDiagnostic(filePath string, diag core.Diagnostic) {
	switch diag.Kind {
	case core.Warning:
		_, _ = color.New(color.FgYellow, color.Bold).Print("warning")
	case core.Error:
		_, _ = color.New(color.FgRed, color.Bold).Print("error")
	}

	_, _ = color.New(color.Bold).Printf(": %s\n", diag.Message)
	_, _ = color.New(color.FgBlue, color.Bold).Print("  --> ")

	fmt.Printf("%s:%d:%d\n", filePath, diag.Range.Start.Line, diag.Range.Start.Column)
}
