package types

type Type interface {
	Equals(other Type) bool
	String() string
}

type Generic interface {
	Type

	GetTypeParams() []*Param

	GetGeneric() Generic
	GetSubstitutions() []Substitution
}

type Composed interface {
	Type

	Underlying() Type
}

func IsNonValue(typ Type) bool {
	switch typ := typ.(type) {
	case *Never:
		return true

	case *Primitive:
		return typ.Kind == Void

	default:
		return false
	}
}

func typeSliceEquals(a, b []Type) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !a[i].Equals(b[i]) {
			return false
		}
	}

	return true
}

// HasParam reports whether typ references any unresolved type parameter
func HasParam(typ Type) bool {
	return hasParam(typ, make(map[Type]bool))
}

func hasParam(typ Type, seen map[Type]bool) bool {
	if seen[typ] {
		return false
	}
	seen[typ] = true

	switch typ := typ.(type) {
	case *Param:
		return true

	case *Pointer:
		return hasParam(typ.Pointee, seen)

	case *Array:
		return hasParam(typ.Element, seen)

	case *Tuple:
		for _, typ := range typ.Types {
			if hasParam(typ, seen) {
				return true
			}
		}

	case *Alias:
		if len(typ.TypeParams) > 0 {
			return true
		}

		return hasParam(typ.Type, seen)

	case *Struct:
		if len(typ.TypeParams) > 0 {
			return true
		}

		for _, field := range typ.Fields {
			if hasParam(field.Type, seen) {
				return true
			}
		}

	case *Union:
		if len(typ.TypeParams) > 0 {
			return true
		}

		for _, cas := range typ.Cases {
			for _, field := range cas.Fields {
				if hasParam(field.Type, seen) {
					return true
				}
			}
		}

	case *Func:
		if len(typ.TypeParams) > 0 {
			return true
		}

		for _, param := range typ.Params {
			if hasParam(param, seen) {
				return true
			}
		}

		return hasParam(typ.Returns, seen)
	}

	return false
}
