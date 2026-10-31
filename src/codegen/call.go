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

func (c *Codegen) ResolveReceiver(node ast.Expr) ir.Value {
	typ := c.UnderlyingExprType(node)
	switch typ.(type) {
	case *types.Reference, *types.Pointer:
		return c.ValueToReceiver(c.Load(node), typ, false, node)
	default:
		return c.ValueToReceiver(c.GenerateExpr(node), typ, c.Sema.ExprInfos[node].Address, node)
	}
}

func (c *Codegen) ValueToReceiver(value ir.Value, typ types.Type, addressable bool, node ast.Node) ir.Value {
	switch t := typ.(type) {
	case *types.Reference:
		return value
	case *types.Pointer:
		c.CheckNull(value, node, "encountered a null pointer when calling a method on '%s'", t.Pointee)
		return value
	default:
		return c.ReceiverToPointer(value, typ, addressable)
	}
}

func (c *Codegen) ReceiverToPointer(value ir.Value, typ types.Type, addressable bool) ir.Value {
	if addressable {
		return value
	}

	irTyp := c.Types.Get(typ)
	ptr := c.Alloca(irTyp, "call.self")
	c.Emitter.Store(value, ptr)

	return ptr
}

func (c *Codegen) ResolveInterfaceCallee(m *ast.Member) (callee ir.Value, receiver ir.Value) {
	interfaceValue := c.Load(m.Expr)
	interfaceType := c.ExprType(m.Expr).(*types.Interface)
	return c.LookupInterfaceMethod(interfaceType, interfaceValue, m.Name.Token.Text, m.Name)
}

func (c *Codegen) LookupInterfaceMethod(iface *types.Interface, interfaceValue ir.Value, methodName string, node ast.Node) (callee ir.Value, receiver ir.Value) {
	receiver = c.Emitter.ExtractValue(interfaceValue, 0)
	vtablePtr := c.Emitter.ExtractValue(interfaceValue, 1)

	methodIndex := -1

	for i, method := range iface.InstanceMethods {
		if method.Name == methodName {
			methodIndex = i
			break
		}
	}

	if methodIndex == -1 {
		panic("codegen.Codegen.LookupInterfaceMethod() - interface method not found")
	}

	vtableArrayType := &ir.StructType{Fields: []ir.Field{
		{Name: "type_info", Type: ir.Pointer},
		{Name: "methods", Type: &ir.ArrayType{
			Length:  uint32(len(iface.InstanceMethods)),
			Element: ir.Pointer,
		}},
	}}

	funcPtrPtr := c.Emitter.GetElementPtrConst(vtableArrayType, vtablePtr, 0, 1, uint32(methodIndex))
	callee = c.Emitter.Load(ir.Pointer, funcPtrPtr)

	return callee, receiver
}

func (c *Codegen) BuildCallSignature(typ *types.Func, hasReceiver bool) *ir.Signature {
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
		classes, info := c.CallConv.Classify(c.Arch, param)

		if len(classes) == 1 && classes[0] == abi.Memory {
			sig.Params = append(sig.Params, ir.Pointer)
		} else {
			sig.Params = append(sig.Params, getTypeForClasses(classes, info.Size))
		}
	}

	// Returns
	classes, info := c.CallConv.Classify(c.Arch, typ.Returns)

	if len(classes) == 1 && classes[0] == abi.Memory {
		sig.Returns = ir.Void
		sig.SRet = c.Types.Get(typ.Returns)
		sig.Params = slices.Insert(sig.Params, 0, ir.Type(ir.Pointer))
	} else {
		sig.Returns = getTypeForClasses(classes, info.Size)
	}

	return sig
}

func (c *Codegen) PrepareExprArgs(funcType *types.Func, receiver ir.Value, args []ast.Expr) ([]ir.Value, []types.Type) {
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

func (c *Codegen) EmitCallExpr(callee ir.Value, sig *ir.Signature, funcType *types.Func, receiver ir.Value, args []ast.Expr, returnType types.Type) ir.Value {
	irArgs, argTypes := c.PrepareExprArgs(funcType, receiver, args)
	return c.EmitCall(callee, sig, funcType, receiver, irArgs, argTypes, returnType)
}

func (c *Codegen) EmitCall(callee ir.Value, sig *ir.Signature, funcType *types.Func, receiver ir.Value, irArgs []ir.Value, argTypes []types.Type, returnType types.Type) ir.Value {
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

		classes, info := c.CallConv.Classify(c.Arch, valueType)

		if len(classes) == 1 && classes[0] == abi.Memory {
			ptr := c.Alloca(argValue.Type(), "call.param")
			c.Emitter.Store(argValue, ptr)
			finalArgs = append(finalArgs, ptr)
			continue
		}

		typ := getTypeForClasses(classes, info.Size)
		argValue = c.BitCast(argValue, typ)
		finalArgs = append(finalArgs, argValue)
	}

	// Return value handling
	returnClasses, _ := c.CallConv.Classify(c.Arch, returnType)

	var returnPtr ir.Value
	if len(returnClasses) == 1 && returnClasses[0] == abi.Memory {
		returnPtr = c.Alloca(c.Types.Get(returnType), "call.sret")
		finalArgs = slices.Insert(finalArgs, 0, returnPtr)
	}

	// Call
	value := c.Emitter.Call(sig, callee, finalArgs)

	// Return
	if core.IsNil(returnPtr) {
		typ := c.Types.Get(returnType)
		return c.BitCast(value, typ)
	}

	typ := c.Types.Get(returnType)
	return c.Emitter.Load(typ, returnPtr)
}

func (c *Codegen) ResolveInterfaceMethod(receiverType types.Type, methodName string, isStatic bool) (ir.Value, *ir.Signature, *types.Func) {
	if pointee, ok := getPointee(receiverType); ok {
		receiverType = pointee
	}

	var sym symbols.Symbol
	var subs []types.Substitution
	var ok bool

	if isStatic {
		sym, subs, ok = c.Ctx.TypeEnv.GetStaticMethodWithSubs(receiverType, methodName)
	} else {
		sym, subs, ok = c.Ctx.TypeEnv.GetInstanceMethodWithSubs(receiverType, methodName)
	}

	if !ok || sym.Kind != symbols.Func {
		panic(fmt.Sprintf("codegen.Codegen.ResolveInterfaceMethod() - method '%s' not found on '%s'", methodName, receiverType))
	}

	concreteFunc := sym.Node.(*ast.Func)
	concreteTyp := sym.Type.(*types.Func)

	if len(subs) > 0 {
		concreteTyp = c.Ctx.Instantiations.Substitute(concreteTyp, subs).(*types.Func)
	}

	in := c.GetFuncInterface(concreteFunc)

	callee := c.GetFunction(concreteFunc, concreteTyp, in)
	sig := callee.Signature

	return callee, sig, concreteTyp
}
