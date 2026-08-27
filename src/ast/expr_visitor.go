package ast

type ExprVisitor[T any] interface {
	VisitBool(b *Bool) T
	VisitNumber(n *Number) T
	VisitCharacter(c *Character) T
	VisitString(s *String) T
	VisitNull(n *Null) T

	VisitStructInitializer(s *StructInitializer) T
	VisitWith(w *With) T
	VisitArrayInitializer(a *ArrayInitializer) T

	VisitSizeOf(s *SizeOf) T
	VisitAlignOf(a *AlignOf) T
	VisitOffsetOf(o *OffsetOf) T
	VisitTypeOf(t *TypeOf) T

	VisitPrefix(p *Prefix) T
	VisitPostfix(p *Postfix) T
	VisitBinary(b *Binary) T

	VisitIdentifier(i *Identifier) T
	VisitIndex(i *Index) T
	VisitMember(m *Member) T
	VisitCall(c *Call) T
	VisitCast(c *Cast) T

	VisitBadExpr(p *BadExpr) T
}

func VisitExpr[V ExprVisitor[T], T any](visitor V, expr Expr) T {
	switch expr := expr.(type) {
	case *Bool:
		return visitor.VisitBool(expr)
	case *Number:
		return visitor.VisitNumber(expr)
	case *Character:
		return visitor.VisitCharacter(expr)
	case *String:
		return visitor.VisitString(expr)
	case *Null:
		return visitor.VisitNull(expr)

	case *StructInitializer:
		return visitor.VisitStructInitializer(expr)
	case *With:
		return visitor.VisitWith(expr)
	case *ArrayInitializer:
		return visitor.VisitArrayInitializer(expr)

	case *SizeOf:
		return visitor.VisitSizeOf(expr)
	case *AlignOf:
		return visitor.VisitAlignOf(expr)
	case *OffsetOf:
		return visitor.VisitOffsetOf(expr)
	case *TypeOf:
		return visitor.VisitTypeOf(expr)

	case *Prefix:
		return visitor.VisitPrefix(expr)
	case *Postfix:
		return visitor.VisitPostfix(expr)
	case *Binary:
		return visitor.VisitBinary(expr)

	case *Identifier:
		return visitor.VisitIdentifier(expr)
	case *Index:
		return visitor.VisitIndex(expr)
	case *Member:
		return visitor.VisitMember(expr)
	case *Call:
		return visitor.VisitCall(expr)
	case *Cast:
		return visitor.VisitCast(expr)

	case *BadExpr:
		return visitor.VisitBadExpr(expr)

	default:
		panic("ast.VisitExpr() - Invalid expression")
	}
}
