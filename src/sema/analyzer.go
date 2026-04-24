package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"fmt"
	"strings"
)

type ExprInfo struct {
	Type types.Type
	Node ast.Node

	Mutable bool
	Address bool
}

func (e ExprInfo) Invalid() bool {
	return e.Type == types.Invalid
}

type analyzer struct {
	scope  *symbols.CombinedScope
	locals *symbols.BlockScope

	topLevelModule string
	path           string

	exprInfos   map[ast.Expr]ExprInfo
	nodeTypes   map[ast.Node]types.Type
	methodTable symbols.MethodTable
	diagnostics []core.Diagnostic

	stringViewType types.Type

	funcType *types.Func
	loop     int
}

func Analyze(file *ast.File, fileSymbols []symbols.Symbol, root symbols.Scope, methodTable symbols.MethodTable, topLevelModule, path string) (map[ast.Expr]ExprInfo, map[ast.Node]types.Type, []core.Diagnostic) {
	defer core.Scope()()

	locals := &symbols.BlockScope{}

	a := analyzer{
		scope:          nil,
		locals:         locals,
		topLevelModule: topLevelModule,
		path:           path,
		exprInfos:      make(map[ast.Expr]ExprInfo),
		nodeTypes:      make(map[ast.Node]types.Type),
		methodTable:    methodTable,
	}

	a.scope = &symbols.CombinedScope{Scopes: []symbols.Scope{
		root,
		a.GetImportsScope(root, file),
		symbols.SymbolScope(fileSymbols),
		locals,
	}}

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

	return a.exprInfos, a.nodeTypes, a.diagnostics
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
		for _, name := range i.Symbols {
			symbol, ok := importScope.GetSymbol(name.Token.Text)
			if !ok {
				a.Error(name, "symbol '%s' cannot be found", name.Token.Text)
				continue
			}

			scope.AddSymbol(symbol)
		}
	}

	return scope
}

func (a *analyzer) GetSymbol(path *ast.IdentifierPath) (symbols.Symbol, bool) {
	if len(path.Entries) == 0 {
		return symbols.Symbol{}, false
	}

	var scope symbols.Scope = a.scope

	for i := 0; i < len(path.Entries)-1; i++ {
		entry := path.Entries[i].Token.Text

		// Get module
		child, ok := scope.GetScope(entry)
		errMsg := "module '%s' cannot be found"

		// Get struct
		if !ok && i == len(path.Entries)-2 {
			var symbol symbols.Symbol
			symbol, ok = scope.GetSymbol(entry)

			if ok {
				name := path.Entries[i+1].Token.Text

				if f, typ := a.methodTable.GetStatic(symbol.Type, name); !core.IsNil(f) {
					return symbols.Symbol{
						Kind: symbols.Func,
						Name: f.Name(),
						Node: f,
						Type: typ,
					}, true
				}

				// Method error
				a.Error(path, "method '%s' cannot be found on type '%s'", name, symbol.Type)
				return symbols.Symbol{}, false
			}

			ok = false
			errMsg = "module or type '%s' cannot be found"
		}

		scope = child

		// Module error
		if !ok {
			sb := strings.Builder{}

			for i, leaf := range path.Entries[:i+1] {
				if i > 0 {
					sb.WriteString("::")
				}
				sb.WriteString(leaf.Token.Text)
			}

			a.Error(path, errMsg, &sb)
			return symbols.Symbol{}, false
		}
	}

	// Get symbol
	symbol, ok := scope.GetSymbol(path.Entries[len(path.Entries)-1].Token.Text)
	if !ok {
		entry := path.Entries[len(path.Entries)-1]
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
		_, ok := GetImplicitCast(expr.Type, typ)

		if !ok {
			a.Error(node, "expected '%s', got '%s'", typ, expr.Type)
		}
	}
}

func (a *analyzer) Error(node ast.Node, format string, args ...any) ExprInfo {
	a.diagnostics = append(a.diagnostics, core.Diagnostic{
		Kind:    core.Error,
		Path:    a.path,
		Range:   node.Range(),
		Message: fmt.Sprintf(format, args...),
	})

	return ExprInfo{Type: types.Invalid}
}
