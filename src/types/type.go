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

	for i := 0; i < len(a); i++ {
		if !a[i].Equals(b[i]) {
			return false
		}
	}

	return true
}
