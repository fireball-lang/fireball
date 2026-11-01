package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/types"
)

type Exits uint8

const (
	ExitFallthrough Exits = 1 << iota
	ExitReturn
	ExitBreak
	ExitContinue
)

type flowAnalyzer struct {
	sema *FileData
}

func AnalyzeFlow(sema *FileData, body ast.Stmt) Exits {
	if exits, ok := sema.StmtExits[body]; ok {
		return exits
	}

	defer core.Scope()()

	f := flowAnalyzer{sema: sema}
	return f.Stmt(body)
}

// Statements

func (f *flowAnalyzer) VisitBlock(b *ast.Block) Exits {
	exits := ExitFallthrough

	for _, stmt := range b.Stmts {
		exits = flowSequence(exits, f.Stmt(stmt))
	}

	return exits
}

func (f *flowAnalyzer) VisitExpression(e *ast.Expression) Exits {
	return f.Expr(e.Expr)
}

func (f *flowAnalyzer) VisitVar(v *ast.Var) Exits {
	return f.Expr(v.Initializer)
}

func (f *flowAnalyzer) VisitIf(i *ast.If) Exits {
	condition := f.Expr(i.Condition)
	branchTrue := f.Stmt(i.BranchTrue)
	branchFalse := f.Stmt(i.BranchFalse)

	branches := branchTrue | branchFalse

	if value, known := flowBool(i.Condition); known {
		if value {
			branches = branchTrue
		} else {
			branches = branchFalse
		}
	}

	return flowSequence(condition, branches)
}

func (f *flowAnalyzer) VisitMatch(m *ast.Match) Exits {
	value := f.Expr(m.Value)

	var arms Exits

	for _, arm := range m.Arms {
		arms |= f.Stmt(arm.Body)
	}

	// An empty match is an error because matches are exhaustive.
	// Treat the body as ExitFallthrough to not generate multiple errors for the same cause.
	if len(m.Arms) == 0 {
		arms = ExitFallthrough
	}

	return flowSequence(value, arms)
}

func (f *flowAnalyzer) VisitWhile(w *ast.While) Exits {
	condition := f.Expr(w.Condition)
	body := f.Stmt(w.Body)

	value, known := flowBool(w.Condition)

	return flowLoop(condition, value, known, body, ExitFallthrough)
}

func (f *flowAnalyzer) VisitFor(loop *ast.For) Exits {
	initializer := f.Stmt(loop.Initializer)
	condition := f.Expr(loop.Condition)
	body := f.Stmt(loop.Body)
	increment := f.Expr(loop.Increment)

	value, known := flowBool(loop.Condition)
	if core.IsNil(loop.Condition) {
		value, known = true, true
	}

	return flowSequence(initializer, flowLoop(condition, value, known, body, increment))
}

func (f *flowAnalyzer) VisitReturn(r *ast.Return) Exits {
	return flowSequence(f.Expr(r.Value), ExitReturn)
}

func (f *flowAnalyzer) VisitBreak(_ *ast.Break) Exits {
	return ExitBreak
}

func (f *flowAnalyzer) VisitContinue(_ *ast.Continue) Exits {
	return ExitContinue
}

func (f *flowAnalyzer) VisitBadStmt(_ *ast.BadStmt) Exits {
	return ExitFallthrough
}

// Expressions

func (f *flowAnalyzer) VisitBool(_ *ast.Bool) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitNumber(_ *ast.Number) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitCharacter(_ *ast.Character) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitString(_ *ast.String) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitNull(_ *ast.Null) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitStructInitializer(s *ast.StructInitializer) Exits {
	exits := ExitFallthrough

	for _, field := range s.Fields {
		exits = flowSequence(exits, f.Expr(field.Value))
	}

	return exits
}

func (f *flowAnalyzer) VisitWith(w *ast.With) Exits {
	exits := f.Expr(w.Expr)

	for _, field := range w.Fields {
		exits = flowSequence(exits, f.Expr(field.Value))
	}

	return exits
}

