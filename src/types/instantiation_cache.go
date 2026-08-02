package types

import (
	"fireball/core"
	"sync"
)

type InstantiationCache struct {
	types map[Type][]cacheEntry
	mu    sync.Mutex

	substituteDependentsQueue []Type
	substituteDependentTypes  bool
}

type cacheEntry struct {
	substitutions []Substitution
	typ           Type
}

type Substitution struct {
	Param *Param
	Type  Type
}

func NewInstantiationCache() *InstantiationCache {
	return &InstantiationCache{types: make(map[Type][]cacheEntry)}
}

// Get creates or retrieves the instantiation of a generic *Struct or *Func.
func (c *InstantiationCache) Get(generic Type, substitutions []Substitution) Type {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.get(generic, substitutions)
}

// Substitute resolves type params within typ using substitutions.
func (c *InstantiationCache) Substitute(typ Type, substitutions []Substitution) Type {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.resolve(typ, substitutions)
}

func (c *InstantiationCache) SubstituteDependentTypes() {
	defer core.Scope()()

	c.substituteDependentTypes = true

	for _, typ := range c.substituteDependentsQueue {
		c.substituteDependents(typ)
	}

	c.substituteDependentsQueue = nil
}

func (c *InstantiationCache) get(generic Type, substitutions []Substitution) Type {
	entries := c.types[generic]

	if isIdentitySubstitution(substitutions) {
		return generic
	}

	for _, entry := range entries {
		if substitutionsEquals(entry.substitutions, substitutions) {
			return entry.typ
		}
	}

	switch generic.(type) {
	case *Struct, *Interface, *Func:
		typ := c.substitute(generic, substitutions)
		c.types[generic] = append(entries, cacheEntry{substitutions: substitutions, typ: typ})
		return typ

	default:
		panic("types.InstantiationCache.get() - Invalid type to instantiate")
	}
}

func (c *InstantiationCache) resolve(typ Type, substitutions []Substitution) Type {
	switch typ := typ.(type) {
	case *Param:
		return getSubstitution(substitutions, typ)

	case *invalid, *Primitive:
		return typ

	case *Pointer:
		pointee := c.resolve(typ.Pointee, substitutions)
		if pointee == typ.Pointee {
			return typ
		}

		return &Pointer{
			Mutable: typ.Mutable,
			Pointee: pointee,
		}

	case *Array:
		element := c.resolve(typ.Element, substitutions)
		if element == typ.Element {
			return typ
		}

		return &Array{
			Size:    typ.Size,
			Element: element,
		}

	case *Enum:
		return typ

	case *Struct:
		if typ.Generic != nil {
			return c.getRemapped(typ.Generic, typ.Substitutions, substitutions)
		}
		if len(typ.TypeParams) > 0 {
			return c.get(typ, substitutions)
		}

		return typ

	case *Interface:
		canonical := typ.AsImmutable()
		result := canonical

		if canonical.Generic != nil {
			result = c.getRemapped(canonical.Generic, canonical.Substitutions, substitutions).(*Interface)
		} else if len(canonical.TypeParams) > 0 {
			result = c.get(canonical, substitutions).(*Interface)
		}

		if typ.Mutable {
			return result.AsMutable()
		}

		return result

	case *Func:
		if typ.Generic != nil {
			return c.getRemapped(typ.Generic, typ.Substitutions, substitutions)
		}
		if len(typ.TypeParams) > 0 {
			return c.get(typ, substitutions)
		}

		params := make([]Type, len(typ.Params))
		changed := false

		for i, param := range typ.Params {
			params[i] = c.resolve(param, substitutions)
			if params[i] != param {
				changed = true
			}
		}

		returns := c.resolve(typ.Returns, substitutions)
		if returns != typ.Returns {
			changed = true
		}

		if !changed {
			return typ
		}

		return &Func{
			TypeParams:    nil,
			Params:        params,
			VarArgs:       typ.VarArgs,
			Returns:       returns,
			Generic:       typ,
			Substitutions: substitutions,
		}

	default:
		panic("types.InstantiationCache.resolve() - Invalid type")
	}
}

