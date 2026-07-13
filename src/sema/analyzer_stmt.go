package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
)

// Visitor

func (a *analyzer) VisitBlock(b *ast.Block) {
	a.locals.Push()

	for _, stmt := range b.Stmts {
		a.AnalyzeStmt(stmt)
	}

	a.locals.Pop()
}

func (a *analyzer) VisitExpression(e *ast.Expression) {
	a.AnalyzeExpr(e.Expr)
}

func (a *analyzer) VisitVar(v *ast.Var) {
	var typ types.Type

	if !core.IsNil(v.Type) {
		typ = a.ResolveAndAnalyzeType(v.Type)
	}

	if !core.IsNil(v.Initializer) {
		init := a.AnalyzeExpr(v.Initializer)

		if core.IsNil(typ) {
			if isLiteralExpr(v.Initializer) {
				typ = defaultType(init.Type)
			} else {
				typ = init.Type
			}
		} else {
			a.ExpectType(typ, init, v.Initializer)
		}
	}

	if core.IsNil(typ) {
		a.Error(v.Name, "variable must have an explicit type or an initializer")
		return
	}

	if typ == types.PrimitiveVoid {
		a.Error(v.Name, "variable cannot be of type 'void'")
		typ = types.Invalid
	}

	a.nodeTypes[v] = typ

	symbol := symbols.Symbol{
		Kind:   symbols.Var,
		Public: true,
		Name:   v.Name.Token.Text,
		Node:   v,
		Type:   typ,
	}

	if !a.locals.Add(symbol) {
		a.Error(v.Name, "variable '%s' already exists in the current scope", symbol.Name)
	}
}

func (a *analyzer) VisitIf(i *ast.If) {
	condition := a.AnalyzeExpr(i.Condition)
	a.ExpectType(types.PrimitiveBool, condition, i.Condition)

	a.AnalyzeStmt(i.BranchTrue)
	a.AnalyzeStmt(i.BranchFalse)
}

func (a *analyzer) VisitWhile(w *ast.While) {
	condition := a.AnalyzeExpr(w.Condition)
	a.ExpectType(types.PrimitiveBool, condition, w.Condition)

	a.loop++
	a.AnalyzeStmt(w.Body)
	a.loop--
}

func (a *analyzer) VisitFor(f *ast.For) {
	a.locals.Push()

	a.AnalyzeStmt(f.Initializer)

	if !core.IsNil(f.Condition) {
		condition := a.AnalyzeExpr(f.Condition)
		a.ExpectType(types.PrimitiveBool, condition, f.Condition)
	}

	a.AnalyzeExpr(f.Increment)

	a.loop++
	a.AnalyzeStmt(f.Body)
	a.loop--

	a.locals.Pop()
}

func (a *analyzer) VisitReturn(r *ast.Return) {
	if core.IsNil(r.Value) {
		if !a.funcType.Returns.Equals(types.PrimitiveVoid) {
			a.Error(r, "expected '%s' got nothing", a.funcType.Returns)
		}
	} else {
		value := a.AnalyzeExpr(r.Value)
		a.ExpectType(a.funcType.Returns, value, r.Value)
	}
}

func (a *analyzer) VisitBreak(b *ast.Break) {
	if a.loop == 0 {
		a.Error(b, "'break' needs to be inside a loop")
	}
}

func (a *analyzer) VisitContinue(c *ast.Continue) {
	if a.loop == 0 {
		a.Error(c, "'continue' needs to be inside a loop")
	}
}

func (a *analyzer) VisitBadStmt(_ *ast.BadStmt) {}

// Utils

func (a *analyzer) AnalyzeStmt(stmt ast.Stmt) {
	if core.IsNil(stmt) {
		return
	}
	ast.VisitStmt(a, stmt)
}
