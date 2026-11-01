package ast

type StmtVisitor[T any] interface {
	VisitBlock(b *Block) T
	VisitExpression(e *Expression) T

	VisitVar(v *Var) T
	VisitIf(i *If) T
	VisitMatch(m *Match) T
	VisitWhile(w *While) T
	VisitFor(f *For) T

	VisitReturn(r *Return) T
	VisitBreak(b *Break) T
	VisitContinue(c *Continue) T

	VisitBadStmt(b *BadStmt) T
}

func VisitStmt[V StmtVisitor[T], T any](visitor V, stmt Stmt) T {
	switch stmt := stmt.(type) {
	case *Block:
		return visitor.VisitBlock(stmt)
	case *Expression:
		return visitor.VisitExpression(stmt)

	case *Var:
		return visitor.VisitVar(stmt)
	case *If:
		return visitor.VisitIf(stmt)
	case *Match:
		return visitor.VisitMatch(stmt)
	case *While:
		return visitor.VisitWhile(stmt)
	case *For:
		return visitor.VisitFor(stmt)

	case *Return:
		return visitor.VisitReturn(stmt)
	case *Break:
		return visitor.VisitBreak(stmt)
	case *Continue:
		return visitor.VisitContinue(stmt)

	case *BadStmt:
		return visitor.VisitBadStmt(stmt)

	default:
		panic("ast.VisitStmt() - Invalid stmt")
	}
}
