package types

import "strings"

type Method struct {
	Name string
	Type *Func
}

type Interface struct {
	Name       string
	ModulePath []string
	TypeParams []*Param

	SelfParam *Param

	InstanceMethods []Method
	StaticMethods   []Method

	Generic       *Interface
	Substitutions []Substitution
}

var interfaceUnderlying = &Struct{
	Name: "__interface",
	Fields: []Field{
		{
			Name: "data",
			Type: &Pointer{
				Mutable: true,
				Pointee: PrimitiveVoid,
			},
			Public: true,
		},
		{
			Name: "vtable",
			Type: &Pointer{
				Mutable: false,
				Pointee: PrimitiveVoid,
			},
			Public: true,
		},
	},
}

func (i *Interface) Underlying() Type {
	return interfaceUnderlying
}

func (i *Interface) Equals(other Type) bool {
	return i == other
}

func (i *Interface) String() string {
	// Instantiation
	if i.Generic != nil {
		var sb strings.Builder

		sb.WriteString(i.Generic.Name)
		sb.WriteRune('[')

		for i, sub := range i.Substitutions {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')

		return sb.String()
	}

	// Generic template
	if len(i.TypeParams) > 0 {
		var sb strings.Builder

		sb.WriteString(i.Name)
		sb.WriteRune('[')

		for i, param := range i.TypeParams {
			if i > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(param.Name)
		}

		sb.WriteRune(']')

		return sb.String()
	}

	// Non-generic
	return i.Name
}
