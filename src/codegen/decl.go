package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/types"
	"slices"
)

// Visitor

func (c *codegen) VisitFunc(f *ast.Func) {
	if core.IsNil(f.Body) {
		return
	}

	// Blocks
	typ := c.nodeTypes[f].(*types.Func)
	fun := c.scope.Get(f.Name()).(*ir.Function)

	c.bVariables = fun.NewBlock("fun.variables")
	bEntry := fun.NewBlock("fun.entry")

	// Variables
	c.emitter.Begin(c.bVariables)

	c.scope.Push()

	paramI := 0

	// Return value
	{
		classes, _ := c.callConv.Classify(c.arch, typ.Returns)

		if len(classes) == 1 && classes[0] == abi.Memory {
			c.returnPtr = fun.ParamValues[paramI]
			paramI++
		}
	}

	// Parameters
	for _, param := range typ.Params {
		name := fun.ParamNames[paramI]
		value := fun.ParamValues[paramI]
		typ := c.types.Get(param)

		ptr := c.emitter.Alloca(typ, 1)
		ptr.SetName("param." + name)

		classes, _ := c.callConv.Classify(c.arch, param)

		if len(classes) == 1 && classes[0] == abi.Memory {
			value = c.emitter.Load(typ, value)
		}

		c.emitter.Store(value, ptr)

		c.scope.Add(name, ptr)

		paramI++
	}

	// Body
	c.emitter.Begin(bEntry)

	c.fun = fun

	f.Body.VisitStmt(c)
	c.emitter.Ret(nil)

	c.emitter.Begin(c.bVariables)
	c.emitter.Br(bEntry)

	c.fun = nil
	c.returnPtr = nil
	c.bVariables = nil

	c.scope.Pop()
}

// Utils

func (c *codegen) CreateFunction(f *ast.Func) *ir.Function {
	typ := c.nodeTypes[f].(*types.Func)

	sig := &ir.Signature{
		Params:  make([]ir.Type, 0, len(f.Params)+1),
		VarArgs: f.VarArgs,
	}

	// Params
	paramNames := make([]string, 0, len(f.Params)+1)

	for i, param := range f.Params {
		classes, info := c.callConv.Classify(c.arch, typ.Params[i])

		if len(classes) == 1 && classes[0] == abi.Memory {
			sig.Params = append(sig.Params, ir.Pointer)
		} else {
			sig.Params = append(sig.Params, getTypeForClasses(classes, info.Size))
		}

		paramNames = append(paramNames, param.Name.Token.Text)
	}

	// Returns
	{
		classes, info := c.callConv.Classify(c.arch, typ.Returns)

		if len(classes) == 1 && classes[0] == abi.Memory {
			sig.Returns = ir.Void
			sig.Params = slices.Insert(sig.Params, 0, ir.Type(ir.Pointer))
			paramNames = slices.Insert(paramNames, 0, "sret")
		} else {
			sig.Returns = getTypeForClasses(classes, info.Size)
		}
	}

	// Function
	fun := c.module.NewFunction(f.Name(), sig, paramNames)

	if core.IsNil(f.Body) {
		fun.Flags = ir.Declare
	} else {
		fun.Flags = ir.DsoLocal
	}

	return fun
}
