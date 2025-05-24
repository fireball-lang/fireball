package project

import (
	"fireball/ast"
)

type globalScope struct {
	types     map[string]ast.Decl
	functions map[string]*ast.Func
}

func newGlobalScope() *globalScope {
	return &globalScope{
		types:     make(map[string]ast.Decl),
		functions: make(map[string]*ast.Func),
	}
}

func (g *globalScope) GetTypeDecl(name string) ast.Decl {
	return g.types[name]
}

func (g *globalScope) GetFuncDecl(name string) *ast.Func {
	return g.functions[name]
}
