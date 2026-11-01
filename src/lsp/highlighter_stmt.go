package lsp

import (
	"fireball/ast"
	"fireball/core"
)

func (hi *highlighter) VisitBlock(b *ast.Block) int {
	for _, stmt := range b.Stmts {
		hi.VisitStmt(stmt)
	}

	return 0
}

func (hi *highlighter) VisitExpression(e *ast.Expression) int {
	hi.VisitExpr(e.Expr)

	return 0
}

func (hi *highlighter) VisitVar(v *ast.Var) int {
	hi.AddFull(v.Name, variableKind, 0)
	hi.VisitType(v.Type)
	hi.VisitExpr(v.Initializer)

	return 0
}

func (hi *highlighter) VisitIf(i *ast.If) int {
	hi.VisitExpr(i.Condition)
	hi.VisitStmt(i.BranchTrue)
	hi.VisitStmt(i.BranchFalse)

	return 0
}

func (hi *highlighter) VisitMatch(m *ast.Match) int {
	hi.VisitExpr(m.Value)

	for _, arm := range m.Arms {
		for _, pattern := range arm.Patterns {
			hi.VisitExpr(pattern)
		}

		hi.VisitStmt(arm.Body)
	}

	return 0
}

func (hi *highlighter) VisitWhile(w *ast.While) int {
	hi.VisitExpr(w.Condition)
	hi.VisitStmt(w.Body)

	return 0
}

func (hi *highlighter) VisitFor(f *ast.For) int {
	hi.VisitStmt(f.Initializer)
	hi.VisitExpr(f.Condition)
	hi.VisitExpr(f.Increment)
	hi.VisitStmt(f.Body)

	return 0
}

func (hi *highlighter) VisitReturn(r *ast.Return) int {
	hi.VisitExpr(r.Value)

	return 0
}

func (hi *highlighter) VisitBreak(_ *ast.Break) int {
	return 0
}

func (hi *highlighter) VisitContinue(_ *ast.Continue) int {
	return 0
}

func (hi *highlighter) VisitBadStmt(_ *ast.BadStmt) int {
	return 0
}

// Utils

func (hi *highlighter) VisitStmt(stmt ast.Stmt) {
	if !core.IsNil(stmt) {
		ast.VisitStmt(hi, stmt)
	}
}
