package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/symbols"
	"fireball/types"
	"fmt"
	"slices"
)

func isInterfaceMethod(f *ast.Func) bool {
	_, ok := f.Parent().(*ast.Interface)
	return ok
}

func isInterfaceStatic(f *ast.Func, callee ast.Expr) bool {
	if _, ok := f.Parent().(*ast.Interface); !ok {
		return false
	}

	ident, ok := callee.(*ast.Identifier)
	return ok && len(ident.Path) >= 2
}

func (c *codegen) ResolveReceiver(node ast.Expr) ir.Value {
	if _, ok := c.UnderlyingExprType(node).(*types.Pointer); ok {
		return c.Load(node)
	}

	value := c.GenerateExpr(node)
	return c.ReceiverToPointer(value, c.UnderlyingExprType(node), c.exprInfos[node].Address)
}

func (c *codegen) ReceiverToPointer(value ir.Value, typ types.Type, addressable bool) ir.Value {
	if addressable {
		return value
	}

	irTyp := c.types.Get(typ)
	ptr := c.Alloca(irTyp, "call.self")
	c.emitter.Store(value, ptr)

	return ptr
}

func (c *codegen) ResolveInterfaceCallee(m *ast.Member) (callee ir.Value, receiver ir.Value) {
	interfaceValue := c.Load(m.Expr)
	interfaceType := c.ExprType(m.Expr).(*types.Interface)
	return c.LookupInterfaceMethod(interfaceType, interfaceValue, m.Name.Token.Text)
}

func (c *codegen) LookupInterfaceMethod(iface *types.Interface, interfaceValue ir.Value, methodName string) (callee ir.Value, receiver ir.Value) {
	receiver = c.emitter.ExtractValue(interfaceValue, 0)
	vtablePtr := c.emitter.ExtractValue(interfaceValue, 1)

	methodIndex := -1

	for i, method := range iface.InstanceMethods {
		if method.Name == methodName {
			methodIndex = i
			break
		}
	}

	if methodIndex == -1 {
		panic("codegen.codegen.LookupInterfaceMethod() - interface method not found")
	}

	vtableArrayType := &ir.StructType{Fields: []ir.Field{
		{Name: "type_info", Type: ir.Pointer},
		{Name: "methods", Type: &ir.ArrayType{
			Length:  uint32(len(iface.InstanceMethods)),
			Element: ir.Pointer,
		}},
	}}

	funcPtrPtr := c.emitter.GetElementPtrConst(vtableArrayType, vtablePtr, 0, 1, uint32(methodIndex))
	callee = c.emitter.Load(ir.Pointer, funcPtrPtr)

	return callee, receiver
}

func (c *codegen) BuildCallSignature(typ *types.Func, hasReceiver bool) *ir.Signature {
	sig := &ir.Signature{
		Params:  make([]ir.Type, 0),
		VarArgs: typ.VarArgs,
	}

	params := typ.Params

	// Receiver
	if hasReceiver {
		sig.Params = append(sig.Params, ir.Pointer)

		if len(params) > 0 {
			params = params[1:]
		}
	}

	// Params
	for _, param := range params {
		classes, info := c.callConv.Classify(c.arch, param)

		if len(classes) == 1 && classes[0] == abi.Memory {
			sig.Params = append(sig.Params, ir.Pointer)
		} else {
			sig.Params = append(sig.Params, getTypeForClasses(classes, info.Size))
		}
	}

	// Returns
	classes, info := c.callConv.Classify(c.arch, typ.Returns)

	if len(classes) == 1 && classes[0] == abi.Memory {
		sig.Returns = ir.Void
		sig.SRet = c.types.Get(typ.Returns)
		sig.Params = slices.Insert(sig.Params, 0, ir.Type(ir.Pointer))
	} else {
		sig.Returns = getTypeForClasses(classes, info.Size)
	}

	return sig
}

func (c *codegen) PrepareExprArgs(funcType *types.Func, receiver ir.Value, args []ast.Expr) ([]ir.Value, []types.Type) {
	irArgs := make([]ir.Value, len(args))
	argTypes := make([]types.Type, len(args))

	params := funcType.Params
	if receiver != nil && len(params) > 0 {
		params = params[1:]
	}

	for i, arg := range args {
		if i < len(params) {
			argTypes[i] = params[i]
			irArgs[i] = c.LoadImplicitCast(arg, params[i])
		} else {
			argTypes[i] = c.UnderlyingExprType(arg)
			irArgs[i] = c.Load(arg)
		}
	}

	return irArgs, argTypes
}

