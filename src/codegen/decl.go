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

func (c *Codegen) BeginFunc(f *ast.Func, typ *types.Func, fun *ir.Function) {
	// Meta

	ref := c.Module.AddMeta(&ir.SubprogramMeta{
		Name:     f.Name().Token.Text,
		LinkName: fun.Name,
		Type:     c.Module.GetMeta(c.Types.GetMeta(typ)).(*ir.DerivedTypeMeta).Base,
		Scope:    c.Emitter.PeekScope(),
		Unit:     c.UnitRef,
		File:     c.FileRef,
		Line:     f.Range().Start.Line,
	})

	fun.SetMeta(ref)

	c.Emitter.PushScope(ref)

	// Blocks
	c.bVariables = fun.NewBlock("fun.variables")
	c.bEntry = fun.NewBlock("fun.entry")

	// Variables
	c.Emitter.Begin(c.bVariables)

	c.scope.Push()

	paramI := 0

	// Return value
	if !c.CompTime {
		classes, _ := c.CallConv.Classify(c.Arch, typ.Returns)

		if len(classes) == 1 && classes[0] == abi.Memory {
			c.returnPtr = fun.ParamValues[paramI]
			paramI++
		}
	}

	// Receiver
	params := typ.Params

	if f.Receiver != nil {
		value := fun.ParamValues[paramI]
		paramI++

		c.Emitter.SetDebugLocation(f.Range().Start)

		ptr := c.Emitter.Alloca(ir.Pointer, 1)
		ptr.SetName("param.self")

		c.emitDbgDeclare("self", params[0], ptr, 0, f.Receiver)

		c.Emitter.Store(value, ptr)

		c.scope.Add("self", ptr)

		params = params[1:]
	}

	// Parameters
	for i, param := range params {
		name := fun.Params[paramI].Name
		value := fun.ParamValues[paramI]
		typ := c.Types.Get(param)

		c.Emitter.SetDebugLocation(f.Params[i].Range().Start)

		ptr := c.Emitter.Alloca(typ, 1)
		ptr.SetName("param." + name)

		c.emitDbgDeclare(name, param, ptr, uint32(i+1), f.Params[i])

		classes, _ := c.CallConv.Classify(c.Arch, param)

		if len(classes) == 1 && classes[0] == abi.Memory {
			value = c.Emitter.Load(typ, value)
		}

		c.Emitter.Store(value, ptr)

		c.scope.Add(name, ptr)

		paramI++
	}

	// Body
	c.Emitter.Begin(c.bEntry)

	c.fun = fun
	c.funcTyp = typ
}

func (c *Codegen) EndFunc() {
	c.Emitter.Begin(c.bVariables)
	c.Emitter.Br(c.bEntry)

	c.fun = nil
	c.funcTyp = nil
	c.returnPtr = nil

	c.bEntry = nil
	c.bVariables = nil

	c.scope.Pop()
	c.Emitter.PopScope()
}

func (c *Codegen) VisitFunc(f *ast.Func, typ *types.Func, fun *ir.Function) {
	if core.IsNil(f.Body) {
		return
	}

	c.BeginFunc(f, typ, fun)

	c.GenerateStmt(f.Body)
	c.Emitter.Ret(nil)

	c.EndFunc()
}

// Utils

func (c *Codegen) CreateGlobalVar(g *ast.GlobalVar, typ types.Type, declare bool) *ir.GlobalVar {
	name := GlobalVarLinkName(g)
	t := c.Types.Get(typ)

	gVar := c.Module.NewGlobalVar(name, t)

	if g.IsExtern() || declare {
		gVar.Flags = ir.External
	} else {
		gVar.Initializer = &ir.ZeroInitializer{Typ: t}

		ref := c.Module.AddMeta(&ir.GlobalVariableMeta{
			Name:     g.Name().Token.Text,
			LinkName: name,
			Type:     c.Types.GetMeta(typ),
			Scope:    c.Emitter.PeekScope(),
			File:     c.FileRef,
			Line:     g.Range().Start.Line,
		})

		ref = c.Module.AddMeta(&ir.GlobalVariableExpressionMeta{Var: ref})

		cu := c.Module.GetMeta(c.UnitRef).(*ir.CompileUnitMeta)
		var globals *ir.RawMeta

		if cu.Globals.Valid() {
			globals = c.Module.GetMeta(cu.Globals).(*ir.RawMeta)
		} else {
			globals = &ir.RawMeta{}
			cu.Globals = c.Module.AddMeta(globals)
		}

		gVar.SetMeta(ref)
		globals.Values = append(globals.Values, ir.RawMetaValue{Ref: ref})
	}

	return gVar
}

func (c *Codegen) CreateFunction(f *ast.Func, typ *types.Func, declare bool, in *types.Interface) *ir.Function {
	// Check already created extern functions
	name := FuncLinkName(f, typ, in)

	if f.IsExtern() {
		if fun := c.Module.GetFunction(name); fun != nil {
			return fun
		}
	}

	// Params
	sig := &ir.Signature{
		Params:  make([]ir.Type, 0, len(f.Params)+1),
		VarArgs: f.VarArgs,
	}

	paramTypes := typ.Params
	params := make([]ir.Param, 0, len(f.Params)+1)

	if f.IsMethod() && f.Receiver != nil {
		sig.Params = append(sig.Params, ir.Pointer)

		params = append(params, ir.Param{
			Name:       "self",
			Attributes: getTypeParamAttributes(paramTypes[0]),
		})

		paramTypes = paramTypes[1:]
	}

	for i, param := range f.Params {
		var attrs ir.ParamAttribute

		if c.CompTime {
			sig.Params = append(sig.Params, c.Types.Get(paramTypes[i]))
		} else {
			classes, info := c.CallConv.Classify(c.Arch, paramTypes[i])

			if len(classes) == 1 && classes[0] == abi.Memory {
				sig.Params = append(sig.Params, ir.Pointer)
				attrs = ir.NonNull
			} else {
				sig.Params = append(sig.Params, getTypeForClasses(classes, info.Size))
			}
		}

		params = append(params, ir.Param{
			Name:       param.Name.Token.Text,
			Attributes: attrs,
		})
	}

	// Returns
	if c.CompTime {
		sig.Returns = c.Types.Get(typ.Returns)
	} else {
		classes, info := c.CallConv.Classify(c.Arch, typ.Returns)

		if len(classes) == 1 && classes[0] == abi.Memory {
			sig.Returns = ir.Void
			sig.SRet = c.Types.Get(typ.Returns)

			sig.Params = slices.Insert(sig.Params, 0, ir.Type(ir.Pointer))
			params = slices.Insert(params, 0, ir.Param{Name: "sret", Attributes: ir.NonNull | ir.WriteOnly})
		} else {
			sig.Returns = getTypeForClasses(classes, info.Size)
		}
	}

	// Function
	fun := c.Module.NewFunction(name, sig, params)
	fun.Data = f

	if f.IsExtern() || declare {
		fun.Flags = ir.Declare
	} else {
		fun.Flags = ir.DsoLocal
	}

	return fun
}

func getTypeParamAttributes(typ types.Type) ir.ParamAttribute {
	switch typ := typ.(type) {
	case *types.Reference:
		if typ.Mutable {
			return ir.NonNull
		}

		return ir.NonNull | ir.ReadOnly

	case *types.Pointer:
		if !typ.Mutable {
			return ir.ReadOnly
		}

	case *types.Func:
		return ir.NonNull | ir.ReadOnly
	}

	return 0
}
