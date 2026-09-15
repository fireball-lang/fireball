package codegen

import (
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/types"
	"fmt"
	"strings"
)

func (c *Codegen) GetTypeInfo(typ types.Type) ir.Value {
	name := TypeInfoLinkName(typ, "type_info")

	// Check already existing type infos
	if gVar := c.Module.GetGlobalVar(name); gVar != nil {
		return gVar
	}

	// Instantiate generic / pseudo-generic implementation
	switch typ := typ.(type) {
	case *types.Array, *types.Reference, *types.Pointer, *types.Func:
		return c.CreateTypeInfo(typ, true)

	case *types.Struct:
		if typ.Generic != nil {
			return c.CreateTypeInfo(typ, true)
		}

	case *types.Interface:
		if typ.Generic != nil {
			return c.CreateTypeInfo(typ, true)
		}
	}

	// Create if evaluating constants
	if c.CompTime {
		return c.CreateTypeInfo(typ, false)
	}

	// Extern type info
	gVar := c.Module.NewGlobalVar(name, c.Types.Get(c.Builtins.TypeInfo))
	gVar.Data = typ
	gVar.Flags = ir.External

	return gVar
}

func (c *Codegen) GetVTable(in *types.Interface, typ types.Type) ir.Value {
	// Vtables are keyed on the canonical (non-mutable) interface.
	in = in.AsImmutable()
	name := VTableLinkName(in, typ)

	// Check already existing vtables
	if gVar := c.Module.GetGlobalVar(name); gVar != nil {
		return gVar
	}

	// Instantiate generic / pseudo-generic implementation
	switch typ := typ.(type) {
	case *types.Struct:
		if typ.Generic != nil {
			return c.CreateVTable(typ, in, true)
		}
	}

	// Extern vtable
	gVar := c.Module.NewGlobalVar(name, c.Types.Get(c.VtableStruct(in)))
	gVar.Flags = ir.External

	return gVar
}

func (c *Codegen) CreateVTable(typ types.Type, in *types.Interface, linkOnce bool) *ir.GlobalVar {
	// Vtables are keyed on the canonical (non-mutable) interface.
	in = in.AsImmutable()
	name := VTableLinkName(in, typ)

	gVar := c.Module.GetGlobalVar(name)

	if gVar == nil {
		gVar = c.Module.NewGlobalVar(name, c.Types.Get(c.VtableStruct(in)))
	}
	if !core.IsNil(gVar.Initializer) {
		return gVar
	}

	value := c.CreateVTableInitializer(typ, in)

	gVar.Flags = ir.Constant
	if linkOnce {
		gVar.Flags |= ir.LinkOnce
	}

	gVar.Initializer = value

	return gVar
}

func (c *Codegen) CreateVTableInitializer(typ types.Type, in *types.Interface) ir.Value {
	if null, ok := typ.(*types.Null); ok {
		typ = null.Underlying()
	}

	t := c.VtableStruct(in)
	sb := c.Struct(t)

	sb.Set("type_info", c.GetTypeInfo(typ))

	ab := c.Array(t.Fields[1].Type.(*types.Array))

	for _, im := range in.InstanceMethods {
		ab.Add(c.VtableMethod(typ, in, im))
	}

	sb.Set("methods", ab.Build())

	return sb.Build()
}

func (c *Codegen) VtableStruct(in *types.Interface) *types.Struct {
	return &types.Struct{Layout: types.C, Fields: []types.Field{
		{Name: "type_info", Type: &types.Pointer{Pointee: c.Builtins.TypeInfo}},
		{Name: "methods", Type: &types.Array{
			Size:    uint32(len(in.InstanceMethods)),
			Element: &types.Pointer{Pointee: types.PrimitiveVoid},
		}},
	}}
}

