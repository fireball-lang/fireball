package sema

import "fireball/types"

func (e *TypeEnvironment) satisfiesConstraint(typ types.Type, constraint *types.Interface, substitutions []types.Substitution) bool {
	instantiated := e.instantiations.Substitute(constraint, substitutions).(*types.Interface)
	instantiated = instantiated.AsImmutable()

	// Type parameter: satisfied if any of its own constraints matches.
	if p, ok := typ.(*types.Param); ok {
		for _, c := range p.Constraints {
			if c.AsImmutable().Equals(instantiated) {
				return true
			}
		}

		return false
	}

	// Pointer: constraints apply to the pointee.
	if ptr, ok := typ.(*types.Pointer); ok {
		return e.satisfiesInterface(ptr.Pointee, instantiated)
	}

	return e.satisfiesInterface(typ, instantiated)
}

func (e *TypeEnvironment) satisfiesInterface(typ types.Type, in *types.Interface) bool {
	for _, conf := range e.GetConformances(typ) {
		if e.interfaceMatches(conf, in) {
			return true
		}
	}

	return false
}

// interfaceMatches reports whether conformance `conf` satisfies the constraint
// `in`. Both must instantiate the same interface template. For generic interfaces
// each constraint argument must be implicitly castable to the conformance's
// argument, mirroring how the binary operators (e.g. `==` via core::Eq) resolve.
func (e *TypeEnvironment) interfaceMatches(conf, in *types.Interface) bool {
	conf = conf.AsImmutable()
	in = in.AsImmutable()

	confTemplate := conf
	if conf.Generic != nil {
		confTemplate = conf.Generic
	}

	inTemplate := in
	if in.Generic != nil {
		inTemplate = in.Generic
	}

	if confTemplate != inTemplate {
		return false
	}

	if in.Generic == nil {
		return true
	}

	if len(in.Substitutions) != len(conf.Substitutions) {
		return false
	}

	for i, sub := range in.Substitutions {
		if _, ok := GetImplicitCast(e, ExprInfo{Type: sub.Type}, conf.Substitutions[i].Type); !ok {
			return false
		}
	}

	return true
}
