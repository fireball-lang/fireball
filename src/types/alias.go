package types

import "strings"

type Alias struct {
	Name       string
	ModulePath []string
	TypeParams []*Param

	Type Type

	Generic       *Alias
	Substitutions []Substitution
}

func (a *Alias) Underlying() Type {
	return a.Type
}

func (a *Alias) Equals(other Type) bool {
	o, ok := other.(*Alias)
	if !ok {
		return false
	}

	if a.Generic != nil && o.Generic != nil {
		return a.Generic == o.Generic && substitutionsEquals(a.Substitutions, o.Substitutions)
	}

	return a == o
}

func (a *Alias) String() string {
	// Instantiation
	if a.Generic != nil {
		var sb strings.Builder

		sb.WriteString(a.Generic.Name)
		sb.WriteRune('[')

		for i, sub := range a.Substitutions {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')

		return sb.String()
	}

	// Generic template
	if len(a.TypeParams) > 0 {
		var sb strings.Builder

		sb.WriteString(a.Name)
		sb.WriteRune('[')

		for i, param := range a.TypeParams {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(param.Name)
		}

		sb.WriteRune(']')

		return sb.String()
	}

	// Non-generic
	return a.Name
}
