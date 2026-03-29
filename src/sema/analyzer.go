package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
	"fmt"
)

type ExprInfo struct {
	Type    types.Type
	Address bool
}

func (e ExprInfo) Invalid() bool {
	return e.Type == types.Invalid
}

type analyzer struct {
	scope  *symbols.StackedScope
	locals *symbols.BlockScope

	path string

	exprInfos   map[ast.Expr]ExprInfo
	nodeTypes   map[ast.Node]types.Type
	diagnostics []core.Diagnostic

	funcType *types.Func
	loop     int

	typ     types.Type
	address bool
}

func Analyze(decls []ast.Decl, sym []symbols.Symbol, scope symbols.Scope, path string) (map[ast.Expr]ExprInfo, map[ast.Node]types.Type, []core.Diagnostic) {
	locals := &symbols.BlockScope{}

	r := analyzer{
		scope:     &symbols.StackedScope{Scopes: []symbols.Scope{scope, locals}},
		locals:    locals,
		path:      path,
		exprInfos: make(map[ast.Expr]ExprInfo),
		nodeTypes: make(map[ast.Node]types.Type),
	}

	// Symbols

	for i := 0; i < len(sym); i++ {
		r.ResolveSymbol(&sym[i])
	}

	// Declarations

	for _, decl := range decls {
		decl.VisitDecl(&r)
	}

	return r.exprInfos, r.nodeTypes, r.diagnostics
}

// Utils

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
	if typ != types.Invalid && !expr.Invalid() && !typ.Equals(expr.Type) {
		a.Error(node, "expected '%s', got '%s'", typ, expr.Type)
	}
}

func (a *analyzer) Error(node ast.Node, format string, args ...any) {
	a.diagnostics = append(a.diagnostics, core.Diagnostic{
		Kind:    core.Error,
		Path:    a.path,
		Range:   node.Range(),
		Message: fmt.Sprintf(format, args...),
	})
}
