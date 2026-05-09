package types

import "strings"

type Func struct {
	TypeParams []*Param

	Params  []Type
	VarArgs bool

	Returns Type

	Generic       *Func
	Substitutions []Substitution
}

var funcType = &Pointer{Pointee: PrimitiveVoid}

func (f *Func) Underlying() Type {
	return funcType
}

func (f *Func) Equals(other Type) bool {
	if other, ok := other.(*Func); ok {
		return typeSliceEquals(f.Params, other.Params) && f.VarArgs == other.VarArgs && f.Returns.Equals(other.Returns)
	}

	return false
}

func (f *Func) String() string {
	var sb strings.Builder

	// Header
	sb.WriteString("func")

	// Substitutions
	if f.Generic != nil {
		// Instantiation
		sb.WriteRune('[')

		for i, sub := range f.Substitutions {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')
	} else if len(f.TypeParams) > 0 {
		// Generic template
		sb.WriteRune('[')

		for i, param := range f.TypeParams {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(param.Name)
		}

		sb.WriteRune(']')
	}

	// Parameters
	sb.WriteRune('(')

	for i, param := range f.Params {
		if i > 0 {
			sb.WriteString(", ")
		}

		sb.WriteString(param.String())
	}

	sb.WriteRune(')')

	// Returns
	if !f.Returns.Equals(PrimitiveVoid) {
		sb.WriteRune(' ')
		sb.WriteString(f.Returns.String())
	}

	return sb.String()
}
