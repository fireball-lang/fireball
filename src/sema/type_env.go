package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
	"slices"
)

type implKey struct {
	Type      types.Type
	Interface *types.Interface
}

type TypeEnvironment struct {
	static   map[types.Type][]symbols.Symbol
	instance map[types.Type][]symbols.Symbol

	typeDeclNodes map[types.Type]ast.Decl
	implNodes     map[implKey]*ast.Impl
	implParams    map[*ast.Impl][]*types.Param

	conformances map[types.Type][]*types.Interface

	paramScopes map[*types.Param]symbols.Scope

	implTargets   map[*types.Struct][]*types.Struct
	implForTarget map[*types.Struct]*ast.Impl

	instantiations *types.InstantiationCache
}

func NewTypeEnvironment(instantiations *types.InstantiationCache) *TypeEnvironment {
	return &TypeEnvironment{
		static:         make(map[types.Type][]symbols.Symbol),
		instance:       make(map[types.Type][]symbols.Symbol),
		typeDeclNodes:  make(map[types.Type]ast.Decl),
		implNodes:      make(map[implKey]*ast.Impl),
		implParams:     make(map[*ast.Impl][]*types.Param),
		conformances:   make(map[types.Type][]*types.Interface),
		paramScopes:    make(map[*types.Param]symbols.Scope),
		implTargets:    make(map[*types.Struct][]*types.Struct),
		implForTarget:  make(map[*types.Struct]*ast.Impl),
		instantiations: instantiations,
	}
}

func (e *TypeEnvironment) CheckCollisions(fn func(diagnostic core.Diagnostic)) {
	for t := range e.typeDeclNodes {
		funcDomain := make(map[string]bool)

		for _, symbol := range e.static[t] {
			if symbol.Kind == symbols.Func {
				if _, ok := funcDomain[symbol.Name]; ok {
					fn(core.Diagnostic{
						Kind:    core.Error,
						Path:    ast.GetFile(symbol.Node).Path,
						Range:   symbol.Node.(*ast.Func).Name().Range(),
						Message: "method '" + symbol.Name + "' already exists on type '" + t.String() + "'",
					})
				} else {
					funcDomain[symbol.Name] = true
				}
			}
		}
	}
}

func (e *TypeEnvironment) RegisterTypeDeclNode(t types.Type, n ast.Decl) {
	if in, ok := t.(*types.Interface); ok {
		t = in.AsImmutable()
	}

	e.typeDeclNodes[t] = n

	if t, ok := t.(*types.Enum); ok {
		n := n.(*ast.Enum)
		syms := e.static[t]

		for i, c := range t.Cases {
			syms = append(syms, symbols.Symbol{
				Kind:   symbols.Case,
				Public: true,
				Name:   c.Name,
				Node:   n.Cases[i],
				Type:   t,
			})
		}

		e.static[t] = syms
	}
}

func (e *TypeEnvironment) GetStructNode(t *types.Struct) *ast.Struct {
	if n, ok := e.typeDeclNodes[t]; ok {
		return n.(*ast.Struct)
	}

	return nil
}

func (e *TypeEnvironment) GetEnumNode(t *types.Enum) *ast.Enum {
	if n, ok := e.typeDeclNodes[t]; ok {
		return n.(*ast.Enum)
	}

	return nil
}

func (e *TypeEnvironment) GetInterfaceNode(t *types.Interface) *ast.Interface {
	t = t.AsImmutable()

	if t.Generic != nil {
		t = t.Generic
	}

	if n, ok := e.typeDeclNodes[t]; ok {
		return n.(*ast.Interface)
	}

	return nil
}

func (e *TypeEnvironment) RegisterImplNode(typ types.Type, in *types.Interface, impl *ast.Impl) {
	e.implNodes[implKey{Type: typ, Interface: in}] = impl
}

func (e *TypeEnvironment) RegisterImplTarget(typ types.Type, impl *ast.Impl) {
	if s, ok := typ.(*types.Struct); ok && s.Generic != nil {
		e.implForTarget[s] = impl

		for _, sub := range s.Substitutions {
			if _, ok := sub.Type.(*types.Param); ok {
				e.implTargets[s.Generic] = append(e.implTargets[s.Generic], s)
				break
			}
		}
	}
}

func (e *TypeEnvironment) RegisterImplParams(impl *ast.Impl, params []*types.Param) {
	e.implParams[impl] = params
}

