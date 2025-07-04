package analyzer

import (
	"fireball/ast"
	"fireball/utils"
)

type Context interface {
	GetAbsoluteModule(path ast.PathLike) Module
}

type SymbolLookup interface {
	GetTypeDecl(name string) ast.Decl
	GetFuncDecl(name string) *ast.Func
	GetStructMethod(s *ast.Struct, name string) *ast.Func
}

type Module interface {
	SymbolLookup

	AbsolutePath() ast.PathLike
}

type Scope interface {
	SymbolLookup

	GetModule(name string) Module
}

// combinedScope

type combinedScope struct {
	scopes []Scope
}

func Combine(scopes ...Scope) Scope {
	return &combinedScope{scopes: scopes}
}

// analyzer.SymbolLookup

func (c *combinedScope) GetTypeDecl(name string) ast.Decl {
	for _, scope := range c.scopes {
		if d := scope.GetTypeDecl(name); !utils.IsNil(d) {
			return d
		}
	}

	return nil
}

func (c *combinedScope) GetFuncDecl(name string) *ast.Func {
	for _, scope := range c.scopes {
		if f := scope.GetFuncDecl(name); f != nil {
			return f
		}
	}

	return nil
}

func (c *combinedScope) GetStructMethod(s *ast.Struct, name string) *ast.Func {
	for _, scope := range c.scopes {
		if m := scope.GetStructMethod(s, name); m != nil {
			return m
		}
	}

	return nil
}

// analyzer.Scope

func (c *combinedScope) GetModule(name string) Module {
	for _, scope := range c.scopes {
		if m := scope.GetModule(name); m != nil {
			return m
		}
	}

	return nil
}
