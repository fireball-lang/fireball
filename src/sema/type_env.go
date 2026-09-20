package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/fb-core"
	"fireball/symbols"
	"fireball/types"
	"iter"
	"slices"
)

type implKey struct {
	Type      types.Type
	Interface *types.Interface
}

// TypeEnvironment stores cross-file type information gathered during resolution
// and consumed by semantics and codegen.
//
// # Impl method storage
//
// A generic struct impl's methods and interface conformances are registered
// under a "target" type:
//
//   - Full generic impls (impl[K, V] ArrayMap[K, V]) register under the struct's
//     canonical *template*; their signatures reference the template's type params.
//   - Partially/full-specialized impls (impl[V] ArrayMap[String, V], impl ... of
//     a concrete type) register under an *instantiation* of the template whose
//     substitutions mix fixed types (String) with params. Lookups for a concrete
//     type (e.g. ArrayMap[String, bool]) therefore search: the exact type, the
//     template, then every indexed partial target that unifies with it.
//
// Constraint satisfaction is implemented in constraints.go.
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
	builtins       fb_core.Builtins
}

func NewTypeEnvironment(instantiations *types.InstantiationCache, builtins fb_core.Builtins) *TypeEnvironment {
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
		builtins:       builtins,
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
	// Compute Zeroable
	var result []*types.Interface

	switch t := typ.(type) {
	case *types.Primitive:
		result = []*types.Interface{e.builtins.Zeroable}

	case *types.Array:
		if slices.Contains(e.GetConformances(t.Element), e.builtins.Zeroable) {
			return []*types.Interface{e.builtins.Zeroable}
		}

		return nil

	case *types.Reference:
		typ = t.Pointee

	case *types.Pointer:
		result = []*types.Interface{e.builtins.Zeroable}
		typ = t.Pointee

	case *types.Func:
		return []*types.Interface{}

	case *types.Enum:
		result = []*types.Interface{e.builtins.Zeroable}

	case *types.Struct:
		var zeroable bool

		if t.Layout == types.Union {
			zeroable = false

			for _, field := range t.Fields {
				if !field.Required && slices.Contains(e.GetConformances(field.Type), e.builtins.Zeroable) {
					zeroable = true
					break
				}
			}
		} else {
			zeroable = true

			for _, field := range t.Fields {
				if field.Required || !slices.Contains(e.GetConformances(field.Type), e.builtins.Zeroable) {
					zeroable = false
					break
				}
			}
		}

		if zeroable {
			result = []*types.Interface{e.builtins.Zeroable}
		}

	case *types.Interface:
		return []*types.Interface{}

	case *types.Param:
		return t.Constraints
	}

	// Check direct conformances
	direct := e.conformances[typ]

	s, ok := typ.(*types.Struct)
	if !ok || s.Generic == nil {
		if result == nil {
			return direct
		}

		return append(result, direct...)
	}

	if result == nil {
		result = make([]*types.Interface, len(direct), len(direct)+8)
		copy(result, direct)
	} else {
		result = append(result, direct...)
	}

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

// implParamsSatisfied checks that every impl type parameter constraint holds for
// the concrete type it is substituted with by `subs`.
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

	return e.implParamsSatisfied(impl, subs)
}

