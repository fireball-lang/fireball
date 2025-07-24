package project

import (
	"fireball/ast"
	"fireball/lexer"
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
				checkName(decl, decl.NameN.Range(), names, &diagnostics)
			case *ast.Enum:
				checkName(decl, decl.NameN.Range(), names, &diagnostics)
			case *ast.Interface:
				checkName(decl, decl.NameN.Range(), names, &diagnostics)
			case *ast.GlobalVar:
				checkName(decl, decl.NameN.Range(), names, &diagnostics)
			case *ast.Func:
				checkName(decl, decl.NameN.Range(), names, &diagnostics)
			}
		}

		if didChange(file.collectSymbolsDiagnostics, diagnostics) {
			file.collectSymbolsDiagnostics = diagnostics
			file.collectSymbolsDiagnosticsChanged = true
		}
	}
}

func checkName(decl ast.Decl, nameRange lexer.Range, names map[string]any, diagnostics *[]utils.Diagnostic) {
	if _, ok := names[decl.Name()]; ok {
		*diagnostics = append(*diagnostics, utils.Diagnostic{
			Kind:    utils.Error,
			Message: "Symbol with the name '" + decl.Name() + "' already exists in this module.",
			Range:   nameRange,
		})
	} else {
		names[decl.Name()] = nil
	}
}

// analyzer.SymbolLookup

func (m *Module) GetTypeDecl(name string) ast.Decl {
	for _, file := range m.files {
		for _, decl := range file.ast.Decls {
			switch decl.(type) {
			case *ast.Struct, *ast.Enum, *ast.Interface:
				if decl.Name() == name {
					return decl
				}
			}
		}
	}

	return nil
}

func (m *Module) GetGlobalVar(name string) *ast.GlobalVar {
	for _, file := range m.files {
		for _, decl := range file.ast.Decls {
			if g, ok := decl.(*ast.GlobalVar); ok && g.Name() == name {
				return g
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

func (m *Module) GetDeclMethod(decl ast.Decl, name string, static bool) *ast.Func {
	for _, file := range m.files {
		for _, d := range file.ast.Decls {
			if i, ok := d.(*ast.Impl); ok && i.Decl == decl {
				methods := i.Methods

				if static {
					methods = i.StaticMethods
				}

				for _, method := range methods {
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

func (m *Module) DeclImplementsInterface(decl ast.Decl, in *ast.Interface) bool {
	for _, file := range m.files {
		for _, d := range file.ast.Decls {
			if impl, ok := d.(*ast.Impl); ok && impl.Decl == decl && impl.Interface == in {
				return true
			}
		}
	}

	return false
}
