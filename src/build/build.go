package build

import (
	"bytes"
	"fireball/codegen"
	"fireball/core"
	"fireball/ir"
	"fireball/ir/llvm"
	"fireball/project"
	"fireball/toolchain"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type EntrypointFn func(module *ir.Module, fun *ir.Function)

func Build(main *project.Project, projMap map[string]*project.Project, target toolchain.Target, profile project.Profile, entrypointFn EntrypointFn) (string, error) {
	defer core.Scope()()

	// Get paths
	profilePath := filepath.Join(main.Path, "build", target.Name, profile.Name)

	// Compile files
	var files []*project.File
	var objFilePaths []string

	linkLibC := false

	for _, proj := range projMap {
		for _, file := range proj.Files {
			files = append(files, file)
			objFilePaths = append(objFilePaths, "")
		}

		if proj.Config.LibC {
			linkLibC = true
		}
	}

	err := core.ParallelFor(files, func(i int, file *project.File) error {
		buildPath := filepath.Join(profilePath, file.Proj.Config.Name)

		objPath := filepath.Join(buildPath, "obj")
		if err := os.MkdirAll(objPath, 0750); err != nil {
			return err
		}

		irPath := ""
		if profile.OutputIr {
			irPath = filepath.Join(buildPath, "ir")
			if err := os.MkdirAll(irPath, 0750); err != nil {
				return err
			}
		}

		name := getBuildFileName(file)
		module := codegen.Generate(file.Ast, target.Arch, target.CallConv, file.ExprInfos, file.NodeTypes, file.Path, profile.Lto)

		objFilePath, err := compileModule(target, profile, objPath, irPath, name, module)
		if err != nil {
			return err
		}

		objFilePaths[i] = objFilePath
		return nil
	})
	if err != nil {
		return "", err
	}

	// Entrypoint
	mainBuildPath := filepath.Join(profilePath, main.Config.Name)
	mainObjPath := filepath.Join(mainBuildPath, "obj")

	mainIrPath := ""
	if profile.OutputIr {
		mainIrPath = filepath.Join(mainBuildPath, "ir")
	}

	{
		// Create module
		module := ir.NewModule()
		module.Path = "__entrypoint"

		if strings.Contains(target.Name, "windows") {
			fltused := module.NewGlobalVar("_fltused", ir.I32)
			fltused.Initializer = &ir.Integer{Typ: ir.I32, Value: core.Signed(1)}
		}

		name := "_start"
		if linkLibC {
			name = "main"
		}

		fun := module.NewFunction(name, &ir.Signature{Returns: ir.I32}, nil)
		fun.Flags = ir.DsoLocal

		entrypointFn(module, fun)

		// Compile
		objFilePath, err := compileModule(target, profile, mainObjPath, mainIrPath, "__entrypoint", module)
		if err != nil {
			return "", err
		}

		objFilePaths = append(objFilePaths, objFilePath)
	}

	// Link executable
	exeFilePath := filepath.Join(mainBuildPath, main.Config.Name+target.ExecutableFileExtension)

	var libc *toolchain.LibC

	if linkLibC {
		lib, err := toolchain.FindLibC()
		if err != nil {
			return "", err
		}

		libc = new(lib)
	}

	if err := toolchain.Link(objFilePaths, exeFilePath, profile.Opt, target, libc); err != nil {
		return "", err
	}

	return exeFilePath, nil
}

func compileModule(target toolchain.Target, profile project.Profile, objPath, irPath string, name string, module *ir.Module) (string, error) {
	defer core.Scope()()

	module.DataLayout = target.DataLayout
	module.Triple = target.Triple

	codegen.AddModuleMetaFlags(module, profile.Lto)

	// Get IR reader
	var irReader io.Reader

	defer func() {
		if c, ok := irReader.(io.Closer); ok {
			_ = c.Close()
		}
	}()

	if profile.OutputIr {
		// Write into .ll file
		irFilePath := filepath.Join(irPath, name+".ll")

		irFile, err := os.Create(irFilePath)
		if err != nil {
			return "", err
		}

		if err := llvm.Write(module, irFile); err != nil {
			_ = irFile.Close()
			return "", err
		}

		_ = irFile.Close()

		irReader, err = os.Open(irFilePath)
		if err != nil {
			return "", err
		}
	} else {
		// Write into memory
		buffer := &bytes.Buffer{}

		if err := llvm.Write(module, buffer); err != nil {
			return "", err
		}

		irReader = buffer
	}

	// Assemble to bitcode file
	if profile.Lto {
		bcFilePath := filepath.Join(objPath, name+".bc")

		if err := toolchain.Assemble(irReader, bcFilePath); err != nil {
			return "", err
		}

		return bcFilePath, nil
	}

	// Compile to object file
	objFilePath := filepath.Join(objPath, name+target.ObjectFileExtension)

	if err := toolchain.Compile(irReader, objFilePath, profile.Opt); err != nil {
		return "", err
	}

	return objFilePath, nil
}
