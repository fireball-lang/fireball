package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/ir"
	"fireball/sema"
	"fireball/types"
)

type codegen struct {
	module *ir.Module

	abi       abi.ABI
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

func Generate(decls []ast.Decl, abi abi.ABI, exprInfos map[ast.Expr]sema.ExprInfo, nodeTypes map[ast.Node]types.Type) *ir.Module {
	module := ir.NewModule()

	c := codegen{
		module: module,

		abi:       abi,
		exprInfos: exprInfos,
		nodeTypes: nodeTypes,

		types:   &TypeCache{Module: module},
		emitter: ir.Emitter{Module: module},
	}

	c.scope.Push()

	for _, decl := range decls {
		if f, ok := decl.(*ast.Func); ok {
			c.scope.Add(decl.Name(), c.CreateFunction(f))
		}
	}

	for _, decl := range decls {
		if f, ok := decl.(*ast.Func); ok {
			c.VisitFunc(f)
		}
	}

	c.scope.Pop()

	return c.module
}

// Utils

func (c *codegen) BitCast(value ir.Value, typ ir.Type) ir.Value {
	if value.Type() == typ {
		return value
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