func (e *TypeEnvironment) GetImplNode(structGeneric types.Type, ifaceGeneric *types.Interface) *ast.Impl {
	return e.implNodes[implKey{Type: structGeneric, Interface: ifaceGeneric}]
}

func (e *TypeEnvironment) AddConformance(typ types.Type, in *types.Interface) bool {
	in = in.AsImmutable()

	if slices.Contains(e.conformances[typ], in) {
		return false
	}

	e.conformances[typ] = append(e.conformances[typ], in)
	return true
}

func (e *TypeEnvironment) GetConformances(typ types.Type) []*types.Interface {
	direct := e.conformances[typ]

	s, ok := typ.(*types.Struct)
	if !ok || s.Generic == nil {
		return direct
	}

	result := make([]*types.Interface, len(direct), len(direct)+8)
	copy(result, direct)

	// Full generic impls (registered on the template)
	for _, in := range e.conformances[s.Generic] {
		if !e.implConstraintsSatisfied(s, in) {
			continue
		}

		instantiated := e.instantiations.Substitute(in, s.Substitutions).(*types.Interface)
		result = append(result, instantiated)
	}

	// Partial specializations (registered on instantiated targets of the template)
	for _, pt := range e.implTargets[s.Generic] {
		subs, ok := e.matchPartialTarget(s, pt)
		if !ok {
			continue
		}

		if impl := e.implForTarget[pt]; impl != nil && !e.implParamsSatisfied(impl, subs) {
			continue
		}

		for _, in := range e.conformances[pt] {
			instantiated := e.instantiations.Substitute(in, subs).(*types.Interface)
			result = append(result, instantiated)
		}
	}

	return result
}

func (e *TypeEnvironment) matchPartialTarget(s, pt *types.Struct) ([]types.Substitution, bool) {
	if s.Generic != pt.Generic || len(s.Substitutions) != len(pt.Substitutions) {
		return nil, false
	}

	var subs []types.Substitution

	for i, psub := range pt.Substitutions {
		arg := psub.Type
		concrete := s.Substitutions[i].Type

		if param, ok := arg.(*types.Param); ok {
			subs = append(subs, types.Substitution{Param: param, Type: concrete})
		} else if !arg.Equals(concrete) {
			return nil, false
		}
	}

	return subs, true
}

func (e *TypeEnvironment) implParamsSatisfied(impl *ast.Impl, subs []types.Substitution) bool {
	for _, param := range e.implParams[impl] {
		var concrete types.Type

		for _, sub := range subs {
			if sub.Param == param {
				concrete = sub.Type
				break
			}
		}

		if concrete == nil {
			continue
		}

		for _, constraint := range param.Constraints {
			if !e.satisfiesConstraint(concrete, constraint, subs) {
				return false
			}
		}
	}

	return true
}

func (e *TypeEnvironment) implConstraintsSatisfied(s *types.Struct, in *types.Interface) bool {
	template := in.AsImmutable()
	if template.Generic != nil {
		template = template.Generic
	}

	impl := e.implNodes[implKey{Type: s.Generic, Interface: template}]
	if impl == nil {
		return true
	}

	params := e.implParams[impl]
	if len(params) == 0 {
		return true
	}

	// Substitute every implementation type parameter with its concrete type argument.
	subs := make([]types.Substitution, 0, len(params))
	for i, param := range params {
		if i >= len(s.Substitutions) {
			break
		}

		subs = append(subs, types.Substitution{Param: param, Type: s.Substitutions[i].Type})
	}

	for i, param := range params {
		if i >= len(s.Substitutions) {
			break
		}

		for _, constraint := range param.Constraints {
			if !e.satisfiesConstraint(s.Substitutions[i].Type, constraint, subs) {
				return false
			}
		}
	}

	return true
}

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

func (e *TypeEnvironment) AddStaticMethod(typ types.Type, symbol symbols.Symbol) bool {
	for _, m := range e.static[typ] {
		if m.Name == symbol.Name {
			return false
		}
	}

	e.static[typ] = append(e.static[typ], symbol)
	return true
}

func (e *TypeEnvironment) AddInstanceMethod(typ types.Type, symbol symbols.Symbol) bool {
	for _, m := range e.instance[typ] {
		if m.Name == symbol.Name {
			return false
		}
	}

	e.instance[typ] = append(e.instance[typ], symbol)
	return true
}

func (e *TypeEnvironment) GetStaticMethod(typ types.Type, name string) (symbols.Symbol, bool) {
	for _, m := range e.static[typ] {
		if m.Name == name {
			return m, true
		}
	}

	return symbols.Symbol{}, false
}

