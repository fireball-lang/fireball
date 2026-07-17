package ast

type CfgPredicateVisitor[T any] interface {
	VisitTargetOs(t *TargetOsCfg) T
	VisitTargetFamily(t *TargetFamilyCfg) T

	VisitNot(n *NotCfg) T
	VisitAll(a *AllCfg) T
	VisitAny(a *AnyCfg) T

	VisitBad(b *BadCfg) T
}

func VisitCfgPredicate[V CfgPredicateVisitor[T], T any](visitor V, cfgPredicate CfgPredicate) T {
	switch pred := cfgPredicate.(type) {
	case *TargetOsCfg:
		return visitor.VisitTargetOs(pred)
	case *TargetFamilyCfg:
		return visitor.VisitTargetFamily(pred)

	case *NotCfg:
		return visitor.VisitNot(pred)
	case *AllCfg:
		return visitor.VisitAll(pred)
	case *AnyCfg:
		return visitor.VisitAny(pred)

	case *BadCfg:
		return visitor.VisitBad(pred)

	default:
		panic("ast.VisitCfgPredicate() - Invalid cfg predicate")
	}
}
