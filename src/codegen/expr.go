package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/lexer"
	"fireball/sema"
	"fireball/types"
	"fmt"
	"slices"
	"strconv"
)

// Visitor

func (c *codegen) VisitBool(b *ast.Bool) ir.Value {
	if b.Value {
		return ir.True
	}

	return ir.False
}

func (c *codegen) VisitNumber(n *ast.Number) ir.Value {
	// Integer
	if lexer.IsInteger(n.Token.Kind) {
		return &ir.Integer{
			Typ:   c.types.Get(c.UnderlyingExprType(n)),
			Value: core.Unsigned(false, lexer.ParseInteger(n.Token)),
		}
	}

	// Float
	if n.Token.Kind == lexer.Decimal32bit {
		value, err := strconv.ParseFloat(n.Token.Text[:len(n.Token.Text)-1], 32)
		if err != nil {
			panic("codegen.codegen.VisitNumber() - Failed to parse float '" + n.Token.Text + "'")
		}

		return &ir.FloatV{Value: float32(value)}
	}

	// Double
	if n.Token.Kind == lexer.Decimal {
		value, err := strconv.ParseFloat(n.Token.Text, 64)
		if err != nil {
			panic("codegen.codegen.VisitNumber() - Failed to parse double '" + n.Token.Text + "'")
		}

		return &ir.DoubleV{Value: value}
	}

	// Unknown
	panic("codegen.codegen.VisitNumber() - Invalid token kind")
}

func (c *codegen) VisitCharacter(e *ast.Character) ir.Value {
	return &ir.Integer{
		Typ:   ir.I32,
		Value: core.Unsigned(false, uint64(e.Rune)),
	}
}

func (c *codegen) VisitString(s *ast.String) ir.Value {
	// Global

	literal := ir.NewString(s.Runes, true)

	global := c.module.NewGlobalVar(fmt.Sprintf("string.%s.%d", c.uid, c.stringCount), literal.Type())
	c.stringCount++

	global.Flags = ir.Private | ir.UnnamedAddr | ir.Constant
	global.Initializer = literal

	// Value

	typ := c.types.Get(c.UnderlyingExprType(s))

	value := &ir.Struct{
		Typ: typ,
		Fields: []ir.Value{
			global,
			&ir.Integer{Typ: ir.I32, Value: core.Unsigned(false, uint64(literal.Size))},
		},
	}

	// Summary

	if c.moduleSummaryRef.Valid() {
		ref := c.module.AddSummary(&ir.VariableSummary{
			Module: c.moduleSummaryRef,
			Name:   global.Name,
			LinkFlags: ir.LinkSummaryFlags{
				Linkage:             ir.LinkagePrivate,
				Visibility:          ir.VisibilityDefault,
				NotEligibleToImport: true,
				Live:                false,
				DsoLocal:            true,
				CanAutoHide:         true,
				ImportType:          ir.ImportDefinition,
			},
			Flags: ir.VarReadOnly | ir.VarConstant,
			Refs:  nil,
		})

		c.summaryRefs = append(c.summaryRefs, ref)
	}

	return value
}

func (c *codegen) VisitNull(_ *ast.Null) ir.Value {
	return &ir.Null{}
}

func (c *codegen) VisitSizeOf(s *ast.SizeOf) ir.Value {
	typ := c.ResolveType(c.nodeTypes[s.Type])
	info := c.arch.Info(typ)

	return &ir.Integer{
		Typ:   ir.I32,
		Value: core.Unsigned(false, uint64(info.Size)),
	}
}

func (c *codegen) VisitAlignOf(e *ast.AlignOf) ir.Value {
	typ := c.ResolveType(c.nodeTypes[e.Type])
	info := c.arch.Info(typ)

	return &ir.Integer{
		Typ:   ir.I32,
		Value: core.Unsigned(false, uint64(info.Align)),
	}
}

func (c *codegen) VisitOffsetOf(o *ast.OffsetOf) ir.Value {
	typ := c.ResolveType(c.nodeTypes[o.Type]).(*types.Struct)
	info := c.arch.Info(typ)
	_, index := typ.Field(o.Field.Token.Text)

	return &ir.Integer{
		Typ:   ir.I32,
		Value: core.Unsigned(false, uint64(info.Offsets[index])),
	}
}

