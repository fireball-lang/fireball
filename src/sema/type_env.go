package sema

import (
	"fireball/ast"
	"fireball/symbols"
	"fireball/types"
)

type TypeEnvironment struct {
	static      map[types.Type][]symbols.Symbol
	instance    map[types.Type][]symbols.Symbol
	structNodes map[*types.Struct]*ast.Struct
}

func NewTypeEnvironment() *TypeEnvironment {
	return &TypeEnvironment{
		static:      make(map[types.Type][]symbols.Symbol),
		instance:    make(map[types.Type][]symbols.Symbol),
		structNodes: make(map[*types.Struct]*ast.Struct),
	}
}

func (e *TypeEnvironment) RegisterStruct(t *types.Struct, n *ast.Struct) {
	e.structNodes[t] = n
}

func (e *TypeEnvironment) GetStructNode(t *types.Struct) *ast.Struct {
	return e.structNodes[t]
}

func (e *TypeEnvironment) AddStaticMethod(typ types.Type, symbol symbols.Symbol) bool {
	for _, m := range e.static[typ] {
		if m.Name == symbol.Name {
			return false
		}
	}

	e.static[typ] = append(e.static[typ], symbol)
	return true
}

func (e *TypeEnvironment) AddInstanceMethod(typ types.Type, symbol symbols.Symbol) bool {
	for _, m := range e.instance[typ] {
		if m.Name == symbol.Name {
			return false
		}
	}

	e.instance[typ] = append(e.instance[typ], symbol)
	return true
}

func (e *TypeEnvironment) GetStaticMethod(typ types.Type, name string) (symbols.Symbol, bool) {
	for _, m := range e.static[typ] {
		if m.Name == name {
			return m, true
		}
	}

	return symbols.Symbol{}, false
}

func (e *TypeEnvironment) GetInstanceMethod(typ types.Type, name string) (symbols.Symbol, bool) {
	for _, m := range e.instance[typ] {
		if m.Name == name {
			return m, true
		}
	}

	return symbols.Symbol{}, false
}

func (e *TypeEnvironment) GetTypeScope(typ types.Type) symbols.Scope {
	methods := e.static[typ]
	if len(methods) == 0 {
		return nil
	}

	return symbols.SymbolScope(methods)
}
