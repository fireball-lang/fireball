package analyzer

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/utils"
	"slices"
)

type fileScope struct {
	parent Scope

	lookups   []SymbolLookup
	types     []ast.Decl
	functions []*ast.Func
	modules   []Module
}

func getFileScope[T utils.DiagnosticConsumer](file *ast.File, ctx Context, scope Scope, diagnostics T) Scope {
	fileScope := &fileScope{
		parent: scope,
	}

	for _, decl := range file.Decls {
		if decl, ok := decl.(*ast.Import); ok {
			if decl.Path == nil {
				continue
			}

			module := ctx.GetAbsoluteModule(decl.Path)

			if utils.IsNil(module) {
				addError(diagnostics, decl.Path, "Module with the path '"+ast.PathString(decl.Path)+"' doesn't exist.")
				continue
			}

			// Module
			if len(decl.Symbols) == 0 {
				fileScope.modules = append(fileScope.modules, module)
				continue
			}

			// All symbols
			if slices.ContainsFunc(decl.Symbols, isLeafStar) {
				fileScope.lookups = append(fileScope.lookups, module)
				continue
			}

			// Some symbols
			if !utils.IsNil(diagnostics) {
				decl.ResolvedSymbols = make([]ast.Decl, len(decl.Symbols))
			}

			for i, symbol := range decl.Symbols {
				var resolved ast.Decl

				// Type
				if decl := module.GetTypeDecl(symbol.Token.Text); ast.IsValid(decl) {
					fileScope.types = append(fileScope.types, decl)
					resolved = decl
				}

				// Function
				if !ast.IsValid(resolved) {
					if fun := module.GetFuncDecl(symbol.Token.Text); ast.IsValid(fun) {
						fileScope.functions = append(fileScope.functions, fun)
						resolved = fun
					}
				}

				// Set resolved
				if !utils.IsNil(diagnostics) {
					decl.ResolvedSymbols[i] = resolved
				}

				// Error
				if !ast.IsValid(resolved) {
					addError(diagnostics, symbol, "Symbol with name '"+symbol.Token.Text+"' in the module '"+ast.PathString(decl.Path)+"' doesn't exist.")
				}
			}
		}
	}

	return fileScope
}

func isLeafStar(leaf *ast.Leaf) bool {
	return leaf.Token.Kind == lexer.Star
}

// analyzer.SymbolLookup

func (f *fileScope) GetTypeDecl(name string) ast.Decl {
	for _, lookup := range f.lookups {
		if decl := lookup.GetTypeDecl(name); ast.IsValid(decl) {
			return decl
		}
	}

	for _, decl := range f.types {
		if decl.Name() == name {
			return decl
		}
	}

	return f.parent.GetTypeDecl(name)
}

func (f *fileScope) GetGlobalVar(name string) *ast.GlobalVar {
	return f.parent.GetGlobalVar(name)
}

func (f *fileScope) GetFuncDecl(name string) *ast.Func {
	for _, lookup := range f.lookups {
		if fun := lookup.GetFuncDecl(name); ast.IsValid(fun) {
			return fun
		}
	}

	for _, fun := range f.functions {
		if fun.Name() == name {
			return fun
		}
	}

	return f.parent.GetFuncDecl(name)
}

func (f *fileScope) GetDeclMethod(decl ast.Decl, name string, static bool) *ast.Func {
	for _, lookup := range f.lookups {
		if method := lookup.GetDeclMethod(decl, name, static); ast.IsValid(method) {
			return method
		}
	}

	return f.parent.GetDeclMethod(decl, name, static)
}

// analyzer.Scope

func (f *fileScope) GetModule(name string) Module {
	for _, module := range f.modules {
		path := module.AbsolutePath()

		if path.SegmentAt(path.SegmentCount()-1) == name {
			return module
		}
	}

	return f.parent.GetModule(name)
}