func (c *codegen) VisitPrefix(p *ast.Prefix) ir.Value {
	switch p.Op {
	case ast.Negate:
		value := c.Load(p.Expr)

		// Floating
		if typ := c.UnderlyingExprType(p.Expr); typ == types.PrimitiveF32 || typ == types.PrimitiveF64 {
			return c.emitter.Fneg(value)
		}

		// Integer
		zero := &ir.Integer{Typ: value.Type(), Value: core.Signed(0)}
		return c.emitter.Sub(zero, value)

	case ast.Not:
		value := c.LoadImplicitCast(p.Expr, types.PrimitiveBool)
		return c.emitter.Xor(value, ir.True)

	case ast.IncrementE:
		ptr := c.GenerateExpr(p.Expr)

		typ := c.types.Get(c.UnderlyingExprType(p.Expr))
		value := c.emitter.Load(typ, ptr)

		value = c.emitter.Add(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(1)})
		c.emitter.Store(value, ptr)

		return value

	case ast.DecrementE:
		ptr := c.GenerateExpr(p.Expr)

		typ := c.types.Get(c.UnderlyingExprType(p.Expr))
		value := c.emitter.Load(typ, ptr)

		value = c.emitter.Sub(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(1)})
		c.emitter.Store(value, ptr)

		return value

	case ast.AddressOf:
		return c.GenerateExpr(p.Expr)

	case ast.Dereference:
		return c.Load(p.Expr)

	default:
		panic("codegen.codegen.VisitPrefix() - Invalid operator")
	}
}

func (c *codegen) VisitPostfix(p *ast.Postfix) ir.Value {
	switch p.Op {
	case ast.IncrementO:
		ptr := c.GenerateExpr(p.Expr)

		typ := c.types.Get(c.UnderlyingExprType(p.Expr))
		value := c.emitter.Load(typ, ptr)

		newValue := c.emitter.Add(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(1)})
		c.emitter.Store(newValue, ptr)

		return value

	case ast.DecrementO:
		ptr := c.GenerateExpr(p.Expr)

		typ := c.types.Get(c.UnderlyingExprType(p.Expr))
		value := c.emitter.Load(typ, ptr)

		newValue := c.emitter.Sub(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(1)})
		c.emitter.Store(newValue, ptr)

		return value

	default:
		panic("codegen.codegen.VisitPostfix() - Invalid operator")
	}
}

func (c *codegen) VisitBinary(b *ast.Binary) ir.Value {
	// Compound assignment
	if b.Op.IsCompoundAssign() {
		ptr := c.GenerateExpr(b.Left)
		typ := c.ExprType(b)

		leftVal := c.emitter.Load(c.types.Get(c.UnderlyingExprType(b.Left)), ptr)
		left := c.ImplicitCast(leftVal, c.ExprType(b.Left), typ)
		right := c.LoadImplicitCast(b.Right, typ)

		op := b.Op.CompoundAssignBase()
		value := c.VisitCompoundBaseBinaryOp(b, left, right, op)
		c.emitter.Store(value, ptr)

		return value
	}

	// Assignment
	if b.Op == ast.Assign {
		ptr := c.GenerateExpr(b.Left)
		value := c.LoadImplicitCast(b.Right, c.ExprType(b.Left))

		c.emitter.Store(value, ptr)
		return value
	}

	switch b.Op {
	// Boolean

	case ast.BoolAnd:
		bRight := c.fun.NewBlock("and.right")
		bExit := c.fun.NewBlock("and.exit")

		// Left
		left := c.LoadImplicitCast(b.Left, types.PrimitiveBool)
		bLeft := c.emitter.Block()
		c.emitter.BrCond(left, bRight, bExit)

		// Right
		c.emitter.Begin(bRight)
		right := c.LoadImplicitCast(b.Right, types.PrimitiveBool)
		bRight = c.emitter.Block()
		c.emitter.Br(bExit)

		// Exit
		c.emitter.Begin(bExit)

		return c.emitter.Phi(
			ir.PhiPair{Block: bLeft, Value: ir.False},
			ir.PhiPair{Block: bRight, Value: right},
		)

	case ast.BoolOr:
		bRight := c.fun.NewBlock("or.right")
		bExit := c.fun.NewBlock("or.exit")

		// Left
		left := c.LoadImplicitCast(b.Left, types.PrimitiveBool)
		bLeft := c.emitter.Block()
		c.emitter.BrCond(left, bExit, bRight)

		// Right
		c.emitter.Begin(bRight)
		right := c.LoadImplicitCast(b.Right, types.PrimitiveBool)
		bRight = c.emitter.Block()
		c.emitter.Br(bExit)

		// Exit
		c.emitter.Begin(bExit)

		return c.emitter.Phi(
			ir.PhiPair{Block: bLeft, Value: ir.True},
			ir.PhiPair{Block: bRight, Value: right},
		)

	// Equality

	case ast.Equal:
		return c.EmitCmp(ir.Eq, b.Left, b.Right)

	case ast.NotEqual:
		return c.EmitCmp(ir.Ne, b.Left, b.Right)

	// Relational

	case ast.Less:
		return c.EmitCmp(ir.Lt, b.Left, b.Right)

	case ast.LessEqual:
		return c.EmitCmp(ir.Le, b.Left, b.Right)

	case ast.Greater:
		return c.EmitCmp(ir.Gt, b.Left, b.Right)

	case ast.GreaterEqual:
		return c.EmitCmp(ir.Ge, b.Left, b.Right)

	default:
		typ := c.ExprType(b)

		left := c.LoadImplicitCast(b.Left, typ)
		right := c.LoadImplicitCast(b.Right, typ)

		return c.VisitCompoundBaseBinaryOp(b, left, right, b.Op)
	}
}

