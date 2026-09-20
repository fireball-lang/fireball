package project

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
)

type Module struct {
	Parent *Module
	Name   string

	Children []*Module
	Files    []*File

	types map[string]*symbols.Symbol
	vars  map[string]*symbols.Symbol
	funcs map[string]*symbols.Symbol
}

func (m *Module) getOrCreateChild(name string) *Module {
	for _, child := range m.Children {
		if child.Name == name {
			return child
		}
	}

	child := &Module{Parent: m, Name: name}
	m.Children = append(m.Children, child)

	return child
}

func (m *Module) CheckCollisions() {
	// Check collisions across files in this module
	types := make(map[string]*symbols.Symbol)
	vars := make(map[string]*symbols.Symbol)
	funcs := make(map[string]*symbols.Symbol)

	for _, file := range m.Files {
		file.collisionDiagnostics = nil

		for i := range file.Symbols {
			symbol := &file.Symbols[i]

			switch symbol.Kind.Domain() {
			case symbols.Type:
				if _, ok := types[symbol.Name]; ok {
					file.collisionDiagnostics = append(file.collisionDiagnostics, m.getCollisionDiagnostic(file, symbol, "type"))
				} else {
					types[symbol.Name] = symbol
				}

			case symbols.Variable:
				if _, ok := vars[symbol.Name]; ok {
					file.collisionDiagnostics = append(file.collisionDiagnostics, m.getCollisionDiagnostic(file, symbol, "variable"))
				} else {
					vars[symbol.Name] = symbol
				}

			case symbols.Function:
				if _, ok := funcs[symbol.Name]; ok {
					file.collisionDiagnostics = append(file.collisionDiagnostics, m.getCollisionDiagnostic(file, symbol, "function"))
				} else {
					funcs[symbol.Name] = symbol
				}

			default:
			}
		}
	}

	m.types = types
	m.vars = vars
	m.funcs = funcs

	// Check collisions in children modules
	for _, child := range m.Children {
		child.CheckCollisions()
	}
}

func (m *Module) getCollisionDiagnostic(file *File, symbol *symbols.Symbol, kind string) core.Diagnostic {
	var rangeNode = symbol.Node

	if decl, ok := rangeNode.(ast.Decl); ok {
		rangeNode = decl.Name()
	}

	return core.Diagnostic{
		Kind:    core.Error,
		Path:    file.Path,
		Range:   rangeNode.Range(),
		Message: kind + " '" + symbol.Name + "' already exists in module '" + m.Name + "'",
	}
}

func (m *Module) GetScope(name string) (symbols.Scope, bool) {
	for _, child := range m.Children {
		if child.Name == name {
			return child, true
		}
	}

	return nil, false
}

func (m *Module) GetSymbol(domain symbols.Domain, name string) (symbols.Symbol, bool) {
	if domain&symbols.Type != 0 {
		if symbol, ok := m.types[name]; ok {
			return *symbol, true
		}
	}

	if domain&symbols.Variable != 0 {
		if symbol, ok := m.vars[name]; ok {
			return *symbol, true
		}
	}

	if domain&symbols.Function != 0 {
		if symbol, ok := m.funcs[name]; ok {
			return *symbol, true
		}
	}

	return symbols.Symbol{}, false
}
