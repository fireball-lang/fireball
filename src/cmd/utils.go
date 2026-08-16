package main

import (
	"errors"
	"fireball/ast"
	"fireball/build"
	"fireball/cfg"
	"fireball/codegen"
	"fireball/core"
	"fireball/ir"
	"fireball/project"
	"fireball/sema"
	"fireball/types"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
)

type TargetOsValue struct {
	Value ast.TargetOsKind
}

func (t *TargetOsValue) String() string {
	switch t.Value {
	case ast.WindowsOs:
		return "windows"
	case ast.Linux:
		return "linux"
	case ast.MacOS:
		return "macos"

	default:
		panic("cmd.TargetOsValue.String() - Invalid value")
	}
}

func (t *TargetOsValue) Set(s string) error {
	switch s {
	case "windows":
		t.Value = ast.WindowsOs
		return nil
	case "linux":
		t.Value = ast.Linux
		return nil
	case "macos":
		t.Value = ast.MacOS
		return nil

	default:
		return fmt.Errorf("expected 'windows', 'linux' or 'macos'")
	}
}

func (t *TargetOsValue) Type() string {
	return "TargetOs"
}

func parseProject(env cfg.Env, start *time.Time) (*project.Project, map[string]*project.Project, error) {
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

	// Order projects by dependency (dependencies before dependents)
	ordered := project.OrderProjects(projMap, depMap)

	// Parse
	for _, proj := range ordered {
		proj.Parse(nil, env)
	}

	for _, proj := range ordered {
		proj.Module.CheckCollisions()
	}

	// Resolve
	builtins := project.GetBuiltins(projMap)
	instantiations := types.NewInstantiationCache()
	typeEnv := sema.NewTypeEnvironment(instantiations, builtins)

	for _, proj := range ordered {
		proj.Resolve(depMap, instantiations, typeEnv, builtins)
	}

	instantiations.SubstituteDependentTypes()

	// Check type-local collisions
	project.CheckTypeLocalCollisions(projMap, typeEnv)

	// Analyze
	for _, proj := range ordered {
		proj.Analyze(depMap, instantiations, typeEnv, builtins)
	}

	// Print diagnostics
	hasDiagnostics := false
	hasErrors := false

	for _, proj := range projMap {
		for _, file := range proj.Files {
			path, err := filepath.Rel(proj.Path, file.Path)
			if err != nil {
				panic(err)
			}

			for diag := range file.Diagnostics() {
				printDiagnostic(path, diag)

				hasDiagnostics = true

				if diag.Kind == core.Error {
					hasErrors = true
				}
			}
		}
	}

	if hasDiagnostics {
		fmt.Println()
	}

	if hasErrors {
		if start != nil {
			duration := time.Since(*start)

			_, _ = color.New(color.FgRed, color.Bold).Print("Build failed\n")
			color.White("  took %s", duration)
		}

		return nil, nil, nil
	}

	return main, projMap, nil
}

func buildProject(proj *project.Project, projMap map[string]*project.Project, profileName string, start time.Time, entrypointFnProvider func(projMap map[string]*project.Project, project2 *project.Project) (build.EntrypointFn, error)) (string, error) {
	defer core.Scope()()

	// Initialize build system
	profile, ok := proj.Config.Profiles[profileName]
	if !ok {
		return "", fmt.Errorf("unknown profile: '%s'", profileName)
	}

	system, err := build.Init(proj.Path, profile)
	if err != nil {
		return "", err
	}

	// Compile project
	objFilePaths, err := system.CompileProjectHierarchy(projMap)
	if err != nil {
		return "", err
	}

	if entrypointFnProvider == nil {
		// Print info
		printBuildSuccessful(start)
		return "", nil
	}

	// Compile entrypoint
	fn, err := entrypointFnProvider(projMap, proj)
	if err != nil {
		return "", err
	}

	entrypointObjFilePath, err := system.CompileEntrypoint(fn)
	if err != nil {
		return "", err
	}

	objFilePaths = append(objFilePaths, entrypointObjFilePath)

	// Link executable
	executablePath, err := system.Link(objFilePaths)
	if err != nil {
		return "", err
	}

	// Print info
	printBuildSuccessful(start)
	return executablePath, nil
}

func printBuildSuccessful(start time.Time) {
	duration := time.Since(start)

	_, _ = color.New(color.FgGreen, color.Bold).Print("Build successful\n")
	color.White("  took %s", duration)
}

func normalEntrypointProvider(projMap map[string]*project.Project, proj *project.Project) (build.EntrypointFn, error) {
	var mainFunc *ast.Func
	var mainFuncTyp *types.Func

outer:
	for _, file := range proj.Files {
		for _, decl := range file.Ast.Decls {
			if f, ok := decl.(*ast.Func); ok && f.Name().Token.Text == "main" && len(f.Params) == 0 && !f.VarArgs {
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

	return func(module *ir.Module, fun *ir.Function) *project.Project {
		typeCache := codegen.TypeCache{Module: module}

		mainFun := module.NewFunction(codegen.FuncLinkName(mainFunc, mainFuncTyp, nil), &ir.Signature{Returns: typeCache.Get(mainFuncTyp.Returns)}, nil)
		mainFun.Flags = ir.Declare

		emitter := ir.Emitter{Module: module}
		emitter.Begin(fun.NewBlock("fun.entry"))

		// Init functions
		inits := findInitFunctions(projMap)
		initSignature := &ir.Signature{Returns: ir.Void}

		for _, init := range inits {
			initFun := module.NewFunction(codegen.FuncLinkName(init.node, init.typ, nil), initSignature, nil)
			initFun.Flags = ir.Declare

			emitter.Call(initSignature, initFun, nil)
		}

		// Fireball main
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

		summaryCalls := make([]ir.FunctionSummaryCall, 0, 1+len(inits))

		fbMainRef := module.AddSummary(&ir.SymbolSummary{Name: mainFun.Name})
		summaryCalls = append(summaryCalls, ir.FunctionSummaryCall{Callee: fbMainRef})

		for _, init := range inits {
			initRef := module.AddSummary(&ir.SymbolSummary{Name: codegen.FuncLinkName(init.node, init.typ, nil)})
			summaryCalls = append(summaryCalls, ir.FunctionSummaryCall{Callee: initRef})
		}

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
			Calls:            summaryCalls,
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

		return proj
	}, nil
}

type initFunc struct {
	node *ast.Func
	typ  *types.Func
}

func findInitFunctions(projMap map[string]*project.Project) []initFunc {
	var inits []initFunc

	for _, proj := range projMap {
		for _, file := range proj.Files {
			for _, decl := range file.Ast.Decls {
				if f, ok := decl.(*ast.Func); ok && ast.GetAttribute[*ast.Init](f) != nil {
					inits = append(inits, initFunc{
						node: f,
						typ:  file.NodeTypes[f].(*types.Func),
					})
				}
			}
		}
	}

	return inits
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
