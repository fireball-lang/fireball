package project

import (
	"fireball/ast"
	"fireball/core"
	"fireball/parser"
	"fireball/symbols"
	"os"
)

type File struct {
	Path string

	Decls       []ast.Decl
	Diagnostics []core.Diagnostic

	Symbols []symbols.Symbol
}

func newFile(path string) *File {
	return &File{
		Path:        path,
		Decls:       nil,
		Diagnostics: nil,
	}
}

func (f *File) parse() {
	file, err := os.Open(f.Path)
	if err != nil {
		panic(err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	f.Decls, f.Diagnostics = parser.Parse(file, f.Path)

	f.Symbols = symbols.Collect(f.Decls)
}
