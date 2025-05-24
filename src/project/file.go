package project

import (
	"fireball/ast"
	"fireball/utils"
	"path/filepath"
)

type FileContentsProvider interface {
	AbsolutePath() string

	Contents() (string, bool)
}

type File struct {
	project *Project

	provider FileContentsProvider
	ast      *ast.File

	parseDiagnostics        []utils.Diagnostic
	parseDiagnosticsChanged bool

	collectSymbolsDiagnostics        []utils.Diagnostic
	collectSymbolsDiagnosticsChanged bool

	analyzeDiagnostics        []utils.Diagnostic
	analyzeDiagnosticsChanged bool
}

func (f *File) AbsolutePath() string {
	return f.provider.AbsolutePath()
}

func (f *File) SrcRelativePath() string {
	path, err := filepath.Rel(filepath.Join(f.project.AbsolutePath, "src"), f.AbsolutePath())
	if err != nil {
		panic(err.Error())
	}

	return path
}

func (f *File) Ast() *ast.File {
	return f.ast
}

func (f *File) Diagnostics() []utils.Diagnostic {
	if f.parseDiagnosticsChanged || f.collectSymbolsDiagnosticsChanged || f.analyzeDiagnosticsChanged {
		f.parseDiagnosticsChanged = false
		f.collectSymbolsDiagnosticsChanged = false
		f.analyzeDiagnosticsChanged = false

		diagnostics := make([]utils.Diagnostic, 0, len(f.parseDiagnostics)+len(f.analyzeDiagnostics))
		diagnostics = append(diagnostics, f.parseDiagnostics...)
		diagnostics = append(diagnostics, f.collectSymbolsDiagnostics...)
		diagnostics = append(diagnostics, f.analyzeDiagnostics...)

		return diagnostics
	}

	return nil
}
