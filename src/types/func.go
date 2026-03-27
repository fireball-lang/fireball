package types

import "strings"

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

func (f *Func) String() string {
	var sb strings.Builder

	sb.WriteString("func(")

	for i, param := range f.Params {
		if i > 0 {
			sb.WriteString(", ")
		}

		sb.WriteString(param.String())
	}

	sb.WriteRune(')')

	if !f.Returns.Equals(PrimitiveVoid) {
		sb.WriteRune(' ')
		sb.WriteString(f.Returns.String())
	}

	return sb.String()
}
