package symbols

import (
	"fireball/ast"
	"fireball/types"
)

type ParamScope struct {
	Parent Scope

	Names  []string
	Params []*types.Param
	Nodes  []*ast.Leaf
}

func (p *ParamScope) GetScope(name string) (Scope, bool) {
	return p.Parent.GetScope(name)
}

func (p *ParamScope) GetSymbol(name string) (Symbol, bool) {
	for i, param := range p.Params {
		var n string
		if i < len(p.Names) {
			n = p.Names[i]
		} else {
			n = param.Name
		}

		if n == name {
			return Symbol{
				Kind: TypeParam,
				Name: name,
				Node: p.Nodes[i],
				Type: p.Params[i],
			}, true
		}
	}

	return p.Parent.GetSymbol(name)
}
