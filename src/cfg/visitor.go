package cfg

import "slices"

import "fireball/ast"

func (e *Env) VisitTargetOs(t *ast.TargetOsCfg) bool {
	if t.Not {
		return e.TargetOs != t.Kind
	}

	return e.TargetOs == t.Kind
}

func (e *Env) VisitTargetFamily(t *ast.TargetFamilyCfg) bool {
	if t.Not {
		return e.TargetFamily != t.Kind
	}

	return e.TargetFamily == t.Kind
}

func (e *Env) VisitNot(n *ast.NotCfg) bool {
	return !e.Visit(n.Predicate)
}

func (e *Env) VisitAll(a *ast.AllCfg) bool {
	for _, pred := range a.Predicates {
		if !e.Visit(pred) {
			return false
		}
	}

	return true
}

func (e *Env) VisitAny(a *ast.AnyCfg) bool {
	return slices.ContainsFunc(a.Predicates, e.Visit)
}

func (e *Env) VisitBad(_ *ast.BadCfg) bool {
	return true
}

// Utils

func (e *Env) Visit(pred ast.CfgPredicate) bool {
	return ast.VisitCfgPredicate(e, pred)
}
