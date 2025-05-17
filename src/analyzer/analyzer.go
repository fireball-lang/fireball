package analyzer

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/utils"
	"fmt"
	"strings"
)

type analyzer struct {
	scope       Scope
	diagnostics []utils.Diagnostic
}

func Analyze(f *ast.File, scope Scope) []utils.Diagnostic {
	a := analyzer{scope: scope}
	a.visit(f)

	return a.diagnostics
}

// Declarations

func (a *analyzer) VisitStruct(s *ast.Struct) {
}

func (a *analyzer) VisitFunc(f *ast.Func) {
}

// Expressions

func (a *analyzer) VisitBlock(b *ast.Block) {
	b.Result().SetInvalid()
}

func (a *analyzer) VisitVar(v *ast.Var) {
	if ast.IsValid(v.Type) {
		a.checkType(v.Value, v.Type)
	}

	v.Result().SetInvalid()
}

func (a *analyzer) VisitIf(i *ast.If) {
	a.checkType(i.Condition, ast.BoolType)

	i.Result().SetInvalid()
}

func (a *analyzer) VisitWhile(w *ast.While) {
	a.checkType(w.Condition, ast.BoolType)

	w.Result().SetInvalid()
}

func (a *analyzer) VisitLiteral(l *ast.Literal) {
	switch l.Value.Token.Kind {
	case lexer.Identifier:
		l.Result().Set(ast.Value, ast.BoolType)

	case lexer.Number:
		type_ := ast.I32Type

		if strings.ContainsRune(l.Value.Token.Text, '.') {
			type_ = ast.F64Type
		}

		l.Result().Set(ast.Value, type_)

	case lexer.String:
		l.Result().Set(ast.Value, &ast.PointerType{Pointee: ast.U8Type})

	default:
		panic("analyzer.analyzer.VisitLiteral() - Invalid token kind")
	}
}

func (a *analyzer) VisitParen(p *ast.Paren) {
	if ast.IsValid(p.Expr) {
		*p.Result() = *p.Expr.Result()
	}
}

func (a *analyzer) VisitIdentifier(i *ast.Identifier) {
}

func (a *analyzer) VisitCall(c *ast.Call) {
}

func (a *analyzer) VisitIndex(i *ast.Index) {
}

func (a *analyzer) VisitMember(m *ast.Member) {
}

func (a *analyzer) VisitUnary(u *ast.Unary) {
}

func (a *analyzer) VisitBinary(b *ast.Binary) {
}

// Utils

func (a *analyzer) visit(node ast.Node) {
	for child := range node.Children() {
		a.visit(child)
	}

	switch node := node.(type) {
	case ast.Decl:
		node.Visit(a)
	case ast.Expr:
		node.Visit(a)
	}
}

func (a *analyzer) checkType(expr ast.Expr, expected ast.Type) {
	if ast.IsValid(expr) && expr.Result().Kind != ast.Invalid && !expr.Result().Type.Equals(expected) {
		a.error(expr, fmt.Sprintf("Expected type '%s' but got '%s'.", expected, expr.Result().Type))
	}
}

func (a *analyzer) error(node ast.Node, message string) {
	a.diagnostics = append(a.diagnostics, utils.Diagnostic{
		Kind:    utils.Error,
		Message: message,
		Range:   node.Range(),
	})
}
