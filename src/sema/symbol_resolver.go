package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
)

func ResolveSymbols(file *ast.File, fileSymbols []symbols.Symbol, root symbols.Scope, path string) []core.Diagnostic {
	a := analyzer{
		path: path,
	}

	a.scope = &symbols.CombinedScope{Scopes: []symbols.Scope{
		root,
		a.GetImportsScope(root, file),
		symbols.SymbolScope(fileSymbols),
	}}

	for i := range fileSymbols {
		a.ResolveSymbol(&fileSymbols[i])
	}

	return a.diagnostics
}
