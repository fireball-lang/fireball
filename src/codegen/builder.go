package codegen

import (
	"fireball/ir"
	"fireball/types"
)

// Array Builder

type ArrayBuilder struct {
	c   *Codegen
	typ *types.Array

	values []ir.Value
}

func (c *Codegen) Array(a *types.Array) ArrayBuilder {
	return ArrayBuilder{
		c:   c,
		typ: a,
	}
}

func (a *ArrayBuilder) Add(value ir.Value) {
	a.values = append(a.values, value)
}

func (a *ArrayBuilder) Build() ir.Value {
	typ := a.c.Types.Get(a.typ).(*ir.ArrayType)

	if len(a.values) == 0 {
		return &ir.ZeroInitializer{Typ: typ}
	}

	// Constants
	constantCount := uint32(0)

	for _, value := range a.values {
		if ir.IsConstant(value) {
			constantCount++
		}
	}

	init := &ir.Array{}

	if constantCount > 0 {
		init.Elements = make([]ir.Value, a.typ.Size)
	}

	for i, value := range a.values {
		if ir.IsConstant(value) {
			init.Elements[i] = value
		}
	}

	if constantCount == a.typ.Size {
		return init
	}

	// Runtime
	if a.c.fun == nil {
		panic("codegen.arrayBuilder.Builder() - Tried to build an array outside of a function with runtime values")
	}

	var arrayValue ir.Value

	if constantCount == 0 {
		arrayValue = &ir.ZeroInitializer{Typ: typ}
	} else {
		for i := uint32(0); i < a.typ.Size; i++ {
			if init.Elements[i] == nil {
				init.Elements[i] = &ir.ZeroInitializer{Typ: typ.Element}
			}
		}

		arrayValue = init
	}

	for i, value := range a.values {
		if !ir.IsConstant(value) {
			arrayValue = a.c.Emitter.InsertValue(arrayValue, value, uint32(i))
		}
	}

	return arrayValue
}

// Struct Builder

type StructBuilder struct {
	c   *Codegen
	typ *types.Struct

	fields map[string]ir.Value
}

func (c *Codegen) Struct(s *types.Struct) StructBuilder {
	return StructBuilder{
		c:      c,
		typ:    s,
		fields: make(map[string]ir.Value),
	}
}

func (s *StructBuilder) Set(name string, value ir.Value) {
	s.fields[name] = value
}

func (s *StructBuilder) Build() ir.Value {
	typ := s.c.Types.Get(s.typ).(ir.StructLikeType)

	if len(s.fields) == 0 {
		return &ir.ZeroInitializer{Typ: typ}
	}

	// Constants
	constantCount := 0

	for _, value := range s.fields {
		if ir.IsConstant(value) {
			constantCount++
		}
	}

	var fields []ir.Value
	if constantCount > 0 {
		if s.typ.Layout == types.LayoutUnion {
			fields = make([]ir.Value, 1)
		} else {
			fields = make([]ir.Value, len(s.typ.Fields))
		}
	}

	init := &ir.Struct{
		Typ:    typ,
		Fields: fields,
	}

	for name, value := range s.fields {
		if ir.IsConstant(value) {
			var i int

			if s.typ.Layout == types.LayoutUnion {
				i = 0
			} else {
				_, i = typ.Field(name)
				if i < 0 {
					panic("codegen.structBuilder.Build() - Failed to find field '" + name + "' on type '" + s.typ.String() + "'")
				}
			}

			init.Fields[i] = value
		}
	}

	if constantCount == len(s.typ.Fields) {
		return init
	}

	// Runtime
	if s.c.fun == nil {
		panic("codegen.structBuilder.Builder() - Tried to build a struct outside of a function with runtime values")
	}

	var structValue ir.Value

	if constantCount == 0 {
		structValue = &ir.ZeroInitializer{Typ: typ}
	} else {
		if s.typ.Layout != types.LayoutUnion {
			for _, field := range s.typ.Fields {
				if value, ok := s.fields[field.Name]; !ok || !ir.IsConstant(value) {
					fieldTyp, i := typ.Field(field.Name)
					init.Fields[i] = &ir.ZeroInitializer{Typ: fieldTyp}
				}
			}
		}

		structValue = init
	}

	for name, value := range s.fields {
		if !ir.IsConstant(value) {
			var i int

			if s.typ.Layout == types.LayoutUnion {
				i = 0
			} else {
				_, i = typ.Field(name)
				if i < 0 {
					panic("codegen.structBuilder.Build() - Failed to find field '" + name + "' on type '" + s.typ.String() + "'")
				}
			}

			structValue = s.c.Emitter.InsertValue(structValue, value, uint32(i))
		}
	}

	return structValue
}

// Global Var

func (c *Codegen) GlobalVar(name string, flags ir.GlobalVarFlags, value ir.Value) *ir.GlobalVar {
	gVar := c.Module.NewGlobalVar(name, value.Type())
	gVar.Flags = flags
	gVar.Initializer = value

	return gVar
}
