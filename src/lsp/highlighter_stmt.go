package lsp

import (
	"fireball/ast"
	"fireball/core"
	"fireball/sema"
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
		patterns := hi.file.Sema.ArmPatterns[arm]

		for i, expr := range arm.Patterns {
			if init, ok := expr.(*ast.StructInitializer); ok && i < len(patterns) {
				if _, ok := patterns[i].(*sema.UnionPattern); ok {
					hi.UnionPattern(init)
					continue
				}
			}

			hi.VisitExpr(expr)
		}

		hi.VisitStmt(arm.Body)
	}

	return 0
}

func (hi *highlighter) UnionPattern(init *ast.StructInitializer) {
	if target, ok := init.Type.(*ast.IdentifierType); ok && len(target.Path) > 0 {
		hi.UnionCasePath(target)
	} else {
		hi.VisitType(init.Type)
	}

	for _, field := range init.Fields {
		hi.AddFull(field.Name, variableKind, 0)

		if id, ok := field.Value.(*ast.Identifier); ok && len(id.Path) == 1 {
			hi.Add(id.Path[0].Name, propertyKind, 0)
		} else {
			hi.VisitExpr(field.Value)
		}
	}
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
