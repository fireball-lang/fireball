package sema

import (
	"fireball/ast"
	fb_core "fireball/fb-core"
	"fireball/ir/eval"
	"fireball/symbols"
	"fireball/types"
)

type Context struct {
	Instantiations *types.InstantiationCache
	TypeEnv        *TypeEnvironment
	Builtins       fb_core.Builtins

	Files map[*ast.File]*FileData
}

type FileData struct {
	Symbols []symbols.Symbol

	ExprInfos   map[ast.Node]ExprInfo
	NodeTypes   map[ast.Node]types.Type
	Evaluations map[ast.Expr]eval.Value
}

type ExprInfo struct {
	Type types.Type
	Node ast.Node

	Symbol symbols.Kind

	Mutable  bool
	Address  bool
	CompTime bool
}

func (e ExprInfo) Invalid() bool {
	return e.Type == types.Invalid
}
