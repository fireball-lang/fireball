package project

import (
	"fireball/ast"
	"fireball/core"
	"fireball/parser"
	"fireball/sema"
	"fireball/symbols"
	"fireball/types"
	"io"
	"iter"
	"os"
)

type File struct {
	Proj *Project
	Path string

	Source Source
	Data   any

	Ast              *ast.File
	parseDiagnostics []core.Diagnostic

	Symbols []symbols.Symbol

	resolveDiagnostics []core.Diagnostic

	ExprInfos       map[ast.Expr]sema.ExprInfo
	NodeTypes       map[ast.Node]types.Type
	Instantiations  types.InstantiationCache
	TypeEnv         *sema.TypeEnvironment
	semaDiagnostics []core.Diagnostic
}

type Source interface {
	Get() io.ReadCloser
}

func newFile(proj *Project, path string) *File {
	return &File{
		Proj:   proj,
		Path:   path,
		Source: &fileSource{path: path},
	}
}

func (f *File) parse() {
	reader := f.Source.Get()

	//goland:noinspection GoUnhandledErrorResult
	defer reader.Close()

	f.Ast, f.parseDiagnostics = parser.Parse(reader, f.Path)
	f.Symbols = symbols.Collect(f.Ast)
}

func (f *File) resolve(root symbols.Scope, instantiations types.InstantiationCache, typeEnv *sema.TypeEnvironment) {
	f.Instantiations = instantiations
	f.TypeEnv = typeEnv
	f.NodeTypes, f.resolveDiagnostics = sema.ResolveSymbols(f.Ast, f.Symbols, instantiations, typeEnv, root, f.Path)
}

func (f *File) analyze(root symbols.Scope, instantiations types.InstantiationCache, typeEnv *sema.TypeEnvironment) {
	f.ExprInfos, f.semaDiagnostics = sema.Analyze(f.Ast, f.Symbols, root, instantiations, typeEnv, f.NodeTypes, f.Proj.Config.Name, f.Path)
}

func (f *File) Diagnostics() iter.Seq[core.Diagnostic] {
	return func(yield func(core.Diagnostic) bool) {
		for _, diagnostic := range f.parseDiagnostics {
			if !yield(diagnostic) {
				return
			}
		}
		for _, diagnostic := range f.resolveDiagnostics {
			if !yield(diagnostic) {
				return
			}
		}
		for _, diagnostic := range f.semaDiagnostics {
			if !yield(diagnostic) {
				return
			}
		}
	}
}

// fileSource

type fileSource struct {
	path string
}

func (f *fileSource) Get() io.ReadCloser {
	file, err := os.Open(f.path)
	if err != nil {
		panic(err)
	}

	return file
}
