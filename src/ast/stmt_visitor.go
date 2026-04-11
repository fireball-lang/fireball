package ast

type StmtVisitor interface {
	VisitBlock(b *Block)
	VisitExpression(e *Expression)

	VisitVar(v *Var)
	VisitIf(i *If)
	VisitWhile(w *While)
	VisitFor(f *For)

	VisitReturn(r *Return)
	VisitBreak(b *Break)
	VisitContinue(c *Continue)

	VisitBadStmt(b *BadStmt)
}

func VisitStmt[V StmtVisitor](visitor V, stmt Stmt) {
	switch stmt := stmt.(type) {
	case *Block:
		visitor.VisitBlock(stmt)
	case *Expression:
		visitor.VisitExpression(stmt)

	case *Var:
		visitor.VisitVar(stmt)
	case *If:
		visitor.VisitIf(stmt)
	case *While:
		visitor.VisitWhile(stmt)
	case *For:
		visitor.VisitFor(stmt)

	case *Return:
		visitor.VisitReturn(stmt)
	case *Break:
		visitor.VisitBreak(stmt)
	case *Continue:
		visitor.VisitContinue(stmt)

	case *BadStmt:
		visitor.VisitBadStmt(stmt)

	default:
		panic("ast.VisitStmt() - Invalid stmt")
	}
}