func (c *codegen) EmitCallExpr(callee ir.Value, sig *ir.Signature, funcType *types.Func, receiver ir.Value, args []ast.Expr, returnType types.Type) ir.Value {
	irArgs, argTypes := c.PrepareExprArgs(funcType, receiver, args)
	return c.EmitCall(callee, sig, funcType, receiver, irArgs, argTypes, returnType)
}

func (c *codegen) EmitCall(callee ir.Value, sig *ir.Signature, funcType *types.Func, receiver ir.Value, irArgs []ir.Value, argTypes []types.Type, returnType types.Type) ir.Value {
	finalArgs := make([]ir.Value, 0, len(irArgs)+1)

	// Receiver
	if receiver != nil {
		finalArgs = append(finalArgs, receiver)
	}

	// Parameters
	params := funcType.Params
	if receiver != nil && len(params) > 0 {
		params = params[1:]
	}

	for i, argValue := range irArgs {
		var valueType types.Type

		if i < len(params) {
			valueType = params[i]
		} else {
			valueType = argTypes[i]
		}

		classes, info := c.callConv.Classify(c.arch, valueType)

		if len(classes) == 1 && classes[0] == abi.Memory {
			ptr := c.Alloca(argValue.Type(), "call.param")
			c.emitter.Store(argValue, ptr)
			finalArgs = append(finalArgs, ptr)
			continue
		}

		typ := getTypeForClasses(classes, info.Size)
		argValue = c.BitCast(argValue, typ)
		finalArgs = append(finalArgs, argValue)
	}

	// Return value handling
	returnClasses, _ := c.callConv.Classify(c.arch, returnType)

	var returnPtr ir.Value
	if len(returnClasses) == 1 && returnClasses[0] == abi.Memory {
		returnPtr = c.Alloca(c.types.Get(returnType), "call.sret")
		finalArgs = slices.Insert(finalArgs, 0, returnPtr)
	}

	// Call
	if !ir.IsConstant(callee) {
		c.funDoesIndirectDispatch = true
	}

	value := c.emitter.Call(sig, callee, finalArgs)

	// Return
	if core.IsNil(returnPtr) {
		typ := c.types.Get(returnType)
		return c.BitCast(value, typ)
	}

	typ := c.types.Get(returnType)
	return c.emitter.Load(typ, returnPtr)
}

func (c *codegen) ResolveInterfaceMethod(receiverType types.Type, methodName string, isStatic bool) (ir.Value, *ir.Signature, *types.Func) {
	if p, ok := receiverType.(*types.Pointer); ok {
		receiverType = p.Pointee
	}

	// For generic structs, methods live on the template
	lookupTyp := receiverType
	if s, ok := receiverType.(*types.Struct); ok && s.Generic != nil {
		lookupTyp = s.Generic
	}

	// Look up the method
	var sym symbols.Symbol
	var ok bool

	if isStatic {
		sym, ok = c.typeEnv.GetStaticMethod(lookupTyp, methodName)
		if !ok || sym.Kind != symbols.Func {
			panic(fmt.Sprintf("codegen.codegen.ResolveInterfaceMethod() - static method '%s' not found on '%s'", methodName, receiverType))
		}
	} else {
		sym, ok = c.typeEnv.GetInstanceMethod(lookupTyp, methodName)
		if !ok {
			panic(fmt.Sprintf("codegen.codegen.ResolveInterfaceMethod() - method '%s' not found on '%s'", methodName, receiverType))
		}
	}

	concreteFunc := sym.Node.(*ast.Func)
	concreteTyp := sym.Type.(*types.Func)

	// Substitute generics if needed
	if s, ok := receiverType.(*types.Struct); ok && s.Generic != nil {
		concreteTyp = c.instantiations.Substitute(concreteTyp, s.Substitutions).(*types.Func)
	}

	in := c.GetFuncInterface(concreteFunc)

	callee := c.GetFunction(concreteFunc, concreteTyp, in)
	sig := callee.Signature

	c.AddSummaryCallee(concreteFunc, concreteTyp, in, true)

	return callee, sig, concreteTyp
}
