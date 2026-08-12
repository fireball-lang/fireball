package codegen

import (
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/types"
	"slices"
	"strings"
)

func (c *codegen) GetTypeInfo(typ types.Type) ir.Value {
	name := TypeInfoLinkName(typ, "type_info")

	// Summary ref
	if c.moduleSummaryRef.Valid() {
		defer func() {
			ref := c.GetSummaryRef(name, true)

			if !slices.Contains(c.summaryRefs, ref) {
				c.summaryRefs = append(c.summaryRefs, ref)
			}
		}()
	}

	// Check already existing type infos
	for gVar := range c.module.GlobalVars() {
		if gVar.Name == name {
			return gVar
		}
	}

	// Instantiate generic / pseudo-generic implementation
	switch typ := typ.(type) {
	case *types.Array, *types.Pointer, *types.Func:
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

	// Extern type info
	gVar := c.module.NewGlobalVar(name, c.types.Get(c.needed.TypeInfo))
	gVar.Flags = ir.External

	return gVar
}

func (c *codegen) GetVTable(in *types.Interface, typ types.Type) ir.Value {
	// Vtables are keyed on the canonical (non-mutable) interface.
	in = in.AsImmutable()
	name := VTableLinkName(in, typ)

	// Summary ref
	if c.moduleSummaryRef.Valid() {
		defer func() {
			ref := c.GetSummaryRef(name, true)

			if !slices.Contains(c.summaryRefs, ref) {
				c.summaryRefs = append(c.summaryRefs, ref)
			}
		}()
	}

	// Check already existing vtables
	for gVar := range c.module.GlobalVars() {
		if gVar.Name == name {
			return gVar
		}
	}

	// Instantiate generic implementation
	if s, ok := typ.(*types.Struct); ok && s.Generic != nil {
		inGeneric := in.Generic
		if inGeneric == nil {
			inGeneric = in
		}

		implNode := c.typeEnv.GetImplNode(s.Generic, inGeneric)
		return c.CreateGenericVTable(name, in, s, implNode)
	}

	// Extern vtable
	gVar := c.module.NewGlobalVar(name, &ir.StructType{Fields: []ir.Field{
		{Name: "type_info", Type: ir.Pointer},
		{Name: "methods", Type: &ir.ArrayType{
			Length:  uint32(len(in.InstanceMethods)),
			Element: ir.Pointer,
		}},
	}})
	gVar.Flags = ir.External

	return gVar
}

func (c *codegen) CreateGenericVTable(name string, in *types.Interface, s *types.Struct, impl *ast.Impl) ir.Value {
	implFileTypes := c.fileDataMap[ast.GetFile(impl)].NodeTypes

	// Collect methods
	methods := make([]ir.Value, 0, len(in.InstanceMethods))

	for _, inMethod := range in.InstanceMethods {
		implMethod := findImplMethod(impl, inMethod)

		implMethodType := implFileTypes[implMethod].(*types.Func)
		instantiatedImplMethodType := c.instantiations.Get(implMethodType, s.Substitutions).(*types.Func)

		methods = append(methods, c.GetFunction(implMethod, instantiatedImplMethodType, in))
	}

	// Create
	return c.CreateVTableVar(name, s, methods, true)
}

func (c *codegen) CreateVTable(impl *ast.Impl) ir.Value {
	in := c.nodeTypes[impl.Interface].(*types.Interface)

	// Collect methods
	methods := make([]ir.Value, 0, len(in.InstanceMethods))

	for _, inMethod := range in.InstanceMethods {
		implMethod := findImplMethod(impl, inMethod)
		implMethodType := c.nodeTypes[implMethod].(*types.Func)

		methods = append(methods, c.GetFunction(implMethod, implMethodType, in))
	}

	// Create
	var typ types.Type

	if p, ok := impl.Type.(*ast.PrimitiveType); ok {
		typ = types.GetPrimitive(p.Kind)
	} else {
		typ = c.nodeTypes[impl.Type]
	}

	name := VTableLinkName(in, typ)
	return c.CreateVTableVar(name, typ, methods, false)
}

func findImplMethod(impl *ast.Impl, inMethod types.Method) *ast.Func {
	var implMethod *ast.Func

	for _, method := range impl.Methods {
		if method.Receiver != nil && method.Name().Token.Text == inMethod.Name {
			implMethod = method
			break
		}
	}

	if implMethod == nil {
		panic("codegen.findImplMethod() - vtable method not found in impl block")
	}

	return implMethod
}

func (c *codegen) CreateTypeInfo(typ types.Type, linkOnce bool) *ir.GlobalVar {
	name := TypeInfoLinkName(typ, "type_info")
	gVar := c.module.GetGlobalVar(name)

	if gVar == nil {
		gVar = c.module.NewGlobalVar(name, c.types.Get(c.needed.TypeInfo))
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

	c.GlobalVarSummary(name, true, linkOnce, value)

	return gVar
}

func (c *codegen) CreateTypeInfoInitializer(typ types.Type, linkOnce bool) ir.Value {
	info := c.arch.Info(typ)
	sb := c.Struct(c.needed.TypeInfo)

	// kind
	{
		field := c.needed.TypeInfo.Field("kind")
		if field == nil {
			panic("codegen.codegen.CreateTypeInfo() - Failed to find 'kind' field on 'core::TypeInfo'")
		}

		var kind uint64

		switch typ.(type) {
		case *types.Primitive:
			kind = 0
		case *types.Array:
			kind = 1
		case *types.Pointer:
			kind = 2
		case *types.Func:
			kind = 3
		case *types.Enum:
			kind = 4
		case *types.Struct:
			kind = 5
		case *types.Interface:
			kind = 6

		default:
			panic("codegen.codegen.CreateTypeInfo() - Invalid type")
		}

		sb.Set("kind", &ir.Integer{Typ: c.types.Get(field.Type), Value: core.Unsigned(false, kind)})
	}

	// name
	{
		field := c.needed.TypeInfo.Field("name")
		if field == nil {
			panic("codegen.codegen.CreateTypeInfo() - Failed to find 'name' field on 'core::TypeInfo'")
		}

		switch typ.(type) {
		case *types.Enum, *types.Struct, *types.Interface:
			sb.Set("name", c.StringView([]rune(typ.String())))

		default:
			sb.Set("name", &ir.ZeroInitializer{Typ: c.types.Get(field.Type)})
		}
	}

	// size
	{
		field := c.needed.TypeInfo.Field("size")
		if field == nil {
			panic("codegen.codegen.CreateTypeInfo() - Failed to find 'size' field on 'core::TypeInfo'")
		}

		sb.Set("size", &ir.Integer{Typ: c.types.Get(field.Type), Value: core.Unsigned(false, uint64(info.Size))})
	}

	// alignment
	{
		field := c.needed.TypeInfo.Field("alignment")
		if field == nil {
			panic("codegen.codegen.CreateTypeInfo() - Failed to find 'alignment' field on 'core::TypeInfo'")
		}

		sb.Set("alignment", &ir.Integer{Typ: c.types.Get(field.Type), Value: core.Unsigned(false, uint64(info.Align))})
	}

	// implements
	{
		field := c.needed.TypeInfo.Field("implements")
		if field == nil {
			panic("codegen.codegen.CreateTypeInfo() - Failed to find 'implements' field on 'core::TypeInfo'")
		}

		implementsSb := c.Struct(field.Type.(*types.Struct))

		implements := c.CreateTypeInfoImplements(typ, linkOnce)

		if implements != nil {
			implementsSb.Set("ptr", implements)

			size := &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(implements.Typ.(*ir.ArrayType).Length))}
			implementsSb.Set("size", size)
		}

		sb.Set("implements", implementsSb.Build())
	}

	// data
	{
		switch typ := typ.(type) {
		case *types.Primitive:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(typ.Kind))})
			sb.Set("data_ptr", &ir.Null{})

		case *types.Array:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(typ.Size))})
			sb.Set("data_ptr", c.GetTypeInfo(typ.Element))

		case *types.Pointer:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, 0)})
			sb.Set("data_ptr", c.GetTypeInfo(typ.Pointee))

		case *types.Func:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, 0)})
			sb.Set("data_ptr", &ir.Null{})

		case *types.Enum:
			cases := c.CreateTypeInfoCases(typ, linkOnce)

			if cases != nil {
				sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(cases.Typ.(*ir.ArrayType).Length))})
				sb.Set("data_ptr", cases)
			} else {
				sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, 0)})
				sb.Set("data_ptr", &ir.Null{})
			}

		case *types.Struct:
			fields := c.CreateTypeInfoFields(typ, linkOnce)

			if fields != nil {
				sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, uint64(fields.Typ.(*ir.ArrayType).Length))})
				sb.Set("data_ptr", fields)
			} else {
				sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, 0)})
				sb.Set("data_ptr", &ir.Null{})
			}

		case *types.Interface:
			sb.Set("data_int", &ir.Integer{Typ: ir.I64, Value: core.Unsigned(false, 0)})
			sb.Set("data_ptr", &ir.Null{})

		default:
			panic("codegen.codegen.CreateTypeInfo() - Invalid type")
		}
	}

	return sb.Build()
}

