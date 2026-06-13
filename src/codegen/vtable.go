package codegen

import (
	"fireball/ast"
	"fireball/ir"
	"fireball/types"
	"slices"
	"strings"
)

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
	gVar := c.module.NewGlobalVar(name, &ir.ArrayType{Length: uint32(len(in.InstanceMethods)), Element: ir.Pointer})
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
	return c.CreateVTableVar(name, methods, methodNodes, true)
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
	name := VTableLinkName(in, c.nodeTypes[impl.Type])
	return c.CreateVTableVar(name, methods, methodNodes, false)
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

func (c *codegen) CreateVTableVar(name string, methods []ir.Value, methodNodes []*ast.Func, linkOnce bool) *ir.GlobalVar {
	// Global variable
	gVar := c.module.NewGlobalVar(name, &ir.ArrayType{Length: uint32(len(methods)), Element: ir.Pointer})
	gVar.Flags = ir.UnnamedAddr | ir.Constant
	gVar.Initializer = &ir.Array{Elements: methods}

	if linkOnce {
		gVar.Flags |= ir.LinkOnce
	}

	// Summary
	if c.moduleSummaryRef.Valid() {
		var methodRefs []ir.SummaryRef

		for _, astFunc := range methodNodes {
			if sumRef, ok := c.functionSummaries[astFunc]; ok {
				methodRefs = append(methodRefs, sumRef)
			}
		}

		linkage := ir.LinkageExternal
		if linkOnce {
			linkage = ir.LinkageLinkOnceODR
		}

		c.module.AddSummary(&ir.VariableSummary{
			Module: c.moduleSummaryRef,
			Name:   gVar.Name,
			LinkFlags: ir.LinkSummaryFlags{
				Linkage:             linkage,
				Visibility:          ir.VisibilityDefault,
				NotEligibleToImport: false,
				Live:                false,
				DsoLocal:            true,
				CanAutoHide:         true,
				ImportType:          ir.ImportDefinition,
			},
			Flags: ir.VarReadOnly | ir.VarConstant,
			Refs:  methodRefs,
		})
	}

	return gVar
}

func VTableLinkName(in *types.Interface, typ types.Type) string {
	sb := strings.Builder{}
	sb.WriteString("fb$vtable$")

	var name string
	var substitutions []types.Substitution

	if t, ok := typ.(*types.Struct); ok {
		name = t.Name
		substitutions = t.Substitutions
	} else {
		t := typ.(*types.Enum)
		name = t.Name
	}

	// Concrete type path
	sb.WriteString(name)

	// Concrete type args
	if len(substitutions) > 0 {
		sb.WriteString("::[")

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
		sb.WriteString("::[")

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
