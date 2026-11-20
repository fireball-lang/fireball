package sema

import (
	"fireball/ast"
	"fireball/symbols"
	"fireball/types"
	"fmt"
)

type Pattern interface {
	_isPattern()
}

// Expr

type ExprPattern struct {
	Expr ast.Expr
}

func (e *ExprPattern) _isPattern() {}

// Union

type Binding struct {
	Local  *ast.Leaf
	Target *types.UnionCaseField
}

type UnionPattern struct {
	Case     *types.UnionCase
	Bindings []Binding
}

func (u *UnionPattern) _isPattern() {}

// Fallback

type FallbackPattern struct{}

func (f *FallbackPattern) _isPattern() {}

var Fallback = &FallbackPattern{}

// Bad

type BadPattern struct{}

func (b *BadPattern) _isPattern() {}

// Analyze

func (a *analyzer) AnalyzePattern(value ExprInfo, expr ast.Expr, union *types.Union) Pattern {
	// Fallback
	if expr, ok := expr.(*ast.Identifier); ok && len(expr.Path) == 1 && len(expr.Path[0].TypeArgs) == 0 && expr.Path[0].Name.Token.Text == "_" {
		return Fallback
	}

	// Union
	if init, ok := expr.(*ast.StructInitializer); ok && union != nil {
		// Case
		astInitType, ok := init.Type.(*ast.IdentifierType)
		if !ok {
			a.Error(init.Type, "union pattern needs an identifier type")
			return &BadPattern{}
		}

		symbol, ok := a.GetSymbol(symbols.Variable, astInitType.Path)
		if !ok {
			return &BadPattern{}
		}

		if symbol.Kind != symbols.UnionCase {
			a.Error(init.Type, "union pattern needs an union case")
			return &BadPattern{}
		}

		if astInitType.Mutable {
			a.Error(init.Type, "union type '%s' cannot be mutable", init.Type)
		}

		valueUnion := symbol.Type.(*types.Union)
		if !union.Equals(valueUnion) {
			a.Error(init.Type, "union pattern type needs to match value, got '%s', expected '%s'", valueUnion, union)
			return &BadPattern{}
		}

		caseNode := symbol.Node.(*ast.UnionCase)
		caseName := caseNode.Name.Token.Text
		cas, ok := union.Case(caseName)
		if !ok {
			panic(fmt.Sprintf("sema.analyzer.VisitStructInitializerUnion() - Failed to find union case '%s::%s'", union, caseName))
		}

		a.exprInfos[init] = ExprInfo{
			Type:   union,
			Node:   caseNode,
			Symbol: symbols.UnionCase,
		}

		// Bindings
		bindings := make([]Binding, 0, len(init.Fields))

		for _, field := range init.Fields {
			ident, ok := field.Value.(*ast.Identifier)
			if !ok || len(ident.Path) != 1 {
				a.Error(field.Value, "target of an union case binding needs to be a single identifier")
				continue
			}

			if len(ident.Path[0].TypeArgs) != 0 {
				a.ErrorRange(ast.SliceRange(ident.Path[0].TypeArgs), "union case binding target cannot have type arguments")
			}

			fieldName := ident.Path[0].Name.Token.Text
			casField := cas.Field(fieldName)

			if casField == nil {
				a.Error(field.Value, "union case '%s::%s' doesn't have a field with the name '%s'", union, cas.Name, fieldName)
				continue
			}

			a.exprInfos[ident] = ExprInfo{
				Type: casField.Type,
				Node: caseNode.Field(fieldName),
			}

			bindings = append(bindings, Binding{
				Local:  field.Name,
				Target: casField,
			})
		}

		return &UnionPattern{
			Case:     cas,
			Bindings: bindings,
		}
	}

	if union != nil {
		a.Error(expr, "invalid match pattern for a union value")
		return &BadPattern{}
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
