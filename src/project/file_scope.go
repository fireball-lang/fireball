package project

import (
	"fireball/analyzer"
	"fireball/ast"
)

type fileScope struct {
	ctx *context
	mod analyzer.Module
}

// analyzer.SymbolLookup

func (f *fileScope) GetTypeDecl(name string) ast.Decl {
	return f.mod.GetTypeDecl(name)
}

func (f *fileScope) GetGlobalVar(name string) *ast.GlobalVar {
	return f.mod.GetGlobalVar(name)
}

func (f *fileScope) GetFuncDecl(name string) *ast.Func {
	return f.mod.GetFuncDecl(name)
}

func (f *fileScope) GetStructMethod(s *ast.Struct, name string) *ast.Func {
	return f.mod.GetStructMethod(s, name)
}

// analyzer.Scope

func (f *fileScope) GetModule(name string) analyzer.Module {
	for _, module := range f.ctx.modules {
		if len(module.path.Segments) == 1 && module.path.Segments[0] == name {
			return module
		}
	}

	return nil
}
