package project

import (
	"fireball/ast"
	"fireball/core"
	"fireball/parser"
	"os"
)

type File struct {
	Path string

	Decls       []ast.Decl
	Diagnostics []core.Diagnostic
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
}
