package build

import (
	"bytes"
	"fireball/ast"
	"fireball/codegen"
	"fireball/core"
	"fireball/ir"
	"fireball/ir/llvm"
	"fireball/project"
	"fireball/toolchain"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type EntrypointFn func(module *ir.Module, fun *ir.Function) *project.Project

type System struct {
	toolchain toolchain.Toolchain
	target    toolchain.Target
	profile   project.Profile

	profilePath string

	linkLibC bool

	libPaths []string
	libs     []string

	mainProjName  string
	mainBuildPath string
}

func Init(path string, profile project.Profile) (*System, error) {
	defer core.Scope()()

	tc, err := toolchain.Validate()
	if err != nil {
		return nil, err
	}

	target, err := toolchain.GetTarget()
	if err != nil {
		return nil, err
	}

	return &System{
		toolchain:   tc,
		target:      target,
		profile:     profile,
		profilePath: filepath.Join(path, "build", target.Name, profile.Name),
		linkLibC:    false,
	}, nil
}

func (s *System) CompileProjectHierarchy(projMap map[string]*project.Project) ([]string, error) {
	defer core.Scope()()

	split := strings.Split(s.target.Name, "-")

	replacer := project.GetReplacer(project.Variables{
		Os:   split[0],
		Arch: split[1],
	})

	var files []*project.File
	var objFilePaths []string

	for _, proj := range projMap {
		for _, file := range proj.Files {
			files = append(files, file)
			objFilePaths = append(objFilePaths, "")
		}

		if proj.Config.LibC {
			s.linkLibC = true
		}

		for _, path := range proj.Config.LibPaths {
			s.libPaths = append(s.libPaths, filepath.Join(proj.Path, replacer.Replace(path)))
		}

		for _, lib := range proj.Config.Libs {
			s.libs = append(s.libs, replacer.Replace(lib))
		}
	}

	fileDataMap := make(map[*ast.File]codegen.FileData, len(files))

	for _, file := range files {
		fileDataMap[file.Ast] = codegen.FileData{
			ExprInfos: file.ExprInfos,
			NodeTypes: file.NodeTypes,
		}
	}

	builtins := project.GetBuiltins(projMap)

	err := core.ParallelFor(files, func(i int, file *project.File) error {
		buildPath := filepath.Join(s.profilePath, file.Proj.Config.Name)

		objPath := filepath.Join(buildPath, "obj")
		if err := os.MkdirAll(objPath, 0750); err != nil {
			return err
		}

		irPath := ""
		if s.profile.OutputIr {
			irPath = filepath.Join(buildPath, "ir")
			if err := os.MkdirAll(irPath, 0750); err != nil {
				return err
			}
		}

		name := getBuildFileName(file)
		module := codegen.Generate(file.Ast, s.target.Arch, s.target.CallConv, file.Instantiations, file.TypeEnv, fileDataMap, builtins, file.Path, s.profile.Lto)

		if module.IsEmpty() {
			return nil
		}

		objFilePath, err := s.compileModule(objPath, irPath, name, module)
		if err != nil {
			return err
		}

		objFilePaths[i] = objFilePath
		return nil
	})

	return objFilePaths, err
}

func (s *System) CompileEntrypoint(fn EntrypointFn) (string, error) {
	defer core.Scope()()

	// Create module
	module := ir.NewModule()
	module.Path = "__entrypoint"

	if strings.Contains(s.target.Name, "windows") {
		// Apparently adding a summary node for '_fltused' causes LTO to remove the global,
		// even if it is marked as "live" and referenced by the entrypoint function.
		// :shrug:
		fltused := module.NewGlobalVar("_fltused", ir.I32)
		fltused.Initializer = &ir.Integer{Typ: ir.I32, Value: core.Signed(1)}
	}

	name := "_start"
	if s.linkLibC {
		name = "main"
	}

	fun := module.NewFunction(name, &ir.Signature{Returns: ir.I32}, nil)
	fun.Flags = ir.DsoLocal

	proj := fn(module, fun)

	// Get paths
	s.mainProjName = proj.Config.Name
	s.mainBuildPath = filepath.Join(s.profilePath, proj.Config.Name)

	mainObjPath := filepath.Join(s.mainBuildPath, "obj")

	mainIrPath := ""
	if s.profile.OutputIr {
		mainIrPath = filepath.Join(s.mainBuildPath, "ir")
	}

	// Compile
	objFilePath, err := s.compileModule(mainObjPath, mainIrPath, "__entrypoint", module)
	if err != nil {
		return "", err
	}

	return objFilePath, nil
}

func (s *System) Link(objFilePaths []string) (string, error) {
	defer core.Scope()()

	if s.mainProjName == "" {
		panic("build.System.Link() - Cannot link a binary without an entrypoint")
	}

	exeFilePath := filepath.Join(s.mainBuildPath, s.mainProjName+s.target.ExecutableFileExtension)

	objFilePaths = slices.DeleteFunc(objFilePaths, func(p string) bool { return p == "" })

	var libc *toolchain.LibC

	if s.linkLibC {
		lib, err := toolchain.FindLibC()
		if err != nil {
			return "", err
		}

		libc = new(lib)
	}

	if err := toolchain.Link(s.toolchain, objFilePaths, exeFilePath, s.profile.Opt, s.target, libc, s.libPaths, s.libs); err != nil {
		return "", err
	}

	return exeFilePath, nil
}

func (s *System) compileModule(objPath, irPath string, name string, module *ir.Module) (string, error) {
	defer core.Scope()()

	module.DataLayout = s.target.DataLayout
	module.Triple = s.target.Triple

	codegen.AddModuleMetaFlags(module, s.profile.Lto)

	// Get IR reader
	var irReader io.Reader

	defer func() {
		if c, ok := irReader.(io.Closer); ok {
			_ = c.Close()
		}
	}()

	if s.profile.OutputIr {
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
	if s.profile.Lto {
		bcFilePath := filepath.Join(objPath, name+".bc")

		if err := toolchain.Assemble(s.toolchain, irReader, bcFilePath); err != nil {
			return "", err
		}

		return bcFilePath, nil
	}

	// Compile to object file
	objFilePath := filepath.Join(objPath, name+s.target.ObjectFileExtension)

	if err := toolchain.Compile(s.toolchain, irReader, objFilePath, s.profile.Opt); err != nil {
		return "", err
	}

	return objFilePath, nil
}
