package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"fmt"
	"slices"
	"strings"
)

type ExprInfo struct {
	Type types.Type
	Node ast.Node

	Symbol symbols.Kind

	Mutable bool
	Address bool
}

func (e ExprInfo) Invalid() bool {
	return e.Type == types.Invalid
}

type analyzer struct {
	scopes symbols.ScopeStack
	locals *symbols.BlockScope

	topLevelModule  string
	path            string
	fileModPath     []string
	checkVisibility bool

	exprInfos      map[ast.Expr]ExprInfo
	nodeTypes      map[ast.Node]types.Type
	instantiations types.InstantiationCache
	typeEnv        *TypeEnvironment
	diagnostics    []core.Diagnostic

	stringViewType types.Type

	selfType types.Type

	funcType *types.Func
	loop     int
}

func Analyze(file *ast.File, fileSymbols []symbols.Symbol, root symbols.Scope, instantiations types.InstantiationCache, typeEnv *TypeEnvironment, nodeTypes map[ast.Node]types.Type, topLevelModule, path string) (map[ast.Expr]ExprInfo, []core.Diagnostic) {
	defer core.Scope()()

	locals := &symbols.BlockScope{}

	fileModPath := make([]string, 0, len(file.Mod.Path.Entries))
	for _, entry := range file.Mod.Path.Entries {
		fileModPath = append(fileModPath, entry.Token.Text)
	}

	a := analyzer{
		locals:          locals,
		topLevelModule:  topLevelModule,
		path:            path,
		fileModPath:     fileModPath,
		checkVisibility: true,
		exprInfos:       make(map[ast.Expr]ExprInfo),
		nodeTypes:       nodeTypes,
		instantiations:  instantiations,
		typeEnv:         typeEnv,
	}

	a.scopes.Push(root)
	a.scopes.Push(a.GetImportsScope(root, file))
	a.scopes.Push(symbols.SymbolScope(fileSymbols))
	a.scopes.Push(locals)

	// Core

	stringViewSymbol, ok := a.GetSymbol(&ast.IdentifierPath{Entries: []*ast.Leaf{
		{Token: lexer.Token{Text: "core"}},
		{Token: lexer.Token{Text: "StringView"}},
	}})
	if !ok {
		panic("analyze.Analyze() - Failed to find 'core::StringView'")
	}

	a.stringViewType = stringViewSymbol.Type

	// Module

	if len(file.Mod.Path.Entries) > 0 && file.Mod.Path.Entries[0].Token.Text != a.topLevelModule {
		a.Error(file.Mod.Path.Entries[0], "top level module needs to match the project name")
	}

	// Declarations

	for _, decl := range file.Decls {
		ast.VisitDecl(&a, decl)
	}

	// Cleanup

	for i := 0; i < 4; i++ {
		a.scopes.Pop()
	}

	a.scopes.ValidateEmpty()

	return a.exprInfos, a.diagnostics
}

// Utils

func (a *analyzer) GetImportsScope(root symbols.Scope, file *ast.File) symbols.Scope {
	scope := symbols.NewBasicScope()

	for _, i := range file.Imports {
		// Get import scope
		importScope, ok := getScope(root, i.Path.Entries)
		if !ok {
			a.Error(i.Path, "module '%s' cannot be found", i.Path)
			continue
		}

		// Scope import
		if len(i.Symbols) == 0 {
			var name string
			var errNode ast.Node

			if i.Alias == nil {
				name = i.Path.LastName()
				errNode = i.Path.Entries[len(i.Path.Entries)-1]
			} else {
				name = i.Alias.Token.Text
				errNode = i.Alias
			}

			if !scope.AddScope(name, importScope) {
				a.Error(errNode, "module alias with the name '%s' already exists", name)
			}

			continue
		}

		// Symbols import
		importModPath := make([]string, len(i.Path.Entries))
		for j, entry := range i.Path.Entries {
			importModPath[j] = entry.Token.Text
		}

		for _, name := range i.Symbols {
			symbol, ok := importScope.GetSymbol(name.Token.Text)
			if !ok {
				a.Error(name, "symbol '%s' cannot be found", name.Token.Text)
				continue
			}

			if a.checkVisibility && !symbol.Public && !slices.Equal(importModPath, a.fileModPath) {
				a.Error(name, "symbol '%s' is private", name.Token.Text)
			}

			scope.AddSymbol(symbol)

			if a.nodeTypes != nil {
				a.nodeTypes[name] = symbol.Type
			}
		}
	}

	return scope
}

