package codegen

import (
	"fireball/core"
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

	// Union
	if s.typ.Layout == types.LayoutUnion {
		if len(s.fields) != 1 {
			panic("codegen.StructBuilder.Build() - Expected one union field")
		}

		var fields []ir.Field
		switch t := typ.(type) {
		case *ir.StructType:
			fields = t.Fields
		case *ir.RefStructType:
			fields = t.Struct.Fields
		}

		for _, storage := range s.fields {
			if ir.IsConstant(storage) {
				return &ir.Struct{
					Typ: typ,
					Fields: []ir.Value{
						&ir.ZeroInitializer{Typ: fields[0].Type},
						storage,
					},
				}
			}

			if s.c.fun == nil {
				panic("codegen.StructBuilder.Build() - Runtime union outside function")
			}

			return s.c.Emitter.InsertValue(
				&ir.ZeroInitializer{Typ: typ},
				storage,
				1,
			)
		}
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
		fields = make([]ir.Value, len(s.typ.Fields))
	}

	init := &ir.Struct{
		Typ:    typ,
		Fields: fields,
	}

	for name, value := range s.fields {
		if ir.IsConstant(value) {
			_, i := typ.Field(name)
			if i < 0 {
				panic("codegen.structBuilder.Build() - Failed to find field '" + name + "' on type '" + s.typ.String() + "'")
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
		for _, field := range s.typ.Fields {
			if value, ok := s.fields[field.Name]; !ok || !ir.IsConstant(value) {
				fieldTyp, i := typ.Field(field.Name)
				init.Fields[i] = &ir.ZeroInitializer{Typ: fieldTyp}
			}
		}

		structValue = init
	}

	for name, value := range s.fields {
		if !ir.IsConstant(value) {
			_, i := typ.Field(name)
			if i < 0 {
				panic("codegen.structBuilder.Build() - Failed to find field '" + name + "' on type '" + s.typ.String() + "'")
			}

			structValue = s.c.Emitter.InsertValue(structValue, value, uint32(i))
		}
	}

	return structValue
}

// Union Builder

type UnionBuilder struct {
	c      *Codegen
	cas    *types.UnionCase
	fields map[string]ir.Value
}

func (c *Codegen) Union(cas *types.UnionCase) UnionBuilder {
	return UnionBuilder{
		c:   c,
		cas: cas,
	}
}

func (u *UnionBuilder) Set(name string, value ir.Value) {
	if u.fields == nil {
		u.fields = make(map[string]ir.Value)
	}

	u.fields[name] = value
}

func (u *UnionBuilder) Build() ir.Value {
	c, cas := u.c, u.cas
	typ := cas.Union

	underlying := typ.Underlying().(*types.Struct)

	dataField := underlying.Field("data")
	if dataField == nil {
		panic("codegen.UnionBuilder.Build() - Failed to find 'data' field")
	}

	dataType := dataField.Type.(*types.Struct)
	data := c.Struct(dataType)

	if len(cas.Fields) > 0 {
		var payload ir.Value

		if len(cas.Fields) == 1 {
			payload = u.fields[cas.Fields[0].Name]
		} else {
			builder := c.Struct(cas.PayloadType().(*types.Struct))

			for name, value := range u.fields {
				builder.Set(name, value)
			}

			payload = builder.Build()
		}

		dataIR := c.Types.Get(dataType).(*ir.StructType)
		data.Set(cas.Name, c.BitCast(payload, dataIR.Fields[1].Type))
	}

	tag := &ir.Integer{
		Typ:   c.Types.Get(typ.TagType()),
		Value: core.Unsigned(false, uint64(cas.Index)),
	}

	result := c.Struct(underlying)

	result.Set("tag", tag)
	result.Set("data", data.Build())

	return result.Build()
}

// Global Var

func (c *Codegen) GlobalVar(name string, flags ir.GlobalVarFlags, value ir.Value) *ir.GlobalVar {
	gVar := c.Module.NewGlobalVar(name, value.Type())
	gVar.Flags = flags
	gVar.Initializer = value

	return gVar
}
