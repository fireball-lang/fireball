package symbols

import (
	"fireball/ast"
	"fireball/types"
)

type Kind uint8

const (
	Invalid Kind = iota
	Struct
	Func
	TypeParam
	Param
	Var
)

type Symbol struct {
	Kind Kind

	Public bool
	Name   string

	Node ast.Node
	Type types.Type
}
