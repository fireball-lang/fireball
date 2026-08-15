package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/sema"
	"fireball/types"
	"fmt"
	"hash/crc32"
	"path/filepath"
	"strings"
)

type FileData struct {
	ExprInfos map[ast.Node]sema.ExprInfo
	NodeTypes map[ast.Node]types.Type
}

type Types struct {
	StringView *types.Struct

	Case  *types.Struct
	Field *types.Struct

	Implementation *types.Struct

	TypeInfo *types.Struct
}

type pendingInstantiation struct {
	f   *ast.Func
	typ *types.Func
	fun *ir.Function
}

type codegen struct {
	module *ir.Module
	uid    string

	arch      abi.Arch
	callConv  abi.CallConv
	exprInfos map[ast.Node]sema.ExprInfo
	nodeTypes map[ast.Node]types.Type
	typeEnv   *sema.TypeEnvironment

	needed Types

	scope       symbolScope
	stringCount uint32

	types   *TypeCache
	emitter ir.Emitter

	fileRef ir.MetaRef
	unitRef ir.MetaRef

	moduleSummaryRef  ir.SummaryRef
	functionSummaries map[string]ir.SummaryRef

	fun                     *ir.Function
	funcTyp                 *types.Func // type of the function currently being generated
	funDoesIndirectDispatch bool
	substitutions           []types.Substitution
	returnPtr               ir.Value
	bVariables              *ir.Block

	bLoopBreak    *ir.Block
	bLoopContinue *ir.Block

	summaryCalls []ir.FunctionSummaryCall
	summaryRefs  []ir.SummaryRef

	instantiations        *types.InstantiationCache
	pendingInstantiations []pendingInstantiation

	fileDataMap map[*ast.File]FileData
}

