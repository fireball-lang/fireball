package main

import (
	"errors"
	"fireball/ast"
	"fireball/build"
	"fireball/codegen"
	"fireball/core"
	"fireball/ir"
	"fireball/project"
	"fireball/toolchain"
	"fireball/types"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fatih/color"
)

func buildProject(proj *project.Project, profileName string, start time.Time, entrypointFnProvider func(project2 *project.Project) (build.EntrypointFn, error)) (string, error) {
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

	exePath, err := build.Build(proj, target, profile, entrypointFn)
	if err != nil {
		return "", err
	}

	// Print
	duration := time.Since(start)

	_, _ = color.New(color.FgGreen, color.Bold).Print("Build succeeded\n")
	color.White("  took %s", duration)

	return exePath, nil
}

func parseProject(start time.Time) (*project.Project, error) {
	proj, err := project.Open(".")
	if err != nil {
		return nil, err
	}

	// Parse
	proj.Parse()

	hasErrors := false

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

	if hasErrors {
		duration := time.Since(start)

		fmt.Println()
		_, _ = color.New(color.FgRed, color.Bold).Print("Build failed\n")
		color.White("  took %s", duration)

		return nil, nil
	}

	return proj, nil
}

func normalEntrypointProvider(proj *project.Project) (build.EntrypointFn, error) {
	var mainFunc *ast.Func
	var mainFuncTyp *types.Func

outer:
	for _, file := range proj.Files {
		for _, decl := range file.Decls {
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
