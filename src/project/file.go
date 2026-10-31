package project

import (
	"fireball/ast"
	"fireball/cfg"
	"fireball/comptime"
	"fireball/core"
	"fireball/ir/eval"
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

	Sema *sema.FileData

	collisionDiagnostics []core.Diagnostic
	resolveDiagnostics   []core.Diagnostic
	semaDiagnostics      []core.Diagnostic
	compTimeDiagnostics  []core.Diagnostic
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

func (f *File) Parse(env cfg.Env) {
	readCloser := f.Source.Get()

	//goland:noinspection GoUnhandledErrorResult
	defer readCloser.Close()

	var builder lineTableBuilder
	reader := io.TeeReader(readCloser, &builder)

	f.Ast, f.parseDiagnostics = parser.Parse(reader, f.Path)
	f.LineTable = builder.Finish()

	env.Strip(f.Ast)

	f.Sema.Symbols = symbols.Collect(f.Ast)
}

func (f *File) resolve(ctx *sema.Context, root symbols.Scope) {
	f.Sema.NodeTypes = make(map[ast.Node]types.Type)
	f.resolveDiagnostics = sema.Resolve(ctx, f.Ast, root, f.Path)
}

func (f *File) analyze(ctx *sema.Context, root symbols.Scope) {
	f.Sema.ExprInfos = make(map[ast.Node]sema.ExprInfo)
	f.semaDiagnostics = sema.Analyze(ctx, f.Ast, root, f.Proj.Config.Name, f.Path)
}

func (f *File) evalCompTime(ctx *sema.Context) {
	f.Sema.Evaluations = make(map[ast.Expr]eval.Value)
	f.compTimeDiagnostics = comptime.Evaluate(ctx, f.Ast)
}

func (f *File) Diagnostics() iter.Seq[core.Diagnostic] {
	return func(yield func(core.Diagnostic) bool) {
		seen := make(map[core.Range]any)

		process := func(diagnostic core.Diagnostic) bool {
			if _, ok := seen[diagnostic.Range]; !ok {
				if !yield(diagnostic) {
					return false
				}
				seen[diagnostic.Range] = nil
			}

			return true
		}

		for _, diagnostic := range f.parseDiagnostics {
			if !process(diagnostic) {
				return
			}
		}
		for _, diagnostic := range f.collisionDiagnostics {
			if !process(diagnostic) {
				return
			}
		}
		for _, diagnostic := range f.resolveDiagnostics {
			if !process(diagnostic) {
				return
			}
		}
		for _, diagnostic := range f.semaDiagnostics {
			if !process(diagnostic) {
				return
			}
		}
		for _, diagnostic := range f.compTimeDiagnostics {
			if !process(diagnostic) {
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