func (c *codegen) VisitCompoundBaseBinaryOp(b *ast.Binary, left, right ir.Value, op ast.BinaryOp) ir.Value {
	switch op {
	// Math

	case ast.Add:
		return c.emitter.Add(left, right)

	case ast.Subtract:
		return c.emitter.Sub(left, right)

	case ast.Multiply:
		return c.emitter.Mul(left, right)

	case ast.Divide:
		kind := c.GetDivKind(b.Left)
		return c.emitter.Div(kind, left, right)

	case ast.Modulo:
		kind := c.GetDivKind(b.Left)
		return c.emitter.Rem(kind, left, right)

	// Bitwise

	case ast.ShiftLeft:
		return c.emitter.Shl(left, right)

	case ast.ShiftRightSignExt:
		return c.emitter.Shr(true, left, right)

	case ast.ShiftRightZeroExt:
		return c.emitter.Shr(false, left, right)

	case ast.BitOr:
		return c.emitter.Or(left, right)

	case ast.BitXor:
		return c.emitter.Xor(left, right)

	case ast.BitAnd:
		return c.emitter.And(left, right)

	default:
		panic("codegen.codegen.VisitCompoundBaseBinaryOp() - Invalid compound base operator")
	}
}

func (c *codegen) VisitIdentifier(i *ast.Identifier) ir.Value {
	switch node := c.exprInfos[i].Node.(type) {
	case *ast.Func:
		typ := c.exprInfos[i].Type.(*types.Func)
		in := c.getFuncInterface(node)
		c.AddSummaryCallee(i, node, typ, in)
		return c.GetFunction(node, typ, in)

	case *ast.Param:
		return c.scope.Get(node.Name.Token.Text)

	case *ast.Var:
		return c.scope.Get(node.Name.Token.Text)

	case *ast.Leaf:
		return c.scope.Get(node.Token.Text)

	case *ast.Receiver:
		return c.scope.Get("self")

	default:
		panic("codegen.codegen.VisitIdentifier() - Invalid node")
	}
}

func (c *codegen) VisitIndex(i *ast.Index) ir.Value {
	typ := c.UnderlyingExprType(i.Expr)
	index := c.Load(i.Index)

	// Pointer indexing
	if p, ok := typ.(*types.Pointer); ok {
		irTyp := c.types.Get(p.Pointee)
		ptr := c.Load(i.Expr)
		return c.emitter.GetElementPtrDyn(irTyp, ptr, index, nil)
	}

	// Array indexing
	irTyp := c.types.Get(typ)

	var ptr ir.Value

	// Get pointer to expression
	if c.exprInfos[i].Address {
		ptr = c.GenerateExpr(i.Expr)
	} else {
		value := c.GenerateExpr(i.Expr)

		ptr = c.Alloca(irTyp, "index")
		c.emitter.Store(value, ptr)
	}

	value := c.emitter.GetElementPtrDyn(irTyp, ptr, ir.False, index)

	if !c.exprInfos[i].Address {
		typ := c.types.Get(typ.(*types.Array).Element)
		value = c.emitter.Load(typ, value)
	}

	return value
}

