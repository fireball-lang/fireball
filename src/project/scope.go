package project

import (
	"fireball/ast"
)

type globalScope struct {
	types         map[string]ast.Decl
	functions     map[string]*ast.Func
	structMethods map[string]map[string]*ast.Func
}

func newGlobalScope() *globalScope {
	return &globalScope{
		types:         make(map[string]ast.Decl),
		functions:     make(map[string]*ast.Func),
		structMethods: make(map[string]map[string]*ast.Func),
	}
}

func (g *globalScope) addMethod(structName string, f *ast.Func) bool {
	methods, ok := g.structMethods[structName]

	if !ok {
		methods = make(map[string]*ast.Func)
		g.structMethods[structName] = methods
	} else {
		if _, ok := methods[f.Name()]; ok {
			return false
		}
	}

	methods[f.Name()] = f
	return true
}

func (g *globalScope) GetTypeDecl(name string) ast.Decl {
	return g.types[name]
}

func (g *globalScope) GetFuncDecl(name string) *ast.Func {
	return g.functions[name]
}

func (g *globalScope) GetStructMethod(s *ast.Struct, name string) *ast.Func {
	if methods, ok := g.structMethods[s.Name()]; ok {
		return methods[name]
	}

	return nil
}