func (e *TypeEnvironment) GetInstanceMethod(typ types.Type, name string) (symbols.Symbol, bool) {
	for _, m := range e.instance[typ] {
		if m.Name == name {
			return m, true
		}
	}

	return symbols.Symbol{}, false
}

func (e *TypeEnvironment) GetInstanceMethodWithSubs(typ types.Type, name string) (symbols.Symbol, []types.Substitution, bool) {
	if sym, ok := e.GetInstanceMethod(typ, name); ok {
		return sym, nil, true
	}

	s, ok := typ.(*types.Struct)
	if !ok || s.Generic == nil {
		return symbols.Symbol{}, nil, false
	}

	// Full generic impl: methods live on the canonical template.
	if sym, ok := e.GetInstanceMethod(s.Generic, name); ok {
		return sym, s.Substitutions, true
	}

	// Partial specializations.
	for _, pt := range e.implTargets[s.Generic] {
		subs, ok := e.matchPartialTarget(s, pt)
		if !ok {
			continue
		}

		if impl := e.implForTarget[pt]; impl != nil && !e.implParamsSatisfied(impl, subs) {
			continue
		}

		if sym, ok := e.GetInstanceMethod(pt, name); ok {
			return sym, subs, true
		}
	}

	return symbols.Symbol{}, nil, false
}

func (e *TypeEnvironment) GetStaticMethodWithSubs(typ types.Type, name string) (symbols.Symbol, []types.Substitution, bool) {
	if sym, ok := e.GetStaticMethod(typ, name); ok {
		return sym, nil, true
	}

	s, ok := typ.(*types.Struct)
	if !ok || s.Generic == nil {
		return symbols.Symbol{}, nil, false
	}

	if sym, ok := e.GetStaticMethod(s.Generic, name); ok {
		return sym, s.Substitutions, true
	}

	for _, pt := range e.implTargets[s.Generic] {
		subs, ok := e.matchPartialTarget(s, pt)
		if !ok {
			continue
		}

		if impl := e.implForTarget[pt]; impl != nil && !e.implParamsSatisfied(impl, subs) {
			continue
		}

		if sym, ok := e.GetStaticMethod(pt, name); ok {
			return sym, subs, true
		}
	}

	return symbols.Symbol{}, nil, false
}

func (e *TypeEnvironment) GetTypeScope(typ types.Type) symbols.Scope {
	// Constrained type parameter: scope is merged associated types and static methods from all constraints.
	if tp, ok := typ.(*types.Param); ok {
		if len(tp.Constraints) == 0 {
			return nil
		}

		if cached, ok := e.paramScopes[tp]; ok {
			return cached
		}

		var staticSymbols []symbols.Symbol

		for _, constraint := range tp.Constraints {
			canonical := constraint.AsImmutable()
			inNode := e.GetInterfaceNode(canonical)

			if inNode == nil {
				continue
			}

			// Associated types
			for _, associatedType := range canonical.AssociatedTypes {
				var a *ast.AssociatedType

				for _, an := range inNode.AssociatedTypes {
					if an.Name.Token.Text == associatedType.Name {
						a = an
						break
					}
				}

				if a == nil {
					continue
				}

				staticSymbols = append(staticSymbols, symbols.Symbol{
					Kind:   symbols.TypeParam,
					Public: true,
					Name:   associatedType.Name,
					Node:   a,
					Type:   associatedType,
				})
			}

			// Methods
			for _, method := range canonical.StaticMethods {
				var f *ast.Func

				for _, mf := range inNode.Methods {
					if mf.Name().Token.Text == method.Name {
						f = mf
						break
					}
				}

				if f == nil {
					continue
				}

				methodType := method.Type
				if canonical.SelfParam != nil {
					subs := []types.Substitution{{Param: canonical.SelfParam, Type: tp}}
					methodType = e.instantiations.Substitute(methodType, subs).(*types.Func)
				}

				staticSymbols = append(staticSymbols, symbols.Symbol{
					Kind:   symbols.Func,
					Public: true,
					Name:   method.Name,
					Node:   f,
					Type:   methodType,
				})
			}
		}

		var scope symbols.Scope
		if len(staticSymbols) > 0 {
			scope = symbols.SymbolScope(staticSymbols)
		}

		e.paramScopes[tp] = scope
		return scope
	}

	// Struct: scope is the registered static methods.
	methods := e.static[typ]
	if len(methods) == 0 {
		return nil
	}

	return symbols.SymbolScope(methods)
}
