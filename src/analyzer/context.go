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
	GetGlobalVar(name string) *ast.GlobalVar
	GetFuncDecl(name string) *ast.Func

	GetDeclMethod(decl ast.Decl, name string, static bool) *ast.Func
}

type Module interface {
	SymbolLookup

	AbsolutePath() ast.PathLike
	DeclImplementsInterface(decl ast.Decl, in *ast.Interface) bool
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

func (c *combinedScope) GetGlobalVar(name string) *ast.GlobalVar {
	for _, scope := range c.scopes {
		if g := scope.GetGlobalVar(name); g != nil {
			return g
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

func (c *combinedScope) GetDeclMethod(decl ast.Decl, name string, static bool) *ast.Func {
	for _, scope := range c.scopes {
		if m := scope.GetDeclMethod(decl, name, static); m != nil {
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
