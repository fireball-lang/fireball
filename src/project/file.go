package project

import (
	"fireball/ast"
	"fireball/core"
	"fireball/parser"
	"fireball/sema"
	"fireball/symbols"
	"fireball/types"
	"iter"
	"os"
)

type File struct {
	Proj *Project
	Path string

	Ast              *ast.File
	parseDiagnostics []core.Diagnostic

	Symbols []symbols.Symbol

	resolveDiagnostics []core.Diagnostic

	ExprInfos       map[ast.Expr]sema.ExprInfo
	NodeTypes       map[ast.Node]types.Type
	semaDiagnostics []core.Diagnostic
}

func newFile(proj *Project, path string) *File {
	return &File{
		Proj: proj,
		Path: path,
		Ast:  nil,
	}
}

func (f *File) parse() {
	file, err := os.Open(f.Path)
	if err != nil {
		panic(err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	f.Ast, f.parseDiagnostics = parser.Parse(file, f.Path)
	f.Symbols = symbols.Collect(f.Ast)
}

func (f *File) resolve(root symbols.Scope, methodTable symbols.MethodTable) {
	f.resolveDiagnostics = sema.ResolveSymbols(f.Ast, f.Symbols, methodTable, root, f.Path)
}

func (f *File) analyze(root symbols.Scope, methodTable symbols.MethodTable) {
	f.ExprInfos, f.NodeTypes, f.semaDiagnostics = sema.Analyze(f.Ast, f.Symbols, root, methodTable, f.Proj.Config.Name, f.Path)
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
