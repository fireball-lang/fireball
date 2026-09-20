package types

import (
	"cmp"
	"slices"
	"strings"
)

type Layout uint8

const (
	Fireball Layout = iota
	C
	Union
)

type Field struct {
	Name string
	Type Type

	Public   bool
	Required bool
}

type Struct struct {
	Name       string
	ModulePath []string
	Layout     Layout
	Packed     bool
	TypeParams []*Param
	Fields     []Field

	Generic       *Struct
	Substitutions []Substitution
}

func (s *Struct) Optimize() {
	if s.Layout == C {
		return
	}

	slices.SortFunc(s.Fields, func(a, b Field) int {
		return cmp.Compare(a.Name, b.Name)
	})
}

func (s *Struct) Field(name string) *Field {
	if s.Layout == C {
		for i := range s.Fields {
			field := &s.Fields[i]
			if field.Name == name {
				return field
			}
		}
	}

	index, ok := slices.BinarySearchFunc(s.Fields, name, func(field Field, s string) int {
		return cmp.Compare(field.Name, s)
	})

	if ok {
		return &s.Fields[index]
	}

	return nil
}

func (s *Struct) Equals(other Type) bool {
	o, ok := other.(*Struct)
	if !ok {
		return false
	}

	if s.Generic != nil && o.Generic != nil {
		return s.Generic == o.Generic && substitutionsEquals(s.Substitutions, o.Substitutions)
	}

	return s == o
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