func (f *flowAnalyzer) VisitArrayInitializer(a *ast.ArrayInitializer) Exits {
	exits := ExitFallthrough

	for _, element := range a.Elements {
		exits = flowSequence(exits, f.Expr(element))
	}

	return exits
}

func (f *flowAnalyzer) VisitTupleInitializer(t *ast.TupleInitializer) Exits {
	exits := ExitFallthrough

	for _, value := range t.Values {
		exits = flowSequence(exits, f.Expr(value))
	}

	return exits
}

func (f *flowAnalyzer) VisitSizeOf(_ *ast.SizeOf) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitAlignOf(_ *ast.AlignOf) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitOffsetOf(_ *ast.OffsetOf) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitTypeOf(_ *ast.TypeOf) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitPrefix(p *ast.Prefix) Exits {
	return f.Expr(p.Expr)
}

func (f *flowAnalyzer) VisitPostfix(p *ast.Postfix) Exits {
	exits := f.Expr(p.Expr)

	if p.Op == ast.PropagateO {
		return flowSequence(exits, ExitReturn|ExitFallthrough)
	}

	return exits
}

func (f *flowAnalyzer) VisitBinary(b *ast.Binary) Exits {
	left := f.Expr(b.Left)
	right := f.Expr(b.Right)

	switch b.Op {
	case ast.BoolAnd, ast.BoolOr:
		if value, known := flowBool(b.Left); known {
			if (b.Op == ast.BoolAnd && !value) || (b.Op == ast.BoolOr && value) {
				return left
			}

			return flowSequence(left, right)
		}

		return flowSequence(left, ExitFallthrough|right)

	case ast.Or:
		return flowSequence(left, ExitFallthrough|right)

	default:
		return flowSequence(left, right)
	}
}

func (f *flowAnalyzer) VisitIdentifier(_ *ast.Identifier) Exits {
	return ExitFallthrough
}

func (f *flowAnalyzer) VisitIndex(i *ast.Index) Exits {
	return flowSequence(f.Expr(i.Expr), f.Expr(i.Index))
}

func (f *flowAnalyzer) VisitMember(m *ast.Member) Exits {
	return f.Expr(m.Expr)
}

func (f *flowAnalyzer) VisitCall(c *ast.Call) Exits {
	exits := f.Expr(c.Callee)

	for _, arg := range c.Args {
		exits = flowSequence(exits, f.Expr(arg))
	}

	if f.sema.ExprInfos[c].Type == types.NeverType {
		return flowSequence(exits, 0)
	}

	return exits
}

func (f *flowAnalyzer) VisitCast(c *ast.Cast) Exits {
	return f.Expr(c.Expr)
}

func (f *flowAnalyzer) VisitBadExpr(_ *ast.BadExpr) Exits {
	return ExitFallthrough
}

// Utils

func flowLoop(condition Exits, value, known bool, body, increment Exits) Exits {
	if known && !value {
		return flowSequence(condition, ExitFallthrough)
	}

	exits := body & ExitReturn
	if !known || body&ExitBreak != 0 {
		exits |= ExitFallthrough
	}

	if body&(ExitFallthrough|ExitContinue) != 0 {
		exits |= increment & ExitReturn
	}

	return flowSequence(condition, exits)
}

func flowSequence(first, second Exits) Exits {
	if first&ExitFallthrough == 0 {
		return first
	}

	return (first & ^ExitFallthrough) | second
}

func flowBool(expr ast.Expr) (value bool, known bool) {
	if core.IsNil(expr) {
		return false, false
	}

	if b, ok := expr.(*ast.Bool); ok {
		return b.Value, true
	}
	return false, false
}

func (f *flowAnalyzer) Stmt(stmt ast.Stmt) Exits {
	if core.IsNil(stmt) {
		return ExitFallthrough
	}

	exits := ast.VisitStmt(f, stmt)
	f.sema.StmtExits[stmt] = exits

	return exits
}

func (f *flowAnalyzer) Expr(expr ast.Expr) Exits {
	if core.IsNil(expr) {
		return ExitFallthrough
	}

	return ast.VisitExpr(f, expr)
}