func (a *analyzer) GetSymbol(path *ast.IdentifierPath) (symbols.Symbol, bool) {
	if len(path.Entries) == 0 {
		return symbols.Symbol{}, false
	}

	var scope symbols.Scope = &a.scopes
	crossedModuleBoundary := false

	for i := 0; i < len(path.Entries)-1; i++ {
		entry := path.Entries[i].Token.Text

		// Try module scope navigation first.
		if child, ok := scope.GetScope(entry); ok {
			crossedModuleBoundary = true
			scope = child
			continue
		}

		// Try type scope (for Struct::staticMethod).
		if symbol, ok := scope.GetSymbol(entry); ok {
			a.nodeTypes[path.Entries[i]] = symbol.Type

			if typeScope := a.typeEnv.GetTypeScope(symbol.Type); typeScope != nil {
				// Check if this type belongs to a different module.
				if a.checkVisibility {
					if s, ok := symbol.Type.(*types.Struct); ok {
						if !slices.Equal(s.ModulePath, a.fileModPath) {
							crossedModuleBoundary = true
						}
					}
				}

				scope = typeScope
				continue
			}

			// Symbol found but has no registered static methods.
			a.Error(path, "method '%s' cannot be found on type '%s'", path.Entries[i+1].Token.Text, symbol.Type)
			return symbols.Symbol{}, false
		}

		// Neither a module nor a type — build the full path for the error message.
		sb := strings.Builder{}

		for j, leaf := range path.Entries[:i+1] {
			if j > 0 {
				sb.WriteString("::")
			}
			sb.WriteString(leaf.Token.Text)
		}

		a.Error(path, "module or type '%s' cannot be found", &sb)
		return symbols.Symbol{}, false
	}

	// Look up the final segment in whatever scope we navigated to.
	entry := path.Entries[len(path.Entries)-1]

	symbol, ok := scope.GetSymbol(entry.Token.Text)
	if ok {
		if crossedModuleBoundary && a.checkVisibility && !symbol.Public {
			a.Error(entry, "symbol '%s' is private", entry.Token.Text)
		}

		a.nodeTypes[entry] = symbol.Type
	} else {
		a.Error(entry, "symbol '%s' cannot be found", entry.Token.Text)
	}

	return symbol, ok
}

func getScope(scope symbols.Scope, path []*ast.Leaf) (symbols.Scope, bool) {
	for _, entry := range path {
		var ok bool
		scope, ok = scope.GetScope(entry.Token.Text)

		if !ok {
			return nil, false
		}
	}

	return scope, true
}

func (a *analyzer) ExpectPrimitiveClass(predicate func(kind types.PrimitiveKind) bool, className string, expr ExprInfo, node ast.Node) types.Type {
	if expr.Invalid() {
		return types.Invalid
	}

	if t, ok := expr.Type.(*types.Primitive); ok && predicate(t.Kind) {
		return t
	}

	a.Error(node, "expected %s type, got '%s'", className, expr.Type)
	return types.Invalid
}

func (a *analyzer) ExpectType(typ types.Type, expr ExprInfo, node ast.Node) {
	if typ != types.Invalid && !expr.Invalid() {
		_, ok := GetImplicitCast(a.typeEnv, expr.Type, typ)

		if !ok {
			a.Error(node, "expected '%s', got '%s'", typ, expr.Type)
		}
	}
}

func (a *analyzer) Error(node ast.Node, format string, args ...any) ExprInfo {
	return a.ErrorRange(node.Range(), format, args...)
}

func (a *analyzer) ErrorRange(range_ core.Range, format string, args ...any) ExprInfo {
	a.diagnostics = append(a.diagnostics, core.Diagnostic{
		Kind:    core.Error,
		Path:    a.path,
		Range:   range_,
		Message: fmt.Sprintf(format, args...),
	})

	return ExprInfo{Type: types.Invalid}
}
