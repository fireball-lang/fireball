package types

import "strings"

type Field struct {
	Name   string
	Type   Type
	Public bool
}

type Struct struct {
	Name       string
	ModulePath []string
	Packed     bool
	TypeParams []*Param
	Fields     []Field

	Generic       *Struct
	Substitutions []Substitution
}

func (s *Struct) Field(name string) (Field, int) {
	for i, field := range s.Fields {
		if field.Name == name {
			return field, i
		}
	}

	return Field{}, -1
}

func (s *Struct) Equals(other Type) bool {
	return s == other
}

func (s *Struct) String() string {
	// Instantiation
	if s.Generic != nil {
		var sb strings.Builder

		sb.WriteString(s.Generic.Name)
		sb.WriteRune('[')

		for i, sub := range s.Substitutions {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')

		return sb.String()
	}

	// Generic template
	if len(s.TypeParams) > 0 {
		var sb strings.Builder

		sb.WriteString(s.Name)
		sb.WriteRune('[')

		for i, param := range s.TypeParams {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(param.Name)
		}

		sb.WriteRune(']')

		return sb.String()
	}

	// Non-generic
	return s.Name
}
