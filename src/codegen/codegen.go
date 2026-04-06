package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/sema"
	"fireball/types"
	"strings"
)

type codegen struct {
	module *ir.Module

	arch      abi.Arch
	callConv  abi.CallConv
	exprInfos map[ast.Expr]sema.ExprInfo
	nodeTypes map[ast.Node]types.Type

	scope       symbolScope
	stringCount uint32

	types   *TypeCache
	emitter ir.Emitter

	fun        *ir.Function
	returnPtr  ir.Value
	bVariables *ir.Block

	bLoopBreak    *ir.Block
	bLoopContinue *ir.Block

	value ir.Value
}

func Generate(file *ast.File, arch abi.Arch, callConv abi.CallConv, exprInfos map[ast.Expr]sema.ExprInfo, nodeTypes map[ast.Node]types.Type) *ir.Module {
	module := ir.NewModule()

	c := codegen{
		module: module,

		arch:      arch,
		callConv:  callConv,
		exprInfos: exprInfos,
		nodeTypes: nodeTypes,

		types:   &TypeCache{Module: module},
		emitter: ir.Emitter{Module: module},
	}

	c.scope.Push()

	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.Func); ok {
			c.scope.Add(decl.Name(), c.CreateFunction(f))
		}
	}

	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.Func); ok {
			c.VisitFunc(f)
		}
	}

	c.scope.Pop()

	return c.module
}

func FuncLinkName(f *ast.Func) string {
	if core.IsNil(f.Body) {
		return f.Name()
	}

	file := ast.GetFile(f)

	sb := strings.Builder{}
	sb.WriteString("fb$")

	for _, entry := range file.Mod.Path.Entries {
		sb.WriteString(entry.Token.Text)
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
	fun := c.CreateFunction(f)
	fun.Flags = ir.Declare

	return fun
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