func Generate(file *ast.File, arch abi.Arch, callConv abi.CallConv, instantiations *types.InstantiationCache, typeEnv *sema.TypeEnvironment, fileDataMap map[*ast.File]FileData, neededTypes Types, path string, summary bool) *ir.Module {
	defer core.Scope()()

	module := ir.NewModule()
	module.Path = path

	c := codegen{
		module: module,
		uid:    fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(path))),

		arch:      arch,
		callConv:  callConv,
		exprInfos: fileDataMap[file].ExprInfos,
		nodeTypes: fileDataMap[file].NodeTypes,
		typeEnv:   typeEnv,

		needed: neededTypes,

		instantiations: instantiations,
		fileDataMap:    fileDataMap,

		types:   &TypeCache{Arch: arch, Module: module},
		emitter: ir.Emitter{Module: module},
	}

	// Setup meta

	c.fileRef = module.AddMeta(&ir.FileMeta{
		Path: path,
	})
	c.emitter.PushScope(c.fileRef)

	c.types.FileRef = c.fileRef

	var retainedTypes ir.MetaRef
	var retainedTypeRefs []ir.RawMetaValue

	for _, decl := range file.Decls {
		if s, ok := decl.(*ast.Struct); ok && len(s.TypeParams) == 0 {
			ref := c.types.GetMeta(c.nodeTypes[s])
			retainedTypeRefs = append(retainedTypeRefs, ir.RawMetaValue{Ref: ref})
		}
	}

	if len(retainedTypeRefs) > 0 {
		retainedTypes = c.module.AddMeta(&ir.RawMeta{Values: retainedTypeRefs})
	}

	c.unitRef = module.AddMeta(&ir.CompileUnitMeta{
		File:          c.fileRef,
		Producer:      "fireball",
		IsOptimized:   false,
		Enums:         0,
		RetainedTypes: retainedTypes,
		Globals:       0,
		Imports:       0,
	})

	c.module.AddNamedMetaRefs(
		"llvm.dbg.cu",
		c.unitRef,
	)

	// Setup summary

	if summary {
		c.moduleSummaryRef = c.module.AddSummary(&ir.ModuleSummary{
			Path: path,
			Hash: [5]uint32{},
		})

		c.functionSummaries = make(map[string]ir.SummaryRef)
	}

	// Function / Global Var declarations

	c.scope.Push()

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GlobalVar:
			typ := c.nodeTypes[decl]
			c.scope.Add(decl.Name().Token.Text, c.CreateGlobalVar(decl, typ, false))

		case *ast.Impl:
			var in *types.Interface
			if decl.Interface != nil {
				in, _ = c.nodeTypes[decl.Interface].(*types.Interface)
			}

			for _, f := range decl.Methods {
				if !c.HasTypeParams(f) {
					typ := c.nodeTypes[f].(*types.Func)
					c.CreateFunction(f, typ, false, in)
				}
			}

		case *ast.Func:
			if !c.HasTypeParams(decl) {
				typ := c.nodeTypes[decl].(*types.Func)
				c.scope.Add(decl.Name().Token.Text, c.CreateFunction(decl, typ, false, nil))
			}
		}
	}

	// V-Tables

	for _, decl := range file.Decls {
		if impl, ok := decl.(*ast.Impl); ok && impl.Interface != nil && len(impl.TypeParams) == 0 {
			in := c.nodeTypes[impl.Interface].(*types.Interface)

			var typ types.Type

			if p, ok := impl.Type.(*ast.PrimitiveType); ok {
				typ = types.GetPrimitive(p.Kind)
			} else {
				typ = c.nodeTypes[impl.Type]
			}

			c.CreateVTable(typ, in, false)
		}
	}

	// Type Infos

	if strings.HasSuffix(filepath.ToSlash(path), "build/dependencies/core/src/reflect.fb") {
		c.CreateTypeInfo(types.PrimitiveVoid, false)
		c.CreateTypeInfo(types.PrimitiveBool, false)

		c.CreateTypeInfo(types.PrimitiveU8, false)
		c.CreateTypeInfo(types.PrimitiveU16, false)
		c.CreateTypeInfo(types.PrimitiveU32, false)
		c.CreateTypeInfo(types.PrimitiveU64, false)

		c.CreateTypeInfo(types.PrimitiveI8, false)
		c.CreateTypeInfo(types.PrimitiveI16, false)
		c.CreateTypeInfo(types.PrimitiveI32, false)
		c.CreateTypeInfo(types.PrimitiveI64, false)

		c.CreateTypeInfo(types.PrimitiveF32, false)
		c.CreateTypeInfo(types.PrimitiveF64, false)
	}

	for _, decl := range file.Decls {
		if decl, ok := decl.(*ast.Interface); ok && !c.HasTypeParams(decl) {
			c.CreateTypeInfo(c.nodeTypes[decl], false)
		}
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Enum:
			c.CreateTypeInfo(c.nodeTypes[decl], false)

		case *ast.Struct:
			if !c.HasTypeParams(decl) {
				c.CreateTypeInfo(c.nodeTypes[decl], false)
			}
		}
	}

	// Function definitions

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Impl:
			var in *types.Interface
			if decl.Interface != nil {
				in, _ = c.nodeTypes[decl.Interface].(*types.Interface)
			}

			for _, f := range decl.Methods {
				if !c.HasTypeParams(f) {
					typ := c.nodeTypes[f].(*types.Func)
					fun := c.GetFunction(f, typ, in)

					c.VisitFunc(f, typ, fun)
				}
			}

		case *ast.Func:
			if !c.HasTypeParams(decl) {
				typ := c.nodeTypes[decl].(*types.Func)
				fun := c.scope.Get(decl.Name().Token.Text).(*ir.Function)

				c.VisitFunc(decl, typ, fun)
			}
		}
	}

	c.scope.Pop()

	// Function instantiations

	for len(c.pendingInstantiations) > 0 {
		pending := c.pendingInstantiations[len(c.pendingInstantiations)-1]
		c.pendingInstantiations = c.pendingInstantiations[:len(c.pendingInstantiations)-1]

		if fd, ok := c.fileDataMap[ast.GetFile(pending.f)]; ok {
			c.exprInfos = fd.ExprInfos
			c.nodeTypes = fd.NodeTypes
		}

		c.substitutions = pending.typ.Substitutions
		c.funcTyp = pending.typ

		if types.HasParam(pending.typ) {
			r := pending.f.Range()

			panic(fmt.Sprintf(
				"codegen.Generate() - function instantiation contains unresolved type parameters:\n  function: %s (%s:%d:%d)\n  type: %s\n  substitutions: %s",
				FuncLinkName(pending.f, pending.typ, nil), ast.GetFile(pending.f).Path, r.Start.Line, r.Start.Column,
				pending.typ.String(), typeSubsString(pending.typ.Substitutions),
			))
		}

		c.VisitFunc(pending.f, pending.typ, pending.fun)

		c.funcTyp = nil
		c.substitutions = nil
	}

	// End summary

	if summary {
		c.module.AddSummary(&ir.SimpleSummary{
			Name:  "flags",
			Value: 520,
		})

		c.module.AddSummary(&ir.SimpleSummary{
			Name:  "blockcount",
			Value: 0,
		})
	}

	return c.module
}

