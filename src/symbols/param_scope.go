package symbols

import (
	"fireball/ast"
	"fireball/types"
)

type ParamScope struct {
	Names  []string
	Params []*types.Param
	Nodes  []*ast.TypeParam
}

func (p *ParamScope) GetScope(_ string) (Scope, bool) {
	return nil, false
}

func (p *ParamScope) GetSymbol(_ Domain, name string) (Symbol, bool) {
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
				Node: p.Nodes[i].Name,
				Type: p.Params[i],
			}, true
		}
	}

	return Symbol{}, false
}