func (e *TypeEnvironment) AddAssociatedConst(typ types.Type, symbol symbols.Symbol) bool {
	for _, m := range e.static[typ] {
		if m.Name == symbol.Name {
			return false
		}
	}

	e.static[typ] = append(e.static[typ], symbol)
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

func (e *TypeEnvironment) GetAssociatedConst(typ types.Type, name string) (symbols.Symbol, bool) {
	for _, symbol := range e.static[typ] {
		if symbol.Kind == symbols.AssociatedConst && symbol.Name == name {
			return symbol, true
		}
	}

	return symbols.Symbol{}, false
}

func (e *TypeEnvironment) GetStaticMethod(typ types.Type, name string) (symbols.Symbol, bool) {
	for _, symbol := range e.static[typ] {
		if symbol.Kind == symbols.Func && symbol.Name == name {
			return symbol, true
		}
	}

	return symbols.Symbol{}, false
}

func (e *TypeEnvironment) GetInstanceMethod(typ types.Type, name string) (symbols.Symbol, bool) {
	for _, symbol := range e.instance[typ] {
		if symbol.Kind == symbols.Func && symbol.Name == name {
			return symbol, true
		}
	}

	return symbols.Symbol{}, false
}

func (e *TypeEnvironment) GetAssociatedConstWithSubs(typ types.Type, name string) (symbols.Symbol, []types.Substitution, bool) {
	return e.getSymbolWithSubs(e.GetAssociatedConst, typ, name)
}

func (e *TypeEnvironment) GetInstanceMethodWithSubs(typ types.Type, name string) (symbols.Symbol, []types.Substitution, bool) {
	return e.getSymbolWithSubs(e.GetInstanceMethod, typ, name)
}

func (e *TypeEnvironment) GetStaticMethodWithSubs(typ types.Type, name string) (symbols.Symbol, []types.Substitution, bool) {
	return e.getSymbolWithSubs(e.GetStaticMethod, typ, name)
}

// getSymbolWithSubs resolves a symbol on `typ`, which may be a concrete
// instantiation of a generic struct. It returns the method symbol together with
// the substitutions needed to instantiate its type for `typ`. Lookup order:
// exact registration, methods on the canonical template (full generic impls),
// then partial specializations that unify with `typ`.
func (e *TypeEnvironment) getSymbolWithSubs(get func(types.Type, string) (symbols.Symbol, bool), typ types.Type, name string) (symbols.Symbol, []types.Substitution, bool) {
	if sym, ok := get(typ, name); ok {
		return sym, nil, true
	}

	s, ok := typ.(*types.Struct)
	if !ok || s.Generic == nil {
		return symbols.Symbol{}, nil, false
	}

	// Full generic impl: methods live on the canonical template.
	if sym, ok := get(s.Generic, name); ok {
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

		if sym, ok := get(pt, name); ok {
			return sym, subs, true
		}
	}

	return symbols.Symbol{}, nil, false
}

type MemberSymbol struct {
	Symbol symbols.Symbol
	Type   types.Type
}

type memberWithSubs struct {
	symbol symbols.Symbol
	subs   []types.Substitution
}

func (e *TypeEnvironment) InstanceMembers(typ types.Type) iter.Seq[MemberSymbol] {
	return e.resolveMembers(e.instance, typ)
}

func (e *TypeEnvironment) StaticMembers(typ types.Type) iter.Seq[MemberSymbol] {
	return e.resolveMembers(e.static, typ)
}

func (e *TypeEnvironment) ParamInstanceMembers(tp *types.Param) iter.Seq[MemberSymbol] {
	if tp == nil || len(tp.Constraints) == 0 {
		return func(yield func(MemberSymbol) bool) {}
	}

	return func(yield func(MemberSymbol) bool) {
		seen := make(map[string]struct{})

		for _, constraint := range tp.Constraints {
			canonical := constraint.AsImmutable()

			inNode := e.GetInterfaceNode(canonical)
			if inNode == nil {
				continue
			}

			for _, method := range canonical.InstanceMethods {
				if _, ok := seen[method.Name]; ok {
					continue
				}

				seen[method.Name] = struct{}{}

				// Method AST node
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

				// Substitution
				methodType := method.Type

				if canonical.SelfParam != nil {
					subs := []types.Substitution{{Param: canonical.SelfParam, Type: tp}}
					methodType = e.instantiations.Substitute(methodType, subs).(*types.Func)
				}

				symbol := symbols.Symbol{
					Kind:   symbols.Func,
					Public: true,
					Name:   method.Name,
					Node:   f,
					Type:   method.Type,
				}

				if !yield(MemberSymbol{Symbol: symbol, Type: methodType}) {
					return
				}
			}
		}
	}
}

func (e *TypeEnvironment) resolveMembers(m map[types.Type][]symbols.Symbol, typ types.Type) iter.Seq[MemberSymbol] {
	return func(yield func(MemberSymbol) bool) {
		seen := make(map[string]struct{}, len(m[typ]))

		for entry := range e.enumerateMembers(m, typ) {
			if _, ok := seen[entry.symbol.Name]; ok {
				continue
			}

			seen[entry.symbol.Name] = struct{}{}

			member := MemberSymbol{Symbol: entry.symbol, Type: entry.symbol.Type}

			if len(entry.subs) > 0 {
				member.Type = e.instantiations.Get(entry.symbol.Type, entry.subs)
			}

			if !yield(member) {
				return
			}
		}
	}
}

func (e *TypeEnvironment) enumerateMembers(m map[types.Type][]symbols.Symbol, typ types.Type) iter.Seq[memberWithSubs] {
	return func(yield func(memberWithSubs) bool) {
		add := func(syms []symbols.Symbol, subs []types.Substitution) bool {
			for _, sym := range syms {
				if !yield(memberWithSubs{symbol: sym, subs: subs}) {
					return false
				}
			}

			return true
		}

		// Exact registration
		if !add(m[typ], nil) {
			return
		}

		s, ok := typ.(*types.Struct)
		if !ok || s.Generic == nil {
			return
		}

		// Full generic impl: methods live on the canonical template.
		if !add(m[s.Generic], s.Substitutions) {
			return
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

			if !add(m[pt], subs) {
				return
			}
		}
	}
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

			// Associated constants
			for _, assocConst := range canonical.AssociatedConsts {
				var a *ast.AssociatedConst

				for _, node := range inNode.AssociatedConsts {
					if node.Name.Token.Text == assocConst.Name {
						a = node
						break
					}
				}

				if a == nil {
					continue
				}

				constType := assocConst.Type
				if canonical.SelfParam != nil {
					subs := []types.Substitution{{Param: canonical.SelfParam, Type: tp}}
					constType = e.instantiations.Substitute(constType, subs)
				}

				staticSymbols = append(staticSymbols, symbols.Symbol{
					Kind:   symbols.AssociatedConst,
					Public: true,
					Name:   assocConst.Name,
					Node:   a,
					Type:   constType,
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
