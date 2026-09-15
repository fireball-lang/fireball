package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/core"
	"fireball/fb-core"
	"fireball/ir"
	"fireball/ir/eval"
	"fireball/sema"
	"fireball/types"
	"fmt"
	"hash/crc32"
	"path/filepath"
	"strings"
)

type FileData struct {
	ExprInfos   map[ast.Node]sema.ExprInfo
	NodeTypes   map[ast.Node]types.Type
	Evaluations map[ast.Expr]eval.Value
}

type pendingInstantiation struct {
	f   *ast.Func
	typ *types.Func
	fun *ir.Function
}

type Codegen struct {
	Module *ir.Module
	Uid    string

	Arch      abi.Arch
	CallConv  abi.CallConv
	ExprInfos map[ast.Node]sema.ExprInfo
	NodeTypes map[ast.Node]types.Type
	TypeEnv   *sema.TypeEnvironment

	Builtins fb_core.Builtins

	CompTime bool

	scope       symbolScope
	stringCount uint32

	Types   *TypeCache
	Emitter ir.Emitter

	FileRef ir.MetaRef
	UnitRef ir.MetaRef

	fun           *ir.Function
	funcTyp       *types.Func // type of the function currently being generated
	substitutions []types.Substitution
	returnPtr     ir.Value

	bVariables *ir.Block
	bEntry     *ir.Block

	bLoopBreak    *ir.Block
	bLoopContinue *ir.Block

	instantiations        *types.InstantiationCache
	pendingInstantiations []pendingInstantiation

	fileDataMap map[*ast.File]FileData
}

func New(module *ir.Module, file *ast.File, arch abi.Arch, callConv abi.CallConv, instantiations *types.InstantiationCache, typeEnv *sema.TypeEnvironment, fileDataMap map[*ast.File]FileData, builtins fb_core.Builtins, compTime bool) *Codegen {
	c := &Codegen{
		Module: module,
		Uid:    fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(module.Path))),

		Arch:      arch,
		CallConv:  callConv,
		ExprInfos: fileDataMap[file].ExprInfos,
		NodeTypes: fileDataMap[file].NodeTypes,
		TypeEnv:   typeEnv,

		Builtins: builtins,
		CompTime: compTime,

		instantiations: instantiations,
		fileDataMap:    fileDataMap,

		Types:   &TypeCache{Arch: arch, Module: module},
		Emitter: ir.Emitter{Module: module},
	}

	// Setup meta

	c.FileRef = c.Module.AddMeta(&ir.FileMeta{
		Path: module.Path,
	})
	c.Emitter.PushScope(c.FileRef)

	c.Types.FileRef = c.FileRef

	c.UnitRef = module.AddMeta(&ir.CompileUnitMeta{
		File:        c.FileRef,
		Producer:    "fireball",
		IsOptimized: false,
		Enums:       0,
		Globals:     0,
		Imports:     0,
	})

	c.Module.AddNamedMetaRefs(
		"llvm.dbg.cu",
		c.UnitRef,
	)

	return c
}

func Generate(file *ast.File, arch abi.Arch, callConv abi.CallConv, instantiations *types.InstantiationCache, typeEnv *sema.TypeEnvironment, fileDataMap map[*ast.File]FileData, builtins fb_core.Builtins, path string, compTime bool) *ir.Module {
	defer core.Scope()()

	module := ir.NewModule()
	module.Path = path

	c := New(module, file, arch, callConv, instantiations, typeEnv, fileDataMap, builtins, compTime)

	// Setup meta

	var retainedTypeRefs []ir.RawMetaValue

	for _, decl := range file.Decls {
		if s, ok := decl.(*ast.Struct); ok && len(s.TypeParams) == 0 {
			ref := c.Types.GetMeta(c.NodeTypes[s])
			retainedTypeRefs = append(retainedTypeRefs, ir.RawMetaValue{Ref: ref})
		}
	}

	if len(retainedTypeRefs) > 0 {
		ref := c.Module.AddMeta(&ir.RawMeta{Values: retainedTypeRefs})
		module.GetMeta(c.UnitRef).(*ir.CompileUnitMeta).RetainedTypes = ref
	}

	// Function / Global Var declarations

	c.scope.Push()

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GlobalVar:
			typ := c.NodeTypes[decl]
			c.scope.Add(decl.Name().Token.Text, c.CreateGlobalVar(decl, typ, false))

		case *ast.Impl:
			var in *types.Interface
			if decl.Interface != nil {
				in, _ = c.NodeTypes[decl.Interface].(*types.Interface)
			}

			for _, f := range decl.Methods {
				if !c.HasTypeParams(f) {
					typ := c.NodeTypes[f].(*types.Func)
					c.CreateFunction(f, typ, false, in)
				}
			}

		case *ast.Func:
			if !c.HasTypeParams(decl) && ast.GetAttribute[*ast.Intrinsic](decl) == nil {
				typ := c.NodeTypes[decl].(*types.Func)
				c.scope.Add(decl.Name().Token.Text, c.CreateFunction(decl, typ, false, nil))
			}
		}
	}

	// V-Tables

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Impl:
			if decl.Interface != nil && len(decl.TypeParams) == 0 {
				in := c.NodeTypes[decl.Interface].(*types.Interface)

				var typ types.Type

				if p, ok := decl.Type.(*ast.PrimitiveType); ok {
					typ = types.GetPrimitive(p.Kind)
				} else {
					typ = c.NodeTypes[decl.Type]
				}

				c.CreateVTable(typ, in, false)
			}
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
			c.CreateTypeInfo(c.NodeTypes[decl], false)
		}
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Enum:
			c.CreateTypeInfo(c.NodeTypes[decl], false)

		case *ast.Struct:
			if !c.HasTypeParams(decl) {
				c.CreateTypeInfo(c.NodeTypes[decl], false)
			}
		}
	}

	// Function / Global Var definitions

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GlobalVar:
			if !core.IsNil(decl.Initializer) {
				typ := c.NodeTypes[decl]
				gVar := c.scope.Get(decl.Name().Token.Text).(*ir.GlobalVar)

				gVar.Initializer = c.GetIrValue(fileDataMap[file].Evaluations[decl.Initializer], typ)
			}

		case *ast.Impl:
			var in *types.Interface
			if decl.Interface != nil {
				in, _ = c.NodeTypes[decl.Interface].(*types.Interface)
			}

			for _, f := range decl.Methods {
				if !c.HasTypeParams(f) {
					typ := c.NodeTypes[f].(*types.Func)
					fun := c.GetFunction(f, typ, in)

					c.VisitFunc(f, typ, fun)
				}
			}

		case *ast.Func:
			if !c.HasTypeParams(decl) && ast.GetAttribute[*ast.Intrinsic](decl) == nil {
				typ := c.NodeTypes[decl].(*types.Func)
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
			c.ExprInfos = fd.ExprInfos
			c.NodeTypes = fd.NodeTypes
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

	return c.Module
}