// Utils

func typeSubsString(subs []types.Substitution) string {
	var sb strings.Builder
	sb.WriteString("[")

	for i, s := range subs {
		if i > 0 {
			sb.WriteString(", ")
		}

		sb.WriteString(s.Param.Name)
		sb.WriteString(" -> ")
		sb.WriteString(s.Type.String())
	}

	sb.WriteString("]")
	return sb.String()
}

func (c *codegen) HasTypeParams(decl ast.Decl) bool {
	switch decl := decl.(type) {
	case *ast.Struct:
		typ := c.nodeTypes[decl].(*types.Struct)

		if len(typ.TypeParams) > 0 || typ.Generic != nil {
			return true
		}

	case *ast.Interface:
		typ := c.nodeTypes[decl].(*types.Interface)

		if len(typ.TypeParams) > 0 || typ.Generic != nil {
			return true
		}

	case *ast.Func:
		typ := c.nodeTypes[decl].(*types.Func)

		if len(typ.TypeParams) > 0 || typ.Generic != nil {
			return true
		}

		if impl, ok := decl.Parent().(*ast.Impl); ok && len(impl.TypeParams) > 0 {
			return true
		}
	}

	return false
}

func GlobalVarLinkName(g *ast.GlobalVar) string {
	linkName := g.GetLinkName()

	// Custom link name
	if linkName != "" {
		return linkName
	}

	// Normal
	file := ast.GetFile(g)

	sb := strings.Builder{}
	sb.WriteString("fb$")

	for _, entry := range file.Mod.Path {
		sb.WriteString(entry.Token.Text)
		sb.WriteString("::")
	}

	sb.WriteString(g.Name().Token.Text)

	return sb.String()
}

func FuncLinkName(f *ast.Func, typ *types.Func, in *types.Interface) string {
	linkName := f.GetLinkName()

	// Extern
	if f.IsExtern() {
		if linkName != "" {
			return linkName
		}

		return f.Name().Token.Text
	}

	// Custom link name
	if linkName != "" {
		return linkName
	}

	// Normal
	file := ast.GetFile(f)

	sb := strings.Builder{}
	sb.WriteString("fb$")

	for i, entry := range file.Mod.Path {
		if i > 0 {
			sb.WriteString("::")
		}

		sb.WriteString(entry.Token.Text)
	}

	if i, ok := f.Parent().(*ast.Impl); ok {
		name := ""

		if p, ok := i.Type.(*ast.PrimitiveType); ok {
			name = p.Kind.String()
		} else {
			path := i.Type.(*ast.IdentifierType).Path
			name = path[len(path)-1].Name.Token.Text
		}

		sb.WriteString("::")
		sb.WriteString(name)
		sb.WriteRune('$')

		// Interface disambiguation
		if in != nil {
			sb.WriteString(in.Name)

			if in.Generic != nil {
				sb.WriteString(":[")

				for j, sub := range in.Substitutions {
					if j > 0 {
						sb.WriteRune(',')
					}

					sb.WriteString(sub.Type.String())
				}

				sb.WriteRune(']')
			}

			sb.WriteRune('$')
		}
	} else {
		sb.WriteRune('$')
	}

	sb.WriteString(f.Name().Token.Text)

	// Generic
	if typ != nil && typ.Generic != nil {
		sb.WriteString(":[")

		for i, sub := range typ.Substitutions {
			if i > 0 {
				sb.WriteRune(',')
			}

			sb.WriteString(sub.Type.String())
		}

		sb.WriteRune(']')
	}

	return sb.String()
}

