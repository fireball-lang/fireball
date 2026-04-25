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
	Param
	Var
)

type Symbol struct {
	Kind Kind
	Name string

	Node ast.Node
	Type types.Type
}
