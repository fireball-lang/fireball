package types

type Type interface {
	Equals(other Type) bool
	String() string
}

type Composed interface {
	Type

	Underlying() Type
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

	case *Struct:
		if len(typ.TypeParams) > 0 {
			return true
		}

		for _, field := range typ.Fields {
			if hasParam(field.Type, seen) {
				return true
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
