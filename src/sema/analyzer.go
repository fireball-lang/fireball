package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"reflect"
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
	common

	locals *symbols.BlockScope

	topLevelModule string

	exprInfos map[ast.Expr]ExprInfo

	stringViewType types.Type

	funcType *types.Func
	loop     int
}

var fileAllowedAttributes = []reflect.Type{
	reflect.TypeFor[ast.Cfg](),
}

var importAllowedAttributes = []reflect.Type{
	reflect.TypeFor[ast.Cfg](),
}

func Analyze(file *ast.File, fileSymbols []symbols.Symbol, root symbols.Scope, instantiations *types.InstantiationCache, typeEnv *TypeEnvironment, nodeTypes map[ast.Node]types.Type, topLevelModule, path string) (map[ast.Expr]ExprInfo, []core.Diagnostic) {
	defer core.Scope()()

	// Setup

	a := analyzer{
		common:         setupCommon(file, fileSymbols, root, instantiations, typeEnv, nodeTypes, path),
		locals:         &symbols.BlockScope{},
		topLevelModule: topLevelModule,
		exprInfos:      make(map[ast.Expr]ExprInfo),
	}

	a.checkTypeConstraints = true

	a.scopes.Push(a.locals)

	// Core

	stringViewSymbol, ok := a.GetSymbol([]*ast.IdentifierEntry{
		{Name: &ast.Leaf{Token: lexer.Token{Text: "core"}}},
		{Name: &ast.Leaf{Token: lexer.Token{Text: "StringView"}}},
	})
	if !ok {
		panic("analyze.Analyze() - Failed to find 'core::StringView'")
	}

	a.stringViewType = stringViewSymbol.Type

	// Module

	if len(file.Mod.Path) > 0 && file.Mod.Path[0].Token.Text != a.topLevelModule {
		a.Error(file.Mod.Path[0], "top level module needs to match the project name")
	}

	// File & Import attributes

	a.CheckAttributes(file.Attributes_, fileAllowedAttributes)

	for _, import_ := range file.Imports {
		a.CheckAttributes(import_.Attributes_, importAllowedAttributes)
	}

	// Declarations

	for _, decl := range file.Decls {
		ast.VisitDecl(&a, decl)
	}

	// Cleanup

	for range 4 {
		a.scopes.Pop()
	}

	a.scopes.ValidateEmpty()

	return a.exprInfos, a.diagnostics
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
	if typ != types.Invalid && !expr.Invalid() {
		if _, ok := typ.(*types.Pointer); ok {
			if _, ok := node.(*ast.Null); ok {
				return
			}
		}

		_, ok := GetImplicitCast(a.typeEnv, expr.Type, typ)

		if !ok {
			a.Error(node, "expected '%s', got '%s'", typ, expr.Type)
		}
	}
}
