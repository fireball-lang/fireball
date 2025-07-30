package project

import (
	"fireball/analyzer"
	"fireball/ast"
	"fireball/utils"
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

func (f *fileScope) GetDeclMethod(decl ast.Decl, name string, static bool) *ast.Func {
	if in, ok := decl.(*ast.Interface); ok {
		for _, method := range in.Methods {
			if method.Name() == name {
				return method
			}
		}
	}

	modPath := ast.Root(decl).ModulePath()
	mod := f.ctx.GetAbsoluteModule(modPath)

	if utils.IsNil(mod) {
		return nil
	}

	return mod.GetDeclMethod(decl, name, static)
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