func (c *Codegen) VtableMethod(typ types.Type, in *types.Interface, im types.Method) ir.Value {
	sym, subs, ok := c.TypeEnv.GetInstanceMethodWithSubs(typ, im.Name)
	if !ok {
		panic(fmt.Sprintf("codegen.vtableMethod() - interface method '%s' not found on '%s'", im.Name, typ))
	}

	concreteFunc := sym.Node.(*ast.Func)
	concreteTyp := sym.Type.(*types.Func)

	if len(subs) > 0 {
		concreteTyp = c.instantiations.Substitute(concreteTyp, subs).(*types.Func)
	}

	return c.GetFunction(concreteFunc, concreteTyp, in)
}

func (c *Codegen) CreateTypeInfo(typ types.Type, linkOnce bool) *ir.GlobalVar {
	name := TypeInfoLinkName(typ, "type_info")
	gVar := c.Module.GetGlobalVar(name)

	if gVar == nil {
		gVar = c.Module.NewGlobalVar(name, c.Types.Get(c.Builtins.TypeInfo))
		gVar.Data = typ
	}
	if !core.IsNil(gVar.Initializer) {
		return gVar
	}

	value := c.CreateTypeInfoInitializer(typ, linkOnce)

	gVar.Flags = ir.Constant
	if linkOnce {
		gVar.Flags |= ir.LinkOnce
	}

	gVar.Initializer = value

	return gVar
}

func (c *Codegen) CreateTypeInfoInitializer(typ types.Type, linkOnce bool) ir.Value {
	if null, ok := typ.(*types.Null); ok {
		typ = null.Underlying()
	}

	info := c.Arch.Info(typ)
	sb := c.Struct(c.Builtins.TypeInfo)

	// kind
	{
		field := c.Builtins.TypeInfo.Field("kind")
		if field == nil {
			panic("codegen.Codegen.CreateTypeInfo() - Failed to find 'kind' field on 'core::TypeInfo'")
		}

		var kind uint64

		switch typ.(type) {
		case *types.Primitive:
			kind = 0
		case *types.Array:
			kind = 1
		case *types.Reference:
			kind = 2
		case *types.Pointer:
			kind = 3
		case *types.Func:
			kind = 4
		case *types.Enum:
			kind = 5
		case *types.Struct:
			kind = 6
		case *types.Interface:
			kind = 7

		default:
			panic("codegen.Codegen.CreateTypeInfo() - Invalid type")
		}

		sb.Set("kind", &ir.Integer{Typ: c.Types.Get(field.Type), Value: core.Unsigned(false, kind)})
	}

	// name
	{
		field := c.Builtins.TypeInfo.Field("name")
		if field == nil {
			panic("codegen.Codegen.CreateTypeInfo() - Failed to find 'name' field on 'core::TypeInfo'")
		}

		switch typ.(type) {
		case *types.Enum, *types.Struct, *types.Interface:
			sb.Set("name", c.StringView([]rune(typ.String())))

		default:
			sb.Set("name", &ir.ZeroInitializer{Typ: c.Types.Get(field.Type)})
		}
	}

	// size
	{
		field := c.Builtins.TypeInfo.Field("size")
		if field == nil {
			panic("codegen.Codegen.CreateTypeInfo() - Failed to find 'size' field on 'core::TypeInfo'")
		}

		sb.Set("size", &ir.Integer{Typ: c.Types.Get(field.Type), Value: core.Unsigned(false, uint64(info.Size))})
	}

	// alignment
	{
		field := c.Builtins.TypeInfo.Field("alignment")
		if field == nil {
			panic("codegen.Codegen.CreateTypeInfo() - Failed to find 'alignment' field on 'core::TypeInfo'")
		}

		sb.Set("alignment", &ir.Integer{Typ: c.Types.Get(field.Type), Value: core.Unsigned(false, uint64(info.Align))})
	}

	// implementations
	{
		field := c.Builtins.TypeInfo.Field("implementations")
		if field == nil {
			panic("codegen.Codegen.CreateTypeInfo() - Failed to find 'implementations' field on 'core::TypeInfo'")
		}

		implementationsSb := c.Struct(field.Type.(*types.Struct))

		implementations := c.CreateTypeInfoImplementations(typ, linkOnce)

		if implementations != nil {
			implementationsSb.Set("ptr", implementations)

			size := &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(implementations.Typ.(*ir.ArrayType).Length))}
			implementationsSb.Set("size", size)
		}

		sb.Set("implementations", implementationsSb.Build())
	}

	// data
	{
		switch typ := typ.(type) {
		case *types.Primitive:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(typ.Kind))})
			sb.Set("data_ptr1", &ir.Null{})
			sb.Set("data_ptr2", &ir.Null{})

		case *types.Array:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(typ.Size))})
			sb.Set("data_ptr1", c.GetTypeInfo(typ.Element))
			sb.Set("data_ptr2", &ir.Null{})

		case *types.Reference:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, 0)})
			sb.Set("data_ptr1", c.GetTypeInfo(typ.Pointee))
			sb.Set("data_ptr2", &ir.Null{})

		case *types.Pointer:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, 0)})
			sb.Set("data_ptr1", c.GetTypeInfo(typ.Pointee))
			sb.Set("data_ptr2", &ir.Null{})

		case *types.Func:
			parameters := c.CreateTypeInfoParameters(typ, linkOnce)

			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(parameters.Typ.(*ir.ArrayType).Length))})
			sb.Set("data_ptr1", parameters)
			sb.Set("data_ptr2", c.GetTypeInfo(typ.Returns))

		case *types.Enum:
			cases := c.CreateTypeInfoCases(typ, linkOnce)

			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(cases.Typ.(*ir.ArrayType).Length))})
			sb.Set("data_ptr1", cases)
			sb.Set("data_ptr2", c.GetTypeInfo(typ.Underlying()))

		case *types.Struct:
			fields := c.CreateTypeInfoFields(typ, linkOnce)

			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(fields.Typ.(*ir.ArrayType).Length))})
			sb.Set("data_ptr1", fields)
			sb.Set("data_ptr2", &ir.Null{})

		case *types.Interface:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, 0)})
			sb.Set("data_ptr1", &ir.Null{})
			sb.Set("data_ptr2", &ir.Null{})

		default:
			panic("codegen.Codegen.CreateTypeInfo() - Invalid type")
		}
	}

	return sb.Build()
}

