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

func (c *codegen) VisitFunc(f *ast.Func, typ *types.Func, fun *ir.Function) {
	if core.IsNil(f.Body) {
		return
	}

	// Meta

	ref := c.module.AddMeta(&ir.SubprogramMeta{
		Name:     f.Name().Token.Text,
		LinkName: fun.Name,
		Type:     c.module.GetMeta(c.types.GetMeta(typ)).(*ir.DerivedTypeMeta).Base,
		Scope:    c.emitter.PeekScope(),
		Unit:     c.unitRef,
		File:     c.fileRef,
		Line:     f.Range().Start.Line,
	})

	fun.SetMeta(ref)

	c.emitter.PushScope(ref)
	defer c.emitter.PopScope()

	// Blocks
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

	// Receiver
	params := typ.Params

	if f.Receiver != nil {
		value := fun.ParamValues[paramI]
		paramI++

		c.emitter.SetDebugLocation(f.Range().Start)

		ptr := c.emitter.Alloca(ir.Pointer, 1)
		ptr.SetName("param.self")

		c.emitDbgDeclare("self", params[0], ptr, 0, f.Receiver)

		c.emitter.Store(value, ptr)

		c.scope.Add("self", ptr)

		params = params[1:]
	}

	// Parameters
	for i, param := range params {
		name := fun.ParamNames[paramI]
		value := fun.ParamValues[paramI]
		typ := c.types.Get(param)

		c.emitter.SetDebugLocation(f.Params[i].Range().Start)

		ptr := c.emitter.Alloca(typ, 1)
		ptr.SetName("param." + name)

		c.emitDbgDeclare(name, param, ptr, uint32(i+1), f.Params[i])

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
	c.funcTyp = typ
	c.funDoesIndirectDispatch = false

	c.GenerateStmt(f.Body)
	c.emitter.Ret(nil)

	c.emitter.Begin(c.bVariables)
	c.emitter.Br(bEntry)

	c.fun = nil
	c.funcTyp = nil
	c.returnPtr = nil
	c.bVariables = nil

	c.scope.Pop()

	// Summary
	if ref, ok := c.functionSummaries[fun.Name]; ok {
		funSum := c.module.GetSummary(ref).(*ir.FunctionSummary)

		for _, block := range fun.Blocks {
			funSum.InstructionCount += block.InstructionCount
		}

		if c.funDoesIndirectDispatch {
			funSum.Flags |= ir.FuncHasUnknownCall
		}

		funSum.Calls = c.summaryCalls
		c.summaryCalls = nil

		funSum.Refs = c.summaryRefs
		c.summaryRefs = nil
	}
}

// Utils

func (c *codegen) CreateGlobalVar(g *ast.GlobalVar, typ types.Type, declare bool) *ir.GlobalVar {
	name := GlobalVarLinkName(g)
	t := c.types.Get(typ)

	gVar := c.module.NewGlobalVar(name, t)

	if g.IsExtern() || declare {
		gVar.Flags = ir.External
	} else {
		gVar.Initializer = &ir.ZeroInitializer{Typ: t}

		ref := c.module.AddMeta(&ir.GlobalVariableMeta{
			Name:     g.Name().Token.Text,
			LinkName: name,
			Type:     c.types.GetMeta(typ),
			Scope:    c.emitter.PeekScope(),
			File:     c.fileRef,
			Line:     g.Range().Start.Line,
		})

		ref = c.module.AddMeta(&ir.GlobalVariableExpressionMeta{Var: ref})

		cu := c.module.GetMeta(c.unitRef).(*ir.CompileUnitMeta)
		var globals *ir.RawMeta

		if cu.Globals.Valid() {
			globals = c.module.GetMeta(cu.Globals).(*ir.RawMeta)
		} else {
			globals = &ir.RawMeta{}
			cu.Globals = c.module.AddMeta(globals)
		}

		gVar.SetMeta(ref)
		globals.Values = append(globals.Values, ir.RawMetaValue{Ref: ref})
	}

	// Summary

	if !declare && !g.IsExtern() && c.moduleSummaryRef.Valid() {
		c.module.AddSummary(&ir.VariableSummary{
			Module: c.moduleSummaryRef,
			Name:   name,
			LinkFlags: ir.LinkSummaryFlags{
				Linkage:             ir.LinkageExternal,
				Visibility:          ir.VisibilityDefault,
				NotEligibleToImport: false,
				Live:                false,
				DsoLocal:            true,
				CanAutoHide:         false,
				ImportType:          ir.ImportDefinition,
			},
			Flags: 0,
			Refs:  nil,
		})
	}

	return gVar
}

func (c *codegen) CreateFunction(f *ast.Func, typ *types.Func, declare bool, in *types.Interface) *ir.Function {
	sig := &ir.Signature{
		Params:  make([]ir.Type, 0, len(f.Params)+1),
		VarArgs: f.VarArgs,
	}

	// Params
	params := typ.Params
	paramNames := make([]string, 0, len(f.Params)+1)

	if f.IsMethod() && f.Receiver != nil {
		sig.Params = append(sig.Params, ir.Pointer)
		paramNames = append(paramNames, "self")
		params = params[1:]
	}

	for i, param := range f.Params {
		classes, info := c.callConv.Classify(c.arch, params[i])

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
			sig.SRet = c.types.Get(typ.Returns)

			sig.Params = slices.Insert(sig.Params, 0, ir.Type(ir.Pointer))
			paramNames = slices.Insert(paramNames, 0, "sret")
		} else {
			sig.Returns = getTypeForClasses(classes, info.Size)
		}
	}

	// Function
	fun := c.module.NewFunction(FuncLinkName(f, typ, in), sig, paramNames)

	if f.IsExtern() || declare {
		fun.Flags = ir.Declare
	} else {
		fun.Flags = ir.DsoLocal
	}

	// Summary

	if !declare && !f.IsExtern() && c.moduleSummaryRef.Valid() {
		linkage := ir.LinkageExternal
		if typ.Generic != nil {
			linkage = ir.LinkageLinkOnceODR
		}

		ref := c.module.AddSummary(&ir.FunctionSummary{
			Module: c.moduleSummaryRef,
			Name:   fun.Name,
			LinkFlags: ir.LinkSummaryFlags{
				Linkage:             linkage,
				Visibility:          ir.VisibilityDefault,
				NotEligibleToImport: false,
				Live:                false,
				DsoLocal:            true,
				CanAutoHide:         false,
				ImportType:          ir.ImportDefinition,
			},
			Flags: ir.FuncNoUnwind,
		})

		c.functionSummaries[fun.Name] = ref
	}

	return fun
}
