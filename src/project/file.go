package project

import (
	"fireball/ast"
	"fireball/cfg"
	"fireball/core"
	"fireball/parser"
	"fireball/sema"
	"fireball/symbols"
	"fireball/types"
	"io"
	"iter"
	"os"
	"unicode/utf8"
)

type File struct {
	Proj *Project
	Path string

	Source Source
	Data   any

	LineTable        []uint32
	Ast              *ast.File
	parseDiagnostics []core.Diagnostic

	Symbols []symbols.Symbol

	resolveDiagnostics []core.Diagnostic

	ExprInfos       map[ast.Expr]sema.ExprInfo
	NodeTypes       map[ast.Node]types.Type
	Instantiations  *types.InstantiationCache
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

func (f *File) parse(env cfg.Env) {
	readCloser := f.Source.Get()

	//goland:noinspection GoUnhandledErrorResult
	defer readCloser.Close()

	var builder lineTableBuilder
	reader := io.TeeReader(readCloser, &builder)

	f.Ast, f.parseDiagnostics = parser.Parse(reader, f.Path)
	f.LineTable = builder.Finish()

	env.Strip(f.Ast)

	f.Symbols = symbols.Collect(f.Ast)
}

func (f *File) resolve(root symbols.Scope, instantiations *types.InstantiationCache, typeEnv *sema.TypeEnvironment) {
	f.Instantiations = instantiations
	f.TypeEnv = typeEnv
	f.NodeTypes, f.resolveDiagnostics = sema.Resolve(f.Ast, f.Symbols, instantiations, typeEnv, root, f.Path)
}

func (f *File) analyze(root symbols.Scope, instantiations *types.InstantiationCache, typeEnv *sema.TypeEnvironment) {
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

// lineTableBuilder

type lineTableBuilder struct {
	table   []uint32
	current uint32
}

func (l *lineTableBuilder) Write(p []byte) (n int, err error) {
	n = len(p)

	for len(p) > 0 {
		r, size := utf8.DecodeRune(p)
		p = p[size:]

		if r == '\n' {
			l.table = append(l.table, l.current)
			l.current = 0
		} else {
			l.current++
		}
	}

	return
}

func (l *lineTableBuilder) Finish() []uint32 {
	if l.current > 0 {
		l.table = append(l.table, l.current)
		l.current = 0
	}

	return l.table
}