func (c *Codegen) CreateTypeInfoImplementations(typ types.Type, linkOnce bool) *ir.GlobalVar {
	interfaces := c.TypeEnv.GetConformances(typ)
	if len(interfaces) == 0 {
		return nil
	}

	ab := c.Array(&types.Array{
		Size:    uint32(len(interfaces)),
		Element: c.Builtins.Implementation,
	})

	for _, in := range interfaces {
		sb := c.Struct(c.Builtins.Implementation)

		var vtable ir.Value
		if len(in.InstanceMethods) == 0 {
			vtable = &ir.Null{}
		} else {
			t := typ

			if pointee, ok := getPointee(t); ok {
				t = pointee
			}

			vtable = c.GetVTable(in, t)
		}

		sb.Set("type_info", c.GetTypeInfo(in))
		sb.Set("vtable", vtable)

		ab.Add(sb.Build())
	}

	name := TypeInfoLinkName(typ, "type_info_implementations")

	flags := ir.Constant
	if linkOnce {
		flags |= ir.LinkOnce
	}

	return c.GlobalVar(name, flags, ab.Build())
}

func (c *Codegen) CreateTypeInfoParameters(typ *types.Func, linkOnce bool) *ir.GlobalVar {
	ab := c.Array(&types.Array{
		Size:    uint32(len(typ.Params)),
		Element: &types.Pointer{Pointee: c.Builtins.TypeInfo},
	})

	for _, param := range typ.Params {
		ab.Add(c.GetTypeInfo(param))
	}

	name := TypeInfoLinkName(typ, "type_info_parameters")

	flags := ir.Constant
	if linkOnce {
		flags |= ir.LinkOnce
	}

	return c.GlobalVar(name, flags, ab.Build())
}