func (c *InstantiationCache) substitute(generic Type, substitutions []Substitution) Type {
	switch generic := generic.(type) {
	case *Struct:
		s := &Struct{
			Name:          generic.Name,
			ModulePath:    generic.ModulePath,
			Packed:        generic.Packed,
			Fields:        nil,
			Generic:       generic,
			Substitutions: substitutions,
		}

		if c.substituteDependentTypes {
			c.substituteDependents(s)
		} else {
			c.substituteDependentsQueue = append(c.substituteDependentsQueue, s)
		}

		return s

	case *Interface:
		in := &Interface{
			Name:            generic.Name,
			ModulePath:      generic.ModulePath,
			SelfParam:       generic.SelfParam,
			AssociatedTypes: generic.AssociatedTypes,
			InstanceMethods: nil,
			StaticMethods:   nil,
			Generic:         generic,
			Substitutions:   substitutions,
		}

		if c.substituteDependentTypes {
			c.substituteDependents(in)
		} else {
			c.substituteDependentsQueue = append(c.substituteDependentsQueue, in)
		}

		return in

	case *Func:
		f := &Func{
			Params:        nil,
			VarArgs:       generic.VarArgs,
			Returns:       nil,
			Generic:       generic,
			Substitutions: substitutions,
		}

		if c.substituteDependentTypes {
			c.substituteDependents(f)
		} else {
			c.substituteDependentsQueue = append(c.substituteDependentsQueue, f)
		}

		return f

	default:
		panic("types.InstantiationCache.substitute() - Invalid generic type")
	}
}

func (c *InstantiationCache) substituteDependents(typ Type) {
	switch typ := typ.(type) {
	case *Struct:
		fields := make([]Field, len(typ.Generic.Fields))

		for i, field := range typ.Generic.Fields {
			fields[i] = Field{
				Name:   field.Name,
				Type:   c.resolve(field.Type, typ.Substitutions),
				Public: field.Public,
			}
		}

		typ.Fields = fields

	case *Interface:
		instanceMethods := make([]Method, len(typ.Generic.InstanceMethods))
		staticMethods := make([]Method, len(typ.Generic.StaticMethods))

		for i, m := range typ.Generic.InstanceMethods {
			instanceMethods[i] = Method{
				Name: m.Name,
				Type: c.resolve(m.Type, typ.Substitutions).(*Func),
			}
		}

		for i, m := range typ.Generic.StaticMethods {
			staticMethods[i] = Method{
				Name: m.Name,
				Type: c.resolve(m.Type, typ.Substitutions).(*Func),
			}
		}

		typ.InstanceMethods = instanceMethods
		typ.StaticMethods = staticMethods

		if v := typ.oppositeMutabilityVariant; v != nil {
			v.InstanceMethods = instanceMethods
			v.StaticMethods = staticMethods
		}

	case *Func:
		params := make([]Type, len(typ.Generic.Params))

		for i, param := range typ.Generic.Params {
			params[i] = c.resolve(param, typ.Substitutions)
		}

		typ.Params = params
		typ.Returns = c.resolve(typ.Generic.Returns, typ.Substitutions)

	default:
		panic("types.InstantiationCache.substituteDependents() - Invalid generic type")
	}
}

func (c *InstantiationCache) getRemapped(generic Type, stored []Substitution, outer []Substitution) Type {
	remapped := make([]Substitution, len(stored))

	for i, s := range stored {
		remapped[i] = Substitution{
			Param: s.Param,
			Type:  c.resolve(s.Type, outer),
		}
	}

	return c.get(generic, remapped)
}

func getSubstitution(substitutions []Substitution, param *Param) Type {
	for _, s := range substitutions {
		if s.Param == param {
			return s.Type
		}
	}

	return param
}

func isIdentitySubstitution(subs []Substitution) bool {
	for _, s := range subs {
		if p, ok := s.Type.(*Param); !ok || p != s.Param {
			return false
		}
	}

	return true
}

func substitutionsEquals(a, b []Substitution) bool {
	if len(a) != len(b) {
		return false
	}

	for i, s := range a {
		o := b[i]

		if s.Param != o.Param || !s.Type.Equals(o.Type) {
			return false
		}
	}

	return true
}
