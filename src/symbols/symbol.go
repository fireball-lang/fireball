package symbols

import (
	"fireball/ast"
	"fireball/types"
)

type Kind uint8

const (
	Invalid Kind = iota
	TypeAlias
	Struct
	Enum
	Interface
	Func
	TypeParam
	Case
	Param
	Var
)

func (k Kind) Domain() Domain {
	switch k {
	case Invalid:
		return 0
	case TypeAlias, Struct, Enum, Interface, TypeParam:
		return Type
	case Func:
		return Function
	case Var, Param, Case:
		return Variable

	default:
		panic("symbols.Kind.Domain() - Invalid Kind")
	}
}

func (k Kind) IsInDomain(domain Domain) bool {
	return domain&k.Domain() != 0
}

type Symbol struct {
	Kind Kind

	Public bool
	Name   string

	Node ast.Node
	Type types.Type
}
