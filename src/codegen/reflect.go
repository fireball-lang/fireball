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

	// Instantiate generic implementation
	if s, ok := typ.(*types.Struct); ok && s.Generic != nil {
		return c.CreateTypeInfo(s, true)
	}
	if in, ok := typ.(*types.Interface); ok && in.Generic != nil {
		return c.CreateTypeInfo(in, true)
	}

	// Extern type info
	gVar := c.module.NewGlobalVar(name, c.types.Get(c.typeInfo))
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
	methodNodes := make([]*ast.Func, 0, len(in.InstanceMethods))

	for _, inMethod := range in.InstanceMethods {
		implMethod := findImplMethod(impl, inMethod)

		implMethodType := implFileTypes[implMethod].(*types.Func)
		instantiatedImplMethodType := c.instantiations.Get(implMethodType, s.Substitutions).(*types.Func)

		methods = append(methods, c.GetFunction(implMethod, instantiatedImplMethodType, in))
		methodNodes = append(methodNodes, implMethod)
	}

	// Create
	return c.CreateVTableVar(name, s, methods, methodNodes, true)
}

func (c *codegen) CreateVTable(impl *ast.Impl) ir.Value {
	in := c.nodeTypes[impl.Interface].(*types.Interface)

	// Collect methods
	methods := make([]ir.Value, 0, len(in.InstanceMethods))
	methodNodes := make([]*ast.Func, 0, len(in.InstanceMethods))

	for _, inMethod := range in.InstanceMethods {
		implMethod := findImplMethod(impl, inMethod)
		implMethodType := c.nodeTypes[implMethod].(*types.Func)

		methods = append(methods, c.GetFunction(implMethod, implMethodType, in))
		methodNodes = append(methodNodes, implMethod)
	}

	// Create
	var typ types.Type

	if p, ok := impl.Type.(*ast.PrimitiveType); ok {
		typ = types.GetPrimitive(p.Kind)
	} else {
		typ = c.nodeTypes[impl.Type]
	}

	name := VTableLinkName(in, typ)
	return c.CreateVTableVar(name, typ, methods, methodNodes, false)
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
	sb := c.Struct(c.typeInfo)

	{
		field := c.typeInfo.Field("implements")
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

	name := TypeInfoLinkName(typ, "type_info")
	return c.GlobalVar(name, true, linkOnce, sb.Build())
}

func (c *codegen) CreateTypeInfoImplements(typ types.Type, linkOnce bool) *ir.GlobalVar {
	interfaces := c.typeEnv.GetConformances(typ)
	if len(interfaces) == 0 {
		return nil
	}

	ab := c.Array(&types.Array{
		Size:    uint32(len(interfaces)),
		Element: &types.Pointer{Pointee: c.typeInfo},
	})

	for _, in := range interfaces {
		ab.Add(c.GetTypeInfo(in))
	}

	name := TypeInfoLinkName(typ, "type_info_implements")
	return c.GlobalVar(name, true, linkOnce, ab.Build())
}

func (c *codegen) CreateVTableVar(name string, typ types.Type, methods []ir.Value, methodNodes []*ast.Func, linkOnce bool) *ir.GlobalVar {
	methodsTyp := &types.Array{Size: uint32(len(methods)), Element: &types.Pointer{Pointee: types.PrimitiveVoid}}

	sb := c.Struct(&types.Struct{Layout: types.C, Fields: []types.Field{
		{Name: "type_info", Type: &types.Pointer{Pointee: c.typeInfo}},
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

	if t, ok := typ.(*types.Primitive); ok {
		name = t.Kind.String()
	} else if t, ok := typ.(*types.Enum); ok {
		name = t.Name
	} else if t, ok := typ.(*types.Struct); ok {
		name = t.Name
		substitutions = t.Substitutions
	} else if t, ok := typ.(*types.Interface); ok {
		name = t.Name
		substitutions = t.Substitutions
	} else {
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
