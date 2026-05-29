package sema

import (
	"fireball/ast"
	"fireball/symbols"
	"fireball/types"
)

type implKey struct {
	Type      types.Type
	Interface *types.Interface
}

type TypeEnvironment struct {
	static   map[types.Type][]symbols.Symbol
	instance map[types.Type][]symbols.Symbol

	structNodes    map[*types.Struct]*ast.Struct
	interfaceNodes map[*types.Interface]*ast.Interface
	implNodes      map[implKey]*ast.Impl

	conformances map[types.Type][]*types.Interface

	instantiations types.InstantiationCache
}

func NewTypeEnvironment(instantiations types.InstantiationCache) *TypeEnvironment {
	return &TypeEnvironment{
		static:         make(map[types.Type][]symbols.Symbol),
		instance:       make(map[types.Type][]symbols.Symbol),
		structNodes:    make(map[*types.Struct]*ast.Struct),
		interfaceNodes: make(map[*types.Interface]*ast.Interface),
		implNodes:      make(map[implKey]*ast.Impl),
		conformances:   make(map[types.Type][]*types.Interface),
		instantiations: instantiations,
	}
}

func (e *TypeEnvironment) RegisterStruct(t *types.Struct, n *ast.Struct) {
	e.structNodes[t] = n
}

func (e *TypeEnvironment) GetStructNode(t *types.Struct) *ast.Struct {
	return e.structNodes[t]
}

func (e *TypeEnvironment) RegisterInterface(t *types.Interface, in *ast.Interface) {
	e.interfaceNodes[t] = in
}

func (e *TypeEnvironment) GetInterfaceNode(t *types.Interface) *ast.Interface {
	if n := e.interfaceNodes[t]; n != nil {
		return n
	}

	if t.Generic != nil {
		return e.interfaceNodes[t.Generic]
	}

	return nil
}

func (e *TypeEnvironment) RegisterImplNode(typ types.Type, in *types.Interface, impl *ast.Impl) {
	e.implNodes[implKey{Type: typ, Interface: in}] = impl
}

func (e *TypeEnvironment) GetImplNode(structGeneric types.Type, ifaceGeneric *types.Interface) *ast.Impl {
	return e.implNodes[implKey{Type: structGeneric, Interface: ifaceGeneric}]
}

func (e *TypeEnvironment) AddConformance(typ types.Type, in *types.Interface) bool {
	for _, in2 := range e.conformances[typ] {
		if in2 == in {
			return false
		}
	}

	e.conformances[typ] = append(e.conformances[typ], in)
	return true
}

func (e *TypeEnvironment) GetConformances(typ types.Type) []*types.Interface {
	direct := e.conformances[typ]

	s, ok := typ.(*types.Struct)
	if !ok || s.Generic == nil {
		return direct
	}

	templateConfs := e.conformances[s.Generic]
	if len(templateConfs) == 0 {
		return direct
	}

	result := make([]*types.Interface, len(direct), len(direct)+len(templateConfs))
	copy(result, direct)

	for _, in := range templateConfs {
		instantiated := e.instantiations.Substitute(in, s.Substitutions).(*types.Interface)
		result = append(result, instantiated)
	}

	return result
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
