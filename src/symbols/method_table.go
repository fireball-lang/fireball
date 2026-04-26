package symbols

import (
	"fireball/ast"
	"fireball/types"
)

type MethodTable struct {
	static   map[types.Type][]method
	instance map[types.Type][]method
}

type method struct {
	ast *ast.Func
	typ *types.Func
}

func NewMethodTable() MethodTable {
	return MethodTable{
		static:   make(map[types.Type][]method),
		instance: make(map[types.Type][]method),
	}
}

func (m MethodTable) AddStatic(typ types.Type, f *ast.Func, t *types.Func) bool {
	methods := m.static[typ]

	for _, m := range methods {
		if m.ast.Name() == f.Name() {
			return false
		}
	}

	methods = append(methods, method{
		ast: f,
		typ: t,
	})

	m.static[typ] = methods
	return true
}

func (m MethodTable) GetStatic(typ types.Type, name string) (*ast.Func, *types.Func) {
	for _, m := range m.static[typ] {
		if m.ast.Name().Token.Text == name {
			return m.ast, m.typ
		}
	}

	return nil, nil
}

func (m MethodTable) Add(typ types.Type, f *ast.Func, t *types.Func) bool {
	methods := m.instance[typ]

	for _, m := range methods {
		if m.ast.Name() == f.Name() {
			return false
		}
	}

	methods = append(methods, method{
		ast: f,
		typ: t,
	})

	m.instance[typ] = methods
	return true
}

func (m MethodTable) Get(typ types.Type, name string) (*ast.Func, *types.Func) {
	for _, m := range m.instance[typ] {
		if m.ast.Name().Token.Text == name {
			return m.ast, m.typ
		}
	}

	return nil, nil
}
