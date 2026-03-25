package types

type Func struct {
	Params  []Type
	VarArgs bool

	Returns Type
}

func (f *Func) Equals(other Type) bool {
	if other, ok := other.(*Func); ok {
		return typeSliceEquals(f.Params, other.Params) && f.VarArgs == other.VarArgs && f.Returns.Equals(other.Returns)
	}

	return false
}