func (c *Codegen) CreateTypeInfoCases(typ *types.Enum, linkOnce bool) *ir.GlobalVar {
	ab := c.Array(&types.Array{
		Size:    uint32(len(typ.Cases)),
		Element: c.Builtins.Case,
	})

	for _, case_ := range typ.Cases {
		sb := c.Struct(c.Builtins.Case)

		negative := ir.False
		if case_.Value.Negative() {
			negative = ir.True
		}

		sb.Set("name", c.StringView([]rune(case_.Name)))
		sb.Set("negative", negative)
		sb.Set("value", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, case_.Value.Raw())})

		ab.Add(sb.Build())
	}

	name := TypeInfoLinkName(typ, "type_info_cases")

	flags := ir.Constant
	if linkOnce {
		flags |= ir.LinkOnce
	}

	return c.GlobalVar(name, flags, ab.Build())
}

func (c *Codegen) CreateTypeInfoFields(typ *types.Struct, linkOnce bool) *ir.GlobalVar {
	ab := c.Array(&types.Array{
		Size:    uint32(len(typ.Fields)),
		Element: c.Builtins.Field,
	})

	info := c.Arch.Info(typ)

	for _, infoField := range info.Fields {
		sb := c.Struct(c.Builtins.Field)

		field := typ.Fields[infoField.Index]

		public := ir.False
		if field.Public {
			public = ir.True
		}

		sb.Set("name", c.StringView([]rune(field.Name)))
		sb.Set("type_info", c.GetTypeInfo(field.Type))
		sb.Set("public", public)
		sb.Set("offset", &ir.Integer{Typ: ir.I32, Value: core.Unsigned(false, uint64(infoField.Offset))})

		ab.Add(sb.Build())
	}

	name := TypeInfoLinkName(typ, "type_info_fields")

	flags := ir.Constant
	if linkOnce {
		flags |= ir.LinkOnce
	}

	return c.GlobalVar(name, flags, ab.Build())
}

func VTableLinkName(in *types.Interface, typ types.Type) string {
	if null, ok := typ.(*types.Null); ok {
		typ = null.Underlying()
	}

	sb := strings.Builder{}
	sb.WriteString("fb$vtable$")

	var name string
	var substitutions []types.Substitution

	switch typ := typ.(type) {
	case *types.Primitive, *types.Array, *types.Reference, *types.Pointer, *types.Enum:
		name = typ.String()

	case *types.Struct:
		name = typ.Name
		substitutions = typ.Substitutions

	default:
		panic("codegen.VTableLinkName() - Invalid type")
	}

	// Concrete type path
	sb.WriteString(name)

	// Concrete type args
	if len(substitutions) > 0 {
		sb.WriteString("[")

		for i, sub := range substitutions {
			if i > 0 {
				sb.WriteRune(',')
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')
	}

	sb.WriteRune('$')

	// Interface type path
	sb.WriteString(in.Name)

	// Interface type args
	if in.Generic != nil {
		sb.WriteString("[")

		for i, sub := range in.Substitutions {
			if i > 0 {
				sb.WriteRune(',')
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')
	}

	return sb.String()
}

func TypeInfoLinkName(typ types.Type, kind string) string {
	if null, ok := typ.(*types.Null); ok {
		typ = null.Underlying()
	}

	sb := strings.Builder{}
	sb.WriteString("fb$")
	sb.WriteString(kind)
	sb.WriteRune('$')

	var name string
	var substitutions []types.Substitution

	switch typ := typ.(type) {
	case *types.Primitive, *types.Array, *types.Reference, *types.Pointer, *types.Func:
		name = typ.String()

	case *types.Enum:
		name = typ.Name

	case *types.Struct:
		name = typ.Name
		substitutions = typ.Substitutions

	case *types.Interface:
		name = typ.Name
		substitutions = typ.Substitutions

	default:
		panic("codegen.TypeInfoLinkName() - Invalid type")
	}

	// Concrete type path
	sb.WriteString(name)

	// Concrete type args
	if len(substitutions) > 0 {
		sb.WriteString("[")

		for i, sub := range substitutions {
			if i > 0 {
				sb.WriteRune(',')
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')
	}

	return sb.String()
}
