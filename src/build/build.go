package build

import (
	"bytes"
	"fireball/codegen"
	"fireball/ir"
	"fireball/ir/llvm"
	"fireball/project"
	"fireball/toolchain"
	"io"
	"os"
	"path/filepath"
)

type EntrypointFn func(module *ir.Module, fun *ir.Function)

func Build(proj *project.Project, target toolchain.Target, profile project.Profile, entrypointFn EntrypointFn) (string, error) {
	// Get paths
	buildPath := filepath.Join(proj.Path, "build", target.Name, profile.Name)
	if err := os.MkdirAll(buildPath, 0750); err != nil {
		return "", err
	}

	objPath := filepath.Join(buildPath, "obj")
	if err := os.MkdirAll(objPath, 0750); err != nil {
		return "", err
	}

	irPath := ""
	if profile.OutputIr {
		irPath = filepath.Join(buildPath, "ir")
		if err := os.MkdirAll(irPath, 0750); err != nil {
			return "", err
		}
	}

	// Compile files
	var objFilePaths []string

	for _, file := range proj.Files {
		name := getBuildFileName(file)
		module := codegen.Generate(file.Decls, target.Arch, target.CallConv, file.ExprInfos, file.NodeTypes)
		module.Path = file.Path

		objFilePath, err := compileModule(target, profile, objPath, irPath, name, module)
		if err != nil {
			return "", err
		}

		objFilePaths = append(objFilePaths, objFilePath)
	}

	// Entrypoint
	{
		// Create module
		module := ir.NewModule()

		name := "_start"
		if proj.Config.LibC {
			name = "main"
		}

		fun := module.NewFunction(name, &ir.Signature{Returns: ir.I32}, nil)
		fun.Flags = ir.DsoLocal

		entrypointFn(module, fun)

		// Compile
		objFilePath, err := compileModule(target, profile, objPath, irPath, "__entrypoint", module)
		if err != nil {
			return "", err
		}

		objFilePaths = append(objFilePaths, objFilePath)
	}

	// Link executable
	exeFilePath := filepath.Join(buildPath, proj.Config.Name+target.ExecutableFileExtension)

	var libc *toolchain.LibC

	if proj.Config.LibC {
		lib, err := toolchain.FindLibC()
		if err != nil {
			return "", err
		}

		libc = new(lib)
	}

	if err := toolchain.Link(objFilePaths, exeFilePath, profile.Opt, libc); err != nil {
		return "", err
	}

	return exeFilePath, nil
}

func compileModule(target toolchain.Target, profile project.Profile, objPath, irPath string, name string, module *ir.Module) (string, error) {
	module.DataLayout = target.DataLayout
	module.Triple = target.Triple

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
