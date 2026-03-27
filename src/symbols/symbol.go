package symbols

import (
	"fireball/ast"
	"fireball/types"
)

type Kind uint8

const (
	Struct Kind = iota
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
