package symbols

import (
	"fireball/ast"
	"fireball/types"
)

type Kind uint8

const (
	Struct Kind = iota
	Func
)

type Symbol struct {
	Kind Kind
	Decl ast.Decl
	Type types.Type
}
