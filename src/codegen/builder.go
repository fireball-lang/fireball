package codegen

import (
	"fireball/ir"
	"fireball/types"
)

// Array Builder

type arrayBuilder struct {
	c   *codegen
	typ *types.Array

	values []ir.Value
}

func (c *codegen) Array(a *types.Array) arrayBuilder {
	return arrayBuilder{
		c:   c,
		typ: a,
	}
}

func (a *arrayBuilder) Add(value ir.Value) {
	a.values = append(a.values, value)
}

func (a *arrayBuilder) Build() ir.Value {
	typ := a.c.types.Get(a.typ).(*ir.ArrayType)

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
			arrayValue = a.c.emitter.InsertValue(arrayValue, value, uint32(i))
		}
	}

	return arrayValue
}

// Struct Builder

type structBuilder struct {
	c   *codegen
	typ *types.Struct

	fields map[string]ir.Value
}

func (c *codegen) Struct(s *types.Struct) structBuilder {
	return structBuilder{
		c:      c,
		typ:    s,
		fields: make(map[string]ir.Value),
	}
}

func (s *structBuilder) Set(name string, value ir.Value) {
	s.fields[name] = value
}

func (s *structBuilder) Build() ir.Value {
	typ := s.c.types.Get(s.typ).(ir.StructLikeType)

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
		if s.typ.Layout == types.Union {
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

			if s.typ.Layout == types.Union {
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
		if s.typ.Layout != types.Union {
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

			if s.typ.Layout == types.Union {
				i = 0
			} else {
				_, i = typ.Field(name)
				if i < 0 {
					panic("codegen.structBuilder.Build() - Failed to find field '" + name + "' on type '" + s.typ.String() + "'")
				}
			}

			structValue = s.c.emitter.InsertValue(structValue, value, uint32(i))
		}
	}

	return structValue
}

// Global Var

func (c *codegen) GlobalVar(name string, constant bool, linkOnce bool, value ir.Value) *ir.GlobalVar {
	gVar := c.module.NewGlobalVar(name, value.Type())
	gVar.Initializer = value

	if constant {
		gVar.Flags = ir.Constant
	}
	if linkOnce {
		gVar.Flags |= ir.LinkOnce
	}

	// Summary
	c.GlobalVarSummary(name, constant, linkOnce, value)

	return gVar
}

func (c *codegen) GlobalVarSummary(name string, constant bool, linkOnce bool, value ir.Value) {
	if c.moduleSummaryRef.Valid() {
		linkage := ir.LinkageExternal
		if linkOnce {
			linkage = ir.LinkageLinkOnceODR
		}

		flags := ir.VariableSummaryFlags(0)
		if constant {
			flags = ir.VarReadOnly | ir.VarConstant
		}

		refs := c.CollectSummaryRefs(nil, value)

		c.module.AddSummary(&ir.VariableSummary{
			Module: c.moduleSummaryRef,
			Name:   name,
			LinkFlags: ir.LinkSummaryFlags{
				Linkage:             linkage,
				Visibility:          ir.VisibilityDefault,
				NotEligibleToImport: false,
				Live:                false,
				DsoLocal:            true,
				CanAutoHide:         true,
				ImportType:          ir.ImportDefinition,
			},
			Flags: flags,
			Refs:  refs,
		})
	}
}