func (c *codegen) VisitMember(m *ast.Member) ir.Value {
	typ := c.UnderlyingExprType(m.Expr)

	var pointer bool
	var s *types.Struct

	if p, ok := typ.(*types.Pointer); ok {
		pointer = true
		s = p.Pointee.(*types.Struct)
	} else {
		s = typ.(*types.Struct)
	}

	_, index := s.Field(m.Name.Token.Text)

	// Method
	if index == -1 {
		f := c.exprInfos[m].Node.(*ast.Func)
		typ := c.exprInfos[m].Type.(*types.Func)
		in := c.getFuncInterface(f)

		c.AddSummaryCallee(m, f, typ, in)
		return c.GetFunction(f, typ, in)
	}

	// Get struct value
	var value ir.Value

	if pointer {
		value = c.Load(m.Expr)
	} else {
		value = c.GenerateExpr(m.Expr)
	}

	// Pointer
	if c.exprInfos[m].Address {
		typ := c.types.Get(s)
		return c.emitter.GetElementPtrConst(typ, value, 0, uint32(index))
	}

	// Value
	return c.emitter.ExtractValue(value, uint32(index))
}

func (c *codegen) VisitCall(e *ast.Call) ir.Value {
	f := c.exprInfos[e.Callee].Node.(*ast.Func)
	typ := c.ResolveType(c.exprInfos[e.Callee].Type).(*types.Func)

	if instTyp, ok := c.nodeTypes[e].(*types.Func); ok {
		typ = instTyp
	}

	var callee ir.Value
	var sig *ir.Signature
	var receiver ir.Value

	if m, ok := e.Callee.(*ast.Member); ok && f.Receiver != nil {
		if _, ok := c.ExprType(m.Expr).(*types.Interface); ok {
			// Interface dispatch.
			callee, receiver = c.PrepareInterfaceCall(m)

			sig = c.BuildSignature(typ, true)
		} else {
			// Direct method call
			in := c.getFuncInterface(f)

			callee = c.GetFunction(f, typ, in)
			sig = callee.(*ir.Function).Signature

			c.AddSummaryCallee(e.Callee, f, typ, in)

			receiver = c.PrepareReceiver(m)
		}
	} else {
		// Static call
		in := c.getFuncInterface(f)

		callee = c.GetFunction(f, typ, in)
		sig = callee.(*ir.Function).Signature

		c.AddSummaryCallee(e.Callee, f, typ, in)
	}

	return c.EmitCall(callee, sig, typ, receiver, e.Args, c.UnderlyingExprType(e))
}

func (c *codegen) PrepareReceiver(m *ast.Member) ir.Value {
	var value ir.Value

	if _, ok := c.UnderlyingExprType(m.Expr).(*types.Pointer); ok {
		value = c.Load(m.Expr)
	} else {
		value = c.GenerateExpr(m.Expr)

		if !c.exprInfos[m.Expr].Address {
			typ := c.types.Get(c.UnderlyingExprType(m.Expr))
			ptr := c.Alloca(typ, "call.self")
			c.emitter.Store(value, ptr)
			value = ptr
		}
	}

	return value
}

func (c *codegen) PrepareInterfaceCall(m *ast.Member) (callee ir.Value, receiver ir.Value) {
	interfaceValue := c.Load(m.Expr)
	interfaceType := c.ExprType(m.Expr).(*types.Interface)

	receiver = c.emitter.ExtractValue(interfaceValue, 0)
	vtablePtr := c.emitter.ExtractValue(interfaceValue, 1)

	methodName := m.Name.Token.Text
	methodIndex := -1

	for i, method := range interfaceType.InstanceMethods {
		if method.Name == methodName {
			methodIndex = i
			break
		}
	}

	if methodIndex == -1 {
		panic("codegen.codegen.PrepareInterfaceCall() - interface method not found")
	}

	vtableArrayType := &ir.ArrayType{
		Length:  uint32(len(interfaceType.InstanceMethods)),
		Element: ir.Pointer,
	}

	funcPtrPtr := c.emitter.GetElementPtrConst(vtableArrayType, vtablePtr, 0, uint32(methodIndex))
	callee = c.emitter.Load(ir.Pointer, funcPtrPtr)

	return callee, receiver
}

