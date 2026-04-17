package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/ir"
	"fireball/sema"
	"fireball/types"
	"fmt"
	"hash/crc32"
	"strings"
)

type codegen struct {
	module *ir.Module
	uid    string

	arch      abi.Arch
	callConv  abi.CallConv
	exprInfos map[ast.Expr]sema.ExprInfo
	nodeTypes map[ast.Node]types.Type

	scope       symbolScope
	stringCount uint32

	types   *TypeCache
	emitter ir.Emitter

	fileRef ir.MetaRef
	unitRef ir.MetaRef

	moduleSummaryRef  ir.SummaryRef
	functionSummaries map[*ast.Func]ir.SummaryRef

	fun        *ir.Function
	returnPtr  ir.Value
	bVariables *ir.Block

	bLoopBreak    *ir.Block
	bLoopContinue *ir.Block

	summaryCalls []ir.FunctionSummaryCall
	summaryRefs  []ir.SummaryRef
}

func Generate(file *ast.File, arch abi.Arch, callConv abi.CallConv, exprInfos map[ast.Expr]sema.ExprInfo, nodeTypes map[ast.Node]types.Type, path string, summary bool) *ir.Module {
	module := ir.NewModule()
	module.Path = path

	c := codegen{
		module: module,
		uid:    fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(path))),

		arch:      arch,
		callConv:  callConv,
		exprInfos: exprInfos,
		nodeTypes: nodeTypes,

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
		if s, ok := decl.(*ast.Struct); ok {
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

		c.functionSummaries = make(map[*ast.Func]ir.SummaryRef)
	}

	// Emit functions

	c.scope.Push()

	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.Func); ok {
			c.scope.Add(decl.Name(), c.CreateFunction(f, false))
		}
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Impl:
			for _, f := range decl.Functions {
				typ := c.nodeTypes[f].(*types.Func)
				fun := c.CreateFunction(f, false)

				c.VisitFunc(f, typ, fun)
			}

		case *ast.Func:
			typ := c.nodeTypes[decl].(*types.Func)
			fun := c.scope.Get(decl.Name()).(*ir.Function)

			c.VisitFunc(decl, typ, fun)
		}
	}

	c.scope.Pop()

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

func FuncLinkName(f *ast.Func) string {
	linkName := f.GetLinkName()

	// Extern
	if f.IsExtern() {
		if linkName != "" {
			return linkName
		}

		return f.Name()
	}

	// Custom link name
	if linkName != "" {
		return linkName
	}

	// Normal
	file := ast.GetFile(f)

	sb := strings.Builder{}
	sb.WriteString("fb$")

	for _, entry := range file.Mod.Path.Entries {
		sb.WriteString(entry.Token.Text)
		sb.WriteString("::")
	}

	if i, ok := f.Parent().(*ast.Impl); ok {
		t := i.Type.(*ast.IdentifierType)

		sb.WriteString(t.Path.LastName())
		sb.WriteString("::")
	}

	sb.WriteString(f.Name())

	return sb.String()
}

// Utils

func (c *codegen) GetFunction(f *ast.Func) *ir.Function {
	// Check already existing functions
	name := FuncLinkName(f)

	for fun := range c.module.Functions() {
		if fun.Name == name {
			return fun
		}
	}

	// Create extern function
	return c.CreateFunction(f, true)
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
	if !ir.IsAggregate(value.Type()) && !ir.IsAggregate(typ) {
		return c.emitter.BitCast(value, typ)
	}

	// Store + Load
	ptr := c.Alloca(typ, "bitcast")
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
