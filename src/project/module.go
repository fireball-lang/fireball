package project

import (
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

func (m *Module) GetScope(name string) (symbols.Scope, bool) {
	for _, child := range m.Children {
		if child.Name == name {
			return child, true
		}
	}

	return nil, false
}

func (m *Module) GetSymbol(name string) (symbols.Symbol, bool) {
	for _, file := range m.Files {
		if symbol, ok := symbols.SymbolScope(file.Symbols).GetSymbol(name); ok {
			return symbol, true
		}
	}

	return symbols.Symbol{}, false
}