func (c *codegen) BuildSignature(typ *types.Func, hasReceiver bool) *ir.Signature {
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

func (c *codegen) EmitCall(callee ir.Value, sig *ir.Signature, funcType *types.Func, receiver ir.Value, args []ast.Expr, returnType types.Type) ir.Value {
	irArgs := make([]ir.Value, 0, len(args)+1)

	// Receiver
	if receiver != nil {
		irArgs = append(irArgs, receiver)
	}

	// Parameters
	params := funcType.Params

	if receiver != nil && len(params) > 0 {
		params = params[1:]
	}

	for i, arg := range args {
		var valueType types.Type
		var value ir.Value

		if i < len(params) {
			valueType = params[i]
			value = c.LoadImplicitCast(arg, valueType)
		} else {
			valueType = c.UnderlyingExprType(arg)
			value = c.Load(arg)
		}

		classes, info := c.callConv.Classify(c.arch, valueType)

		if len(classes) == 1 && classes[0] == abi.Memory {
			ptr := c.Alloca(value.Type(), "call.param")
			c.emitter.Store(value, ptr)
			irArgs = append(irArgs, ptr)
			continue
		}

		typ := getTypeForClasses(classes, info.Size)
		value = c.BitCast(value, typ)
		irArgs = append(irArgs, value)
	}

	// Return value handling
	returnClasses, _ := c.callConv.Classify(c.arch, returnType)

	var returnPtr ir.Value
	if len(returnClasses) == 1 && returnClasses[0] == abi.Memory {
		returnPtr = c.Alloca(c.types.Get(returnType), "call.sret")
		irArgs = slices.Insert(irArgs, 0, returnPtr)
	}

	// Call
	value := c.emitter.Call(sig, callee, irArgs)

	// Return
	if core.IsNil(returnPtr) {
		typ := c.types.Get(returnType)
		return c.BitCast(value, typ)
	}

	typ := c.types.Get(returnType)
	return c.emitter.Load(typ, returnPtr)
}

func (c *codegen) VisitCast(e *ast.Cast) ir.Value {
	value := c.Load(e.Expr)

	from := c.ExprType(e.Expr)
	to := c.ExprType(e)

	kind, _ := sema.GetExplicitCast(c.typeEnv, from, to)

	return c.Cast(value, kind, from, to)
}

func (c *codegen) VisitBadExpr(_ *ast.BadExpr) ir.Value {
	panic("codegen.codegen.VisitBadExpr() - Shouldn't ever get here")
}

// Utils

func (c *codegen) LoadImplicitCast(expr ast.Expr, typ types.Type) ir.Value {
	value := c.Load(expr)
	return c.ImplicitCast(value, c.ExprType(expr), typ)
}

func (c *codegen) ImplicitCast(value ir.Value, from, to types.Type) ir.Value {
	if kind, ok := sema.GetImplicitCast(c.typeEnv, from, to); ok {
		value = c.Cast(value, kind, from, to)
	}

	return value
}

func (c *codegen) Cast(value ir.Value, kind sema.CastKind, from, to types.Type) ir.Value {
	if kind == sema.Noop {
		return value
	}

	toTyp := c.types.Get(to)

	// Compile time conversion
	switch value := value.(type) {
	case *ir.Integer:
		switch kind {
		case sema.ZeroExtend, sema.SignExtend, sema.Truncate:
			return &ir.Integer{Typ: toTyp, Value: value.Value}

		case sema.IntToFloat:
			var floatValue float64
			if types.IsSigned(from.(*types.Primitive).Kind) {
				floatValue = float64(value.Value.Signed())
			} else {
				floatValue = float64(value.Value.Raw())
			}

			if toTyp == ir.Float {
				return &ir.FloatV{Value: float32(floatValue)}
			}
			return &ir.DoubleV{Value: floatValue}

		case sema.IntToPointer:
			// fall through

		default:
			panic("codegen.codegen.Cast() - Invalid cast kind for integer literal")
		}

	case *ir.FloatV:
		switch kind {
		case sema.FloatToInt:
			return &ir.Integer{Typ: toTyp, Value: core.Signed(int64(value.Value))}
		case sema.FloatExtend:
			return &ir.DoubleV{Value: float64(value.Value)}
		default:
			panic("codegen.codegen.Cast() - Invalid cast kind for float literal")
		}

	case *ir.DoubleV:
		switch kind {
		case sema.FloatToInt:
			return &ir.Integer{Typ: toTyp, Value: core.Signed(int64(value.Value))}
		case sema.FloatTruncate:
			return &ir.FloatV{Value: float32(value.Value)}
		default:
			panic("codegen.codegen.Cast() - Invalid cast kind for double literal")
		}
	}

	// Runtime conversion
	switch kind {
	case sema.ZeroExtend:
		value = c.emitter.Ext(ir.Unsigned, value, toTyp)

	case sema.SignExtend:
		value = c.emitter.Ext(ir.Signed, value, toTyp)

	case sema.Truncate:
		value = c.emitter.Trunc(value, toTyp)

	case sema.IntToFloat:
		signed := types.IsSigned(from.(*types.Primitive).Kind)
		value = c.emitter.IntToFp(signed, value, toTyp)

	case sema.FloatToInt:
		signed := types.IsSigned(to.(*types.Primitive).Kind)
		value = c.emitter.FpToInt(signed, value, toTyp)

	case sema.FloatExtend:
		value = c.emitter.Ext(ir.Floating, value, toTyp)

	case sema.FloatTruncate:
		value = c.emitter.Trunc(value, toTyp)

	case sema.IntToPointer:
		value = c.emitter.IntToPtr(value, toTyp)

	case sema.PointerToInt:
		value = c.emitter.PtrToInt(value, toTyp)

	case sema.PointerToInterface:
		typ := c.types.Get(to)
		vtable := c.GetVTable(to.(*types.Interface), from.(*types.Pointer).Pointee)

		value = c.emitter.InsertValue(&ir.Struct{
			Typ: typ,
			Fields: []ir.Value{
				&ir.Null{},
				vtable,
			},
		}, value, 0)

	case sema.InterfaceToPointer:
		value = c.emitter.ExtractValue(value, 0)

	default:
		panic("codegen.codegen.Cast() - Invalid cast kind")
	}

	return value
}

func (c *codegen) EmitCmp(op ir.CmpOp, left, right ast.Expr) ir.Value {
	leftType := c.ExprType(left)
	rightType := c.ExprType(right)

	common := sema.CommonType(leftType, rightType)
	if common == nil {
		common = leftType
	}

	leftV := c.LoadImplicitCast(left, common)
	rightV := c.LoadImplicitCast(right, common)

	if c, ok := common.(types.Composed); ok {
		common = c
	}

	// Pointer
	if _, ok := common.(*types.Pointer); ok {
		return c.emitter.ICmp(op, false, leftV, rightV)
	}

	// Primitive
	prim := common.(*types.Primitive).Kind

	if types.IsFloating(prim) {
		return c.emitter.FCmp(op, false, leftV, rightV)
	}

	signed := types.IsSignedInteger(prim)
	return c.emitter.ICmp(op, signed, leftV, rightV)
}

func (c *codegen) GetDivKind(expr ast.Expr) ir.DivKind {
	prim := c.UnderlyingExprType(expr).(*types.Primitive).Kind
	kind := ir.Floating

	if types.IsSignedInteger(prim) {
		kind = ir.Signed
	} else if types.IsUnsignedInteger(prim) {
		kind = ir.Unsigned
	}

	return kind
}

func (c *codegen) UnderlyingExprType(expr ast.Expr) types.Type {
	typ := c.ExprType(expr)

	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	return typ
}

func (c *codegen) ExprType(expr ast.Expr) types.Type {
	return c.ResolveType(c.exprInfos[expr].Type)
}

func (c *codegen) Load(expr ast.Expr) ir.Value {
	value := c.GenerateExpr(expr)

	if info := c.exprInfos[expr]; info.Address {
		typ := c.ResolveType(info.Type)
		if t, ok := typ.(types.Composed); ok {
			typ = t.Underlying()
		}

		value = c.emitter.Load(c.types.Get(typ), value)
	}

	return value
}

func (c *codegen) ResolveType(typ types.Type) types.Type {
	if len(c.substitutions) == 0 {
		return typ
	}

	return c.instantiations.Substitute(typ, c.substitutions)
}

func (c *codegen) GenerateExpr(expr ast.Expr) ir.Value {
	c.emitter.SetDebugLocation(expr.Range().Start)
	return ast.VisitExpr(c, expr)
}
