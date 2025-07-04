package project

import (
	"fireball/ast"
	"fireball/utils"
)

type Module struct {
	path  *ast.StringPath
	files []*File
}

func (m *Module) checkNameCollisions() {
	names := make(map[string]any)

	for _, file := range m.files {
		var diagnostics []utils.Diagnostic

		for _, decl := range file.ast.Decls {
			switch decl := decl.(type) {
			case *ast.Struct:
				if _, ok := names[decl.Name()]; ok {
					diagnostics = append(diagnostics, utils.Diagnostic{
						Kind:    utils.Error,
						Message: "Symbol with the name '" + decl.Name() + "' already exists in this module.",
						Range:   decl.NameN.Range(),
					})
				} else {
					names[decl.Name()] = nil
				}

			case *ast.Func:
				if _, ok := names[decl.Name()]; ok {
					diagnostics = append(diagnostics, utils.Diagnostic{
						Kind:    utils.Error,
						Message: "Symbol with the name '" + decl.Name() + "' already exists in this module.",
						Range:   decl.NameN.Range(),
					})
				} else {
					names[decl.Name()] = nil
				}
			}
		}

		if didChange(file.collectSymbolsDiagnostics, diagnostics) {
			file.collectSymbolsDiagnostics = diagnostics
			file.collectSymbolsDiagnosticsChanged = true
		}
	}
}

// analyzer.SymbolLookup

func (m *Module) GetTypeDecl(name string) ast.Decl {
	for _, file := range m.files {
		for _, decl := range file.ast.Decls {
			if s, ok := decl.(*ast.Struct); ok && s.Name() == name {
				return s
			}
		}
	}

	return nil
}

func (m *Module) GetFuncDecl(name string) *ast.Func {
	for _, file := range m.files {
		for _, decl := range file.ast.Decls {
			if s, ok := decl.(*ast.Func); ok && s.Name() == name {
				return s
			}
		}
	}

	return nil
}

func (m *Module) GetStructMethod(s *ast.Struct, name string) *ast.Func {
	for _, file := range m.files {
		for _, decl := range file.ast.Decls {
			if i, ok := decl.(*ast.Impl); ok && i.Struct == s {
				for _, method := range i.Methods {
					if method.Name() == name {
						return method
					}
				}
			}
		}
	}

	return nil
}

// analyzer.Module

func (m *Module) AbsolutePath() ast.PathLike {
	return m.path
}
