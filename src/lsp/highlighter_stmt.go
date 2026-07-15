package lsp

import (
	"fireball/ast"
	"fireball/core"
)

func (hi *highlighter) VisitBlock(b *ast.Block) {
	for _, stmt := range b.Stmts {
		hi.VisitStmt(stmt)
	}
}

func (hi *highlighter) VisitExpression(e *ast.Expression) {
	hi.VisitExpr(e.Expr)
}

func (hi *highlighter) VisitVar(v *ast.Var) {
	hi.AddFull(v.Name, variableKind)
	hi.VisitType(v.Type)
	hi.VisitExpr(v.Initializer)
}

func (hi *highlighter) VisitIf(i *ast.If) {
	hi.VisitExpr(i.Condition)
	hi.VisitStmt(i.BranchTrue)
	hi.VisitStmt(i.BranchFalse)
}

func (hi *highlighter) VisitWhile(w *ast.While) {
	hi.VisitExpr(w.Condition)
	hi.VisitStmt(w.Body)
}

func (hi *highlighter) VisitFor(f *ast.For) {
	hi.VisitStmt(f.Initializer)
	hi.VisitExpr(f.Condition)
	hi.VisitExpr(f.Increment)
	hi.VisitStmt(f.Body)
}

func (hi *highlighter) VisitReturn(r *ast.Return) {
	hi.VisitExpr(r.Value)
}

func (hi *highlighter) VisitBreak(_ *ast.Break) {
}

func (hi *highlighter) VisitContinue(_ *ast.Continue) {
}

func (hi *highlighter) VisitBadStmt(_ *ast.BadStmt) {
}

// Utils

func (hi *highlighter) VisitStmt(stmt ast.Stmt) {
	if !core.IsNil(stmt) {
		ast.VisitStmt(hi, stmt)
	}
}