func (c *codegen) CreateTypeInfoImplements(typ types.Type, linkOnce bool) *ir.GlobalVar {
	interfaces := c.typeEnv.GetConformances(typ)
	if len(interfaces) == 0 {
		return nil
	}

	ab := c.Array(&types.Array{
		Size:    uint32(len(interfaces)),
		Element: &types.Pointer{Pointee: c.needed.TypeInfo},
	})

	for _, in := range interfaces {
		ab.Add(c.GetTypeInfo(in))
	}

	name := TypeInfoLinkName(typ, "type_info_implements")
	return c.GlobalVar(name, true, linkOnce, ab.Build())
}

func (c *codegen) CreateTypeInfoCases(typ types.Type, linkOnce bool) *ir.GlobalVar {
	if e, ok := typ.(*types.Enum); ok {
		ab := c.Array(&types.Array{
			Size:    uint32(len(e.Cases)),
			Element: c.needed.Case,
		})

		for _, case_ := range e.Cases {
			sb := c.Struct(c.needed.Case)

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
		return c.GlobalVar(name, true, linkOnce, ab.Build())
	}

	return nil
}

func (c *codegen) CreateTypeInfoFields(typ types.Type, linkOnce bool) *ir.GlobalVar {
	if s, ok := typ.(*types.Struct); ok {
		ab := c.Array(&types.Array{
			Size:    uint32(len(s.Fields)),
			Element: c.needed.Field,
		})

		info := c.arch.Info(s)

		for _, infoField := range info.Fields {
			sb := c.Struct(c.needed.Field)

			field := s.Fields[infoField.Index]

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
		return c.GlobalVar(name, true, linkOnce, ab.Build())
	}

	return nil
}

func (c *codegen) CreateVTableVar(name string, typ types.Type, methods []ir.Value, linkOnce bool) *ir.GlobalVar {
	methodsTyp := &types.Array{Size: uint32(len(methods)), Element: &types.Pointer{Pointee: types.PrimitiveVoid}}

	sb := c.Struct(&types.Struct{Layout: types.C, Fields: []types.Field{
		{Name: "type_info", Type: &types.Pointer{Pointee: c.needed.TypeInfo}},
		{Name: "methods", Type: methodsTyp},
	}})

	sb.Set("type_info", c.GetTypeInfo(typ))

	{
		ab := c.Array(methodsTyp)

		for _, method := range methods {
			ab.Add(method)
		}

		sb.Set("methods", ab.Build())
	}

	return c.GlobalVar(name, true, linkOnce, sb.Build())
}

func VTableLinkName(in *types.Interface, typ types.Type) string {
	sb := strings.Builder{}
	sb.WriteString("fb$vtable$")

	var name string
	var substitutions []types.Substitution

	if t, ok := typ.(*types.Primitive); ok {
		name = t.Kind.String()
	} else if t, ok := typ.(*types.Enum); ok {
		name = t.Name
	} else if t, ok := typ.(*types.Struct); ok {
		name = t.Name
		substitutions = t.Substitutions
	} else {
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
	sb := strings.Builder{}
	sb.WriteString("fb$")
	sb.WriteString(kind)
	sb.WriteRune('$')

	var name string
	var substitutions []types.Substitution

	switch typ := typ.(type) {
	case *types.Primitive, *types.Array, *types.Pointer, *types.Func:
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