// Utils

func (c *Codegen) EmitPanic(_ ast.Node, format string, args ...any) {
	str := fmt.Sprintf(format, args...)
	msg := c.StringView([]rune(str))

	f := c.Builtins.PanicNode
	typ := c.Builtins.PanicType

	callee := c.GetFunction(f, typ, nil)

	c.EmitCall(callee, callee.Signature, typ, nil, []ir.Value{msg}, typ.Params, typ.Returns)
}

func getPointee(typ types.Type) (types.Type, bool) {
	switch typ := typ.(type) {
	case *types.Null:
		return types.PrimitiveVoid, true
	case *types.Reference:
		return typ.Pointee, true
	case *types.Pointer:
		return typ.Pointee, true
	default:
		return nil, false
	}
}

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

func (c *Codegen) HasTypeParams(decl ast.Decl) bool {
	switch decl := decl.(type) {
	case *ast.Struct:
		typ := c.NodeTypes[decl].(*types.Struct)

		if len(typ.TypeParams) > 0 || typ.Generic != nil {
			return true
		}

	case *ast.Interface:
		typ := c.NodeTypes[decl].(*types.Interface)

		if len(typ.TypeParams) > 0 || typ.Generic != nil {
			return true
		}

	case *ast.Func:
		typ := c.NodeTypes[decl].(*types.Func)

		if len(typ.TypeParams) > 0 || typ.Generic != nil {
			return true
		}

		if impl, ok := decl.Parent().(*ast.Impl); ok && len(impl.TypeParams) > 0 {
			return true
		}
	}

	return false
}

