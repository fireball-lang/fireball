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
	f.Sema.DeprecatedRanges = nil
	f.resolveDiagnostics = sema.Resolve(ctx, f.Ast, root, f.Path)
}

func (f *File) analyze(ctx *sema.Context, root symbols.Scope) {
	f.Sema.ExprInfos = make(map[ast.Node]sema.ExprInfo)
	f.Sema.ArmPatterns = make(map[*ast.Arm][]sema.Pattern)
	f.Sema.StmtExits = make(map[ast.Stmt]sema.Exits)
	f.semaDiagnostics = sema.Analyze(ctx, f.Ast, root, f.Proj.Config.Name, f.Path)
}

func (f *File) Diagnostics() iter.Seq[core.Diagnostic] {
	return func(yield func(core.Diagnostic) bool) {
		groups := [][]core.Diagnostic{
			f.parseDiagnostics,
			f.collisionDiagnostics,
			f.resolveDiagnostics,
			f.semaDiagnostics,
			f.compTimeDiagnostics,
		}

		// Collect errors
		errorRanges := make(map[core.Range]bool)

		for _, group := range groups {
			for _, d := range group {
				if d.Kind == core.Error {
					errorRanges[d.Range] = true
				}
			}
		}

		// Yield diagnostics with errors having priority
		seen := make(map[core.Range]bool)

		for _, group := range groups {
			for _, d := range group {
				if seen[d.Range] || (d.Kind == core.Warning && errorRanges[d.Range]) {
					continue
				}

				seen[d.Range] = true

				if !yield(d) {
					return
				}
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
