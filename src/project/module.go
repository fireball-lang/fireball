package project

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
)

type Module struct {
	Name string

	Children []*Module
	Files    []*File
}

func (m *Module) getOrCreateChild(name string) *Module {
	for _, child := range m.Children {
		if child.Name == name {
			return child
		}
	}

	child := &Module{Name: name}
	m.Children = append(m.Children, child)

	return child
}

func (m *Module) CheckCollisions() {
	// Check collisions across files in this module
	typeDomain := make(map[string]bool)
	varDomain := make(map[string]bool)
	funcDomain := make(map[string]bool)

	for _, file := range m.Files {
		file.collisionDiagnostics = nil

		for _, symbol := range file.Symbols {
			switch symbol.Kind {
			case symbols.Enum, symbols.Struct, symbols.Interface:
				if _, ok := typeDomain[symbol.Name]; ok {
					file.collisionDiagnostics = append(file.collisionDiagnostics, m.getCollisionDiagnostic(file, symbol, "type"))
				} else {
					typeDomain[symbol.Name] = true
				}

			case symbols.Var:
				if _, ok := varDomain[symbol.Name]; ok {
					file.collisionDiagnostics = append(file.collisionDiagnostics, m.getCollisionDiagnostic(file, symbol, "variable"))
				} else {
					varDomain[symbol.Name] = true
				}

			case symbols.Func:
				if _, ok := funcDomain[symbol.Name]; ok {
					file.collisionDiagnostics = append(file.collisionDiagnostics, m.getCollisionDiagnostic(file, symbol, "function"))
				} else {
					funcDomain[symbol.Name] = true
				}

			default:
			}
		}
	}

	// Check collisions in children modules
	for _, child := range m.Children {
		child.CheckCollisions()
	}
}

func (m *Module) getCollisionDiagnostic(file *File, symbol symbols.Symbol, kind string) core.Diagnostic {
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
	for _, file := range m.Files {
		if symbol, ok := symbols.SymbolScope(file.Symbols).GetSymbol(domain, name); ok {
			return symbol, true
		}
	}

	return symbols.Symbol{}, false
}