func ConstLinkName(c *ast.Const) string {
	// Normal
	file := ast.GetFile(c)

	sb := strings.Builder{}
	sb.WriteString("fb$")

	for _, entry := range file.Mod.Path {
		sb.WriteString(entry.Token.Text)
		sb.WriteString("::")
	}

	sb.WriteString(c.Name().Token.Text)

	return sb.String()
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

func (c *Codegen) GetConst(decl *ast.Const) *ir.GlobalVar {
	// Check already existing constants
	name := ConstLinkName(decl)

	if gVar := c.Module.GetGlobalVar(name); gVar != nil {
		return gVar
	}

	// Create constant
	typ := c.ResolveType(c.fileDataMap[ast.GetFile(decl)].NodeTypes[decl.Type])
	val := c.fileDataMap[ast.GetFile(decl)].Evaluations[decl.Value]

	gVar := c.GlobalVar(name, ir.Constant|ir.UnnamedAddr|ir.LinkOnce, c.GetIrValue(val, typ))

	return gVar
}

func (c *Codegen) GetIrValue(val eval.Value, typ types.Type) ir.Value {
	if comp, ok := typ.(types.Composed); ok {
		typ = comp.Underlying()
	}

	switch val := val.(type) {
	case *eval.IntValue:
		switch typ.(*types.Primitive).Kind {
		case types.Bool:
			if val.Value == 1 {
				return ir.True
			}

			return ir.False

		case types.U8:
			return &ir.Integer{Value: core.Unsigned(false, val.Value&0xFF), Typ: ir.I8}
		case types.U16:
			return &ir.Integer{Value: core.Unsigned(false, val.Value&0xFFFF), Typ: ir.I16}
		case types.U32:
			return &ir.Integer{Value: core.Unsigned(false, val.Value&0xFFFFFFFF), Typ: ir.I32}
		case types.U64:
			return &ir.Integer{Value: core.Unsigned(false, val.Value&0xFFFFFFFFFFFFFFFF), Typ: ir.I64}

		case types.I8:
			return &ir.Integer{Value: core.TwosComplementWidth(val.Value, 8), Typ: ir.I8}
		case types.I16:
			return &ir.Integer{Value: core.TwosComplementWidth(val.Value, 16), Typ: ir.I16}
		case types.I32:
			return &ir.Integer{Value: core.TwosComplementWidth(val.Value, 32), Typ: ir.I32}
		case types.I64:
			return &ir.Integer{Value: core.TwosComplementWidth(val.Value, 64), Typ: ir.I64}

		default:
			panic("codegen.Codegen.GetIrValue() - Invalid integer primitive kind")
		}

	case *eval.FloatValue:
		if typ.(*types.Primitive).Kind == types.F64 {
			return &ir.DoubleV{Value: val.Value}
		}

		return &ir.FloatV{Value: float32(val.Value)}

	case *eval.StringValue:
		return c.StringPtr(val.Value)

	case *eval.NullValue:
		return &ir.Null{}

	case *eval.AggregateValue:
		if typ, ok := typ.(*types.Array); ok {
			elements := make([]ir.Value, len(val.Values))

			for i, value := range val.Values {
				elements[i] = c.GetIrValue(value, typ.Element)
			}

			return &ir.Array{Elements: elements}
		}

		fields := make([]ir.Value, len(val.Values))
		info := c.Arch.Info(typ)

		for i, value := range val.Values {
			fields[i] = c.GetIrValue(value, typ.(*types.Struct).Fields[info.Fields[i].Index].Type)
		}

		return &ir.Struct{Typ: c.Types.Get(typ), Fields: fields}

	case *eval.GlobalValue:
		if strings.HasPrefix(val.Name, "fb$link_name$") {
			return c.GetTypeInfo(val.Data.(types.Type))
		}

		gVar := c.Module.GetGlobalVar(val.Name)

		if gVar == nil {
			gVar = c.Module.NewGlobalVar(val.Name, val.Typ)
			gVar.Flags = ir.External | val.Flags
		}

		return gVar

	case *eval.FuncValue:
		f := val.Data.(*ast.Func)
		typ := c.fileDataMap[ast.GetFile(f)].NodeTypes[f].(*types.Func)
		in := c.GetFuncInterface(f)

		return c.GetFunction(f, typ, in)

	default:
		panic("codegen.Codegen.GetIrValue() - Invalid eval.Value")
	}
}

func (c *Codegen) GetFuncInterface(f *ast.Func) *types.Interface {
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

func (c *Codegen) GetGlobalVar(g *ast.GlobalVar, typ types.Type) *ir.GlobalVar {
	// Check already existing global variables
	name := GlobalVarLinkName(g)

	if gVar := c.Module.GetGlobalVar(name); gVar != nil {
		return gVar
	}

	// Create extern global var
	return c.CreateGlobalVar(g, typ, true)
}

func (c *Codegen) GetFunction(f *ast.Func, typ *types.Func, iface *types.Interface) *ir.Function {
	// Check already existing functions
	name := FuncLinkName(f, typ, iface)

	if fun := c.Module.GetFunction(name); fun != nil {
		return fun
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

func (c *Codegen) BitCast(value ir.Value, typ ir.Type) ir.Value {
	if value.Type() == typ {
		return value
	}

	// Bool (I1) -> I8
	if value.Type() == ir.I1 {
		if t, ok := typ.(*ir.IntegerType); ok && t.Bits == 8 {
			return c.Emitter.Ext(ir.Unsigned, value, t)
		}
	}

	// I8 -> Bool (I1)
	if value.Type() == ir.I8 {
		if t, ok := typ.(*ir.IntegerType); ok && t.Bits == 1 {
			return c.Emitter.Trunc(value, t)
		}
	}

	// Ptr -> Int
	if value.Type() == ir.Pointer {
		if _, ok := typ.(*ir.IntegerType); ok {
			return c.Emitter.PtrToInt(value, typ)
		}
	}

	// Int -> Ptr
	if _, ok := value.Type().(*ir.IntegerType); ok {
		if typ == ir.Pointer {
			return c.Emitter.IntToPtr(value)
		}
	}

	// BitCast
	if !ir.IsAggregate(value.Type()) && !ir.IsAggregate(typ) && value.Type().Info().Size == typ.Info().Size {
		return c.Emitter.BitCast(value, typ)
	}

	// Store + Load
	ptr := c.Alloca(value.Type(), "bitcast")
	c.Emitter.Store(value, ptr)

	return c.Emitter.Load(typ, ptr)
}

func (c *Codegen) Alloca(typ ir.Type, name string) ir.Value {
	prevBlock := c.Emitter.Block()
	c.Emitter.Begin(c.bVariables)

	ptr := c.Emitter.Alloca(typ, 1)
	ptr.SetName(name)

	c.Emitter.Begin(prevBlock)
	return ptr
}
