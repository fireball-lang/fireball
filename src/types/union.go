package types

import (
	"math"
	"strings"
)

type Union struct {
	Name       string
	ModulePath []string
	TypeParams []*Param
	Cases      []*UnionCase

	Generic       *Union
	Substitutions []Substitution

	underlying *Struct
}

type UnionCase struct {
	Union *Union
	Index uint32

	Name   string
	Fields []UnionCaseField
}

func (u *UnionCase) Field(name string) *UnionCaseField {
	for i := 0; i < len(u.Fields); i++ {
		field := &u.Fields[i]

		if field.Name == name {
			return field
		}
	}

	return nil
}

func (u *UnionCase) PayloadType() Type {
	switch len(u.Fields) {
	case 0:
		return nil

	case 1:
		return u.Fields[0].Type

	default:
		underlying := u.Union.Underlying().(*Struct)

		dataField := underlying.Field("data")
		if dataField == nil {
			panic("types.UnionCase.PayloadType() - Failed to find 'data' underlying field")
		}

		data := dataField.Type.(*Struct)

		caseField := data.Field(u.Name)
		if caseField == nil {
			panic("types.UnionCase.PayloadType() - Failed to find '" + u.Name + "' underlying case field")
		}

		return caseField.Type
	}
}

type UnionCaseField struct {
	Name string
	Type Type
}

func (u *Union) Case(name string) *UnionCase {
	for _, c := range u.Cases {
		if c.Name == name {
			return c
		}
	}

	return nil
}

func (u *Union) GetTypeParams() []*Param {
	return u.TypeParams
}

func (u *Union) GetGeneric() Generic {
	if u.Generic == nil {
		return nil
	}

	return u.Generic
}

func (u *Union) GetSubstitutions() []Substitution {
	return u.Substitutions
}

func (u *Union) Underlying() Type {
	if u.underlying != nil {
		return u.underlying
	}

	dataFields := make([]Field, len(u.Cases))

	for i, case_ := range u.Cases {
		// Empty
		if len(case_.Fields) == 0 {
			dataFields[i] = Field{
				Name: case_.Name,
				Type: PrimitiveU8,
			}

			continue
		}

		// Single field
		if len(case_.Fields) == 1 {
			dataFields[i] = Field{
				Name:   case_.Name,
				Type:   case_.Fields[0].Type,
				Public: true,
			}

			continue
		}

		// Multiple fields
		fields := make([]Field, len(case_.Fields))

		for j, field := range case_.Fields {
			fields[j] = Field{
				Name:   field.Name,
				Type:   field.Type,
				Public: true,
			}
		}

		s := &Struct{
			Layout: LayoutFireball,
			Fields: fields,
		}
		s.Optimize()

		dataFields[i] = Field{
			Name:   case_.Name,
			Type:   s,
			Public: true,
		}
	}

	dataStruct := &Struct{
		Layout: LayoutUnion,
		Fields: dataFields,
	}
	dataStruct.Optimize()

	u.underlying = &Struct{
		Name:       u.Name,
		ModulePath: u.ModulePath,
		Layout:     LayoutFireball,
		TypeParams: u.TypeParams,
		Fields: []Field{
			{
				Name:   "tag",
				Type:   u.TagType(),
				Public: true,
			},
			{
				Name:   "data",
				Type:   dataStruct,
				Public: true,
			},
		},
		Substitutions: u.Substitutions,
	}
	u.underlying.Optimize()

	if u.Generic != nil {
		u.underlying.Generic = u.Generic.Underlying().(*Struct)
	}

	return u.underlying
}

func (u *Union) TagType() Type {
	c := len(u.Cases)

	if c <= math.MaxUint8+1 {
		return PrimitiveU8
	}
	if c <= math.MaxUint16+1 {
		return PrimitiveU16
	}
	if c <= math.MaxUint32+1 {
		return PrimitiveU32
	}
	return PrimitiveU64
}

func (u *Union) Equals(other Type) bool {
	o, ok := other.(*Union)
	if !ok {
		return false
	}

	if u.Generic != nil && o.Generic != nil {
		return u.Generic == o.Generic && substitutionsEquals(u.Substitutions, o.Substitutions)
	}

	return u == o
}

func (u *Union) String() string {
	// Instantiation
	if u.Generic != nil {
		var sb strings.Builder

		sb.WriteString(u.Generic.Name)
		sb.WriteRune('[')

		for i, sub := range u.Substitutions {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')

		return sb.String()
	}

	// Generic template
	if len(u.TypeParams) > 0 {
		var sb strings.Builder

		sb.WriteString(u.Name)
		sb.WriteRune('[')

		for i, param := range u.TypeParams {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(param.Name)
		}

		sb.WriteRune(']')

		return sb.String()
	}

	// Non-generic
	return u.Name
}
