package project

import (
	"fireball/ast"
	"fireball/core"
	"fireball/parser"
	"fireball/sema"
	"fireball/symbols"
	"iter"
	"os"
)

type File struct {
	Path string

	Decls            []ast.Decl
	parseDiagnostics []core.Diagnostic

	Symbols []symbols.Symbol

	ExprInfos       map[ast.Expr]sema.ExprInfo
	semaDiagnostics []core.Diagnostic
}

func newFile(path string) *File {
	return &File{
		Path:  path,
		Decls: nil,
	}
}

func (f *File) parse() {
	file, err := os.Open(f.Path)
	if err != nil {
		panic(err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	f.Decls, f.parseDiagnostics = parser.Parse(file, f.Path)
	f.Symbols = symbols.Collect(f.Decls)
	f.ExprInfos, f.semaDiagnostics = sema.Analyze(f.Decls, f.Symbols, symbols.SimpleScope(f.Symbols), f.Path)
}

func (f *File) Diagnostics() iter.Seq[core.Diagnostic] {
	return func(yield func(core.Diagnostic) bool) {
		for _, diagnostic := range f.parseDiagnostics {
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
