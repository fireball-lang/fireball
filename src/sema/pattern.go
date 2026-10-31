package sema

import "fireball/ast"

type Pattern interface {
	_isPattern()
}

type ExprPattern struct {
	Expr ast.Expr
}

func (e *ExprPattern) _isPattern() {}

type FallbackPattern struct{}

func (f *FallbackPattern) _isPattern() {}

var Fallback = &FallbackPattern{}

func (a *analyzer) AnalyzePattern(value ExprInfo, expr ast.Expr) Pattern {
	// Fallback
	if expr, ok := expr.(*ast.Identifier); ok && len(expr.Path) == 1 && len(expr.Path[0].TypeArgs) == 0 && expr.Path[0].Name.Token.Text == "_" {
		return Fallback
	}

	// Expr
	info := a.AnalyzeExpr(expr)

	if !info.CompTime {
		expr, _ := a.FindDeepestNonCompTimeExpr(expr)
		a.Error(expr, "expression cannot be evaluated at compile time")
	} else {
		a.ExpectType(value.Type, info, expr)
	}

	return &ExprPattern{Expr: expr}
}
