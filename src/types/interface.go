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

	AssociatedTypes []*Param

	InstanceMethods []Method
	StaticMethods   []Method

	Generic       *Interface
	Substitutions []Substitution

	Mutable                   bool
	oppositeMutabilityVariant *Interface
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

func (i *Interface) AsMutable() *Interface {
	if i.Mutable {
		return i
	}

	if i.oppositeMutabilityVariant == nil {
		mut := *i
		mut.Mutable = true
		mut.oppositeMutabilityVariant = i
		i.oppositeMutabilityVariant = &mut
	}

	return i.oppositeMutabilityVariant
}

func (i *Interface) AsImmutable() *Interface {
	if !i.Mutable {
		return i
	}

	return i.oppositeMutabilityVariant
}

func (i *Interface) Equals(other Type) bool {
	o, ok := other.(*Interface)
	if !ok {
		return false
	}

	if i.Generic != nil && o.Generic != nil {
		return i.Generic == o.Generic && substitutionsEquals(i.Substitutions, o.Substitutions)
	}

	return i == o

}

func (i *Interface) String() string {
	prefix := ""
	if i.Mutable {
		prefix = "mut "
	}

	// Instantiation
	if i.Generic != nil {
		var sb strings.Builder

		sb.WriteString(prefix)
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

		sb.WriteString(prefix)
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
	return prefix + i.Name
}