func (c *codegen) GetFuncInterface(f *ast.Func) *types.Interface {
	impl, ok := f.Parent().(*ast.Impl)
	if !ok || impl.Interface == nil {
		return nil
	}

	fileData, ok := c.fileDataMap[ast.GetFile(impl)]
	if !ok {
		return nil
	}

	raw, ok := fileData.NodeTypes[impl.Interface]
	if !ok {
		return nil
	}

	in, _ := c.ResolveType(raw).(*types.Interface)
	return in
}

func (c *codegen) GetGlobalVar(g *ast.GlobalVar, typ types.Type) *ir.GlobalVar {
	// Check already existing global variables
	name := GlobalVarLinkName(g)

	for gVar := range c.module.GlobalVars() {
		if gVar.Name == name {
			return gVar
		}
	}

	// Create extern global var
	return c.CreateGlobalVar(g, typ, true)
}

func (c *codegen) GetFunction(f *ast.Func, typ *types.Func, iface *types.Interface) *ir.Function {
	// Check already existing functions
	name := FuncLinkName(f, typ, iface)

	for fun := range c.module.Functions() {
		if fun.Name == name {
			return fun
		}
	}

	// Instantiation
	if typ.Generic != nil {
		fun := c.CreateFunction(f, typ, false, iface)
		fun.Flags = ir.DsoLocal | ir.LinkOnceODR

		c.pendingInstantiations = append(c.pendingInstantiations, pendingInstantiation{
			f,
			typ,
			fun,
		})

		return fun
	}

	// Create extern function
	return c.CreateFunction(f, typ, true, iface)
}

func (c *codegen) BitCast(value ir.Value, typ ir.Type) ir.Value {
	if value.Type() == typ {
		return value
	}

	// Bool (I1) -> I8
	if value.Type() == ir.I1 {
		if t, ok := typ.(*ir.IntegerType); ok && t.Bits == 8 {
			return c.emitter.Ext(ir.Unsigned, value, t)
		}
	}

	// I8 -> Bool (I1)
	if value.Type() == ir.I8 {
		if t, ok := typ.(*ir.IntegerType); ok && t.Bits == 1 {
			return c.emitter.Trunc(value, t)
		}
	}

	// Ptr -> Int
	if value.Type() == ir.Pointer {
		if _, ok := typ.(*ir.IntegerType); ok {
			return c.emitter.PtrToInt(value, typ)
		}
	}

	// Int -> Ptr
	if _, ok := value.Type().(*ir.IntegerType); ok {
		if typ == ir.Pointer {
			return c.emitter.IntToPtr(value, typ)
		}
	}

	// BitCast
	if !ir.IsAggregate(value.Type()) && !ir.IsAggregate(typ) && value.Type().Info().Size == typ.Info().Size {
		return c.emitter.BitCast(value, typ)
	}

	// Store + Load
	ptr := c.Alloca(value.Type(), "bitcast")
	c.emitter.Store(value, ptr)

	return c.emitter.Load(typ, ptr)
}

func (c *codegen) Alloca(typ ir.Type, name string) ir.Value {
	prevBlock := c.emitter.Block()
	c.emitter.Begin(c.bVariables)

	ptr := c.emitter.Alloca(typ, 1)
	ptr.SetName(name)

	c.emitter.Begin(prevBlock)
	return ptr
}
