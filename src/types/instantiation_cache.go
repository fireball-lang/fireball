package types

type InstantiationCache struct {
	types map[Type][]cacheEntry
}

type cacheEntry struct {
	substitutions []Substitution
	typ           Type
}

type Substitution struct {
	Param *Param
	Type  Type
}

func NewInstantiationCache() InstantiationCache {
	return InstantiationCache{types: make(map[Type][]cacheEntry)}
}

// Get creates or retrieves the instantiation of a generic *Struct or *Func.
func (c InstantiationCache) Get(generic Type, substitutions []Substitution) Type {
	return c.get(generic, substitutions)
}

// Substitute resolves type params within typ using substitutions.
func (c InstantiationCache) Substitute(typ Type, substitutions []Substitution) Type {
	return c.resolve(typ, substitutions)
}

func (c InstantiationCache) get(generic Type, substitutions []Substitution) Type {
	entries := c.types[generic]

	for _, entry := range entries {
		if substitutionsEquals(entry.substitutions, substitutions) {
			return entry.typ
		}
	}

	switch generic.(type) {
	case *Struct, *Func:
		typ := c.substitute(generic, substitutions)
		c.types[generic] = append(entries, cacheEntry{substitutions: substitutions, typ: typ})
		return typ

	default:
		panic("types.InstantiationCache.get() - Invalid type to instantiate")
	}
}

func (c InstantiationCache) resolve(typ Type, substitutions []Substitution) Type {
	switch typ := typ.(type) {
	case *Param:
		return getSubstitution(substitutions, typ)

	case *invalid:
		return typ

	case *Primitive:
		return typ

	case *Pointer:
		return &Pointer{
			Mutable: typ.Mutable,
			Pointee: c.resolve(typ.Pointee, substitutions),
		}

	case *Array:
		return &Array{
			Size:    typ.Size,
			Element: c.resolve(typ.Element, substitutions),
		}

	case *Struct:
		if typ.Generic != nil {
			return c.getRemapped(typ.Generic, typ.Substitutions, substitutions)
		}
		if len(typ.TypeParams) > 0 {
			return c.get(typ, substitutions)
		}

		return typ

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

func (c InstantiationCache) substitute(generic Type, substitutions []Substitution) Type {
	switch generic := generic.(type) {
	case *Struct:
		fields := make([]Field, len(generic.Fields))

		for i, field := range generic.Fields {
			fields[i] = Field{
				Name:   field.Name,
				Type:   c.resolve(field.Type, substitutions),
				Public: field.Public,
			}
		}

		return &Struct{
			Name:          generic.Name,
			ModulePath:    generic.ModulePath,
			Packed:        generic.Packed,
			Fields:        fields,
			Generic:       generic,
			Substitutions: substitutions,
		}

	case *Func:
		params := make([]Type, len(generic.Params))

		for i, param := range generic.Params {
			params[i] = c.resolve(param, substitutions)
		}

		return &Func{
			TypeParams:    nil,
			Params:        params,
			VarArgs:       generic.VarArgs,
			Returns:       c.resolve(generic.Returns, substitutions),
			Generic:       generic,
			Substitutions: substitutions,
		}

	default:
		panic("types.InstantiationCache.substitute() - Invalid generic type")
	}
}

func (c InstantiationCache) getRemapped(generic Type, stored []Substitution, outer []Substitution) Type {
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
