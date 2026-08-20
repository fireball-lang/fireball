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
			Value: lexer.ParseInteger(n.Token),
		}
	}

	// Float
	if n.Token.Kind == lexer.Decimal32bit {
		value, err := lexer.ParseDecimal(n.Token)
		if err != nil {
			panic("codegen.codegen.VisitNumber() - Failed to parse float '" + n.Token.Text + "'")
		}

		return &ir.FloatV{Value: float32(value)}
	}

	// Double
	if n.Token.Kind == lexer.Decimal {
		value, err := lexer.ParseDecimal(n.Token)
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
		Typ:   c.types.Get(c.ExprType(e)),
		Value: core.Unsigned(false, uint64(e.Rune)),
	}
}

func (c *codegen) VisitString(s *ast.String) ir.Value {
	return c.StringView(s.Runes)
}

func (c *codegen) VisitNull(_ *ast.Null) ir.Value {
	return &ir.Null{}
}

func (c *codegen) VisitStructInitializer(s *ast.StructInitializer) ir.Value {
	typ := c.ExprType(s).(*types.Struct)
	t := c.types.Get(typ).(*ir.RefStructType)
	info := c.arch.Info(typ)

	sb := c.Struct(typ)

	for _, field := range s.Fields {
		name := field.Name.Token.Text
		expr := field.Value

		var fieldTyp types.Type

		for _, f := range info.Fields {
			if typ.Fields[f.Index].Name == field.Name.Token.Text {
				fieldTyp = typ.Fields[f.Index].Type
				break
			}
		}

		value := c.LoadImplicitCast(expr, fieldTyp)

		if typ.Layout == types.Union {
			value = c.BitCast(value, t.Struct.Fields[0].Type)
		}

		sb.Set(name, value)
	}

	return sb.Build()
}

func (c *codegen) VisitArrayInitializer(a *ast.ArrayInitializer) ir.Value {
	typ := c.ExprType(a).(*types.Array)

	ab := c.Array(typ)

	for _, element := range a.Elements {
		value := c.LoadImplicitCast(element, typ.Element)
		ab.Add(value)
	}

	return ab.Build()
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

	var field abi.Field

	for _, f := range info.Fields {
		if typ.Fields[f.Index].Name == o.Field.Token.Text {
			field = f
			break
		}
	}

	return &ir.Integer{
		Typ:   ir.I32,
		Value: core.Unsigned(false, uint64(field.Offset)),
	}
}

func (c *codegen) VisitTypeOf(t *ast.TypeOf) ir.Value {
	typ := c.ResolveType(c.nodeTypes[t.Type])
	return c.GetTypeInfo(typ)
}

func (c *codegen) VisitPrefix(p *ast.Prefix) ir.Value {
	// core::<interface>
	typ_ := c.ExprType(p.Expr)

	if _, ok := typ_.(*types.Primitive); !ok {
		if name := p.Op.InterfaceName(); name != "" {
			if fTyp, fName := sema.GetUnaryMethod(c.typeEnv, c.instantiations, name, typ_); fTyp != nil {
				return c.CallMethodExpr(fTyp, fName, p.Expr, nil)
			}
		}
	}

	// Built-in
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

	case ast.PropagateO:
		typ := c.ExprType(p.Expr)
		t := c.types.Get(typ).(ir.StructLikeType)

		_, hasI := t.Field("has_value")
		if hasI < 0 {
			panic("codegen.codegen.VisitPostfix() - Failed to find 'has_value' field on 'core::Option'")
		}

		bNone := c.fun.NewBlock("propagate.none")
		bSome := c.fun.NewBlock("propagate.some")

		// Condition
		value := c.Load(p.Expr)

		some := c.emitter.ExtractValue(value, uint32(hasI))
		c.emitter.BrCond(some, bSome, bNone)

		// None
		c.emitter.Begin(bNone)
		c.ReturnValue(&ir.ZeroInitializer{Typ: c.types.Get(c.funcTyp.Returns)})

		// Some
		c.emitter.Begin(bSome)
		value = c.emitter.ExtractValue(value, uint32(1-hasI))

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
		left := c.ImplicitCast(leftVal, c.ExprInfo(b.Left), typ, b)
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

	// core::<interface>
	typ := c.ExprType(b.Left)

	if _, ok := typ.(*types.Primitive); !ok {
		if name := b.Op.InterfaceName(); name != "" {
			if fTyp, fName := sema.GetBinaryMethod(c.typeEnv, c.instantiations, name, typ, c.ExprType(b.Right)); fTyp != nil {
				value := c.CallMethodExpr(fTyp, fName, b.Left, []ast.Expr{b.Right})

				switch b.Op {
				case ast.NotEqual:
					return c.emitter.Xor(value, ir.True)

				case ast.Less, ast.Greater:
					name := "Less"
					if b.Op == ast.Greater {
						name = "Greater"
					}

					ordering := fTyp.Returns.(*types.Enum)
					caseValue, ok := ordering.Case(name)
					if !ok {
						panic("codegen.VisitBinary() - Failed to find '" + fTyp.Returns.String() + "::" + name + "' enum case")
					}

					return c.emitter.ICmp(ir.Eq, caseValue.Negative(), value, &ir.Integer{Typ: c.types.Get(fTyp.Returns), Value: caseValue})

				case ast.LessEqual, ast.GreaterEqual:
					name := "Less"
					if b.Op == ast.GreaterEqual {
						name = "Greater"
					}

					ordering := fTyp.Returns.(*types.Enum)

					nameValue, ok := ordering.Case(name)
					if !ok {
						panic("codegen.VisitBinary() - Failed to find '" + fTyp.Returns.String() + "::" + name + "' enum case")
					}

					equalValue, ok := ordering.Case("Equal")
					if !ok {
						panic("codegen.VisitBinary() - Failed to find '" + fTyp.Returns.String() + "::Equal' enum case")
					}

					nameOk := c.emitter.ICmp(ir.Eq, nameValue.Negative(), value, &ir.Integer{Typ: c.types.Get(fTyp.Returns), Value: nameValue})
					equalOk := c.emitter.ICmp(ir.Eq, equalValue.Negative(), value, &ir.Integer{Typ: c.types.Get(fTyp.Returns), Value: equalValue})

					return c.emitter.Or(nameOk, equalOk)

				default:
					return value
				}
			}
		}
	}

	// Built-in
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

	// Or

	case ast.Or:
		typ := c.ExprType(b.Left)
		t := c.types.Get(typ).(ir.StructLikeType)

		_, hasI := t.Field("has_value")
		if hasI < 0 {
			panic("codegen.codegen.VisitBinary() - Failed to find 'has_value' field on 'core::Option'")
		}

		bLeft := c.fun.NewBlock("or.left")
		bRight := c.fun.NewBlock("or.right")
		bExit := c.fun.NewBlock("or.exit")

		// Entry
		left := c.Load(b.Left)
		some := c.emitter.ExtractValue(left, uint32(hasI))
		c.emitter.BrCond(some, bLeft, bRight)

		// Left
		c.emitter.Begin(bLeft)
		leftValue := c.emitter.ExtractValue(left, uint32(1-hasI))
		bLeft = c.emitter.Block()
		c.emitter.Br(bExit)

		// Right
		c.emitter.Begin(bRight)
		right := c.LoadImplicitCast(b.Right, c.ExprType(b))
		bRight = c.emitter.Block()
		c.emitter.Br(bExit)

		// Exit
		c.emitter.Begin(bExit)

		return c.emitter.Phi(
			ir.PhiPair{Block: bLeft, Value: leftValue},
			ir.PhiPair{Block: bRight, Value: right},
		)

	// Base

	default:
		typ := c.ExprType(b)

		left := c.LoadImplicitCast(b.Left, typ)
		right := c.LoadImplicitCast(b.Right, typ)

		return c.VisitCompoundBaseBinaryOp(b, left, right, b.Op)
	}
}

func (c *codegen) VisitCompoundBaseBinaryOp(b *ast.Binary, left, right ir.Value, op ast.BinaryOp) ir.Value {
	// core::<interface>
	typ := c.ExprType(b.Left)

	if _, ok := typ.(*types.Primitive); !ok {
		if name := op.InterfaceName(); name != "" {
			if fTyp, fName := sema.GetBinaryMethod(c.typeEnv, c.instantiations, name, typ, c.ExprType(b.Right)); fTyp != nil {
				return c.CallMethodExpr(fTyp, fName, b.Left, []ast.Expr{b.Right})
			}
		}
	}

	// Built-in
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

	// Invalid

	default:
		panic("codegen.codegen.VisitCompoundBaseBinaryOp() - Invalid compound base operator")
	}
}

func (c *codegen) VisitIdentifier(i *ast.Identifier) ir.Value {
	switch node := c.exprInfos[i].Node.(type) {
	case *ast.GlobalVar:
		typ := c.ExprType(i)
		c.AddSummaryGlobalVar(node)
		return c.GetGlobalVar(node, typ)

	case *ast.Func:
		typ := c.ExprType(i).(*types.Func)
		in := c.GetFuncInterface(node)

		call := false
		if c, ok := i.Parent().(*ast.Call); ok && c.Callee == i {
			call = true
		}
		c.AddSummaryCallee(node, typ, in, call)

		return c.GetFunction(node, typ, in)

	case *ast.Case:
		typ := c.ExprType(i).(*types.Enum)
		value := typ.Cases[slices.Index(node.Parent().(*ast.Enum).Cases, node)].Value

		return &ir.Integer{
			Typ:   c.types.Get(typ),
			Value: value,
		}

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
	typ := c.ExprType(i.Expr)

	// core::Index[T]
	if fTyp, fName := sema.GetBinaryMethod(c.typeEnv, c.instantiations, "core::Index", typ, c.ExprType(i.Index)); fTyp != nil {
		return c.CallMethodExpr(fTyp, fName, i.Expr, []ast.Expr{i.Index})
	}

	// Pointer indexing
	if p, ok := typ.(*types.Pointer); ok {
		irTyp := c.types.Get(p.Pointee)
		ptr := c.Load(i.Expr)
		index := c.Load(i.Index)
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

	index := c.Load(i.Index)
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

	if r, ok := typ.(*types.Reference); ok {
		pointer = true
		s = r.Pointee.(*types.Struct)
	} else if p, ok := typ.(*types.Pointer); ok {
		pointer = true
		s = p.Pointee.(*types.Struct)
	} else {
		s = typ.(*types.Struct)
	}

	index := -1

	if s.Layout == types.Union {
		if f := s.Field(m.Name.Token.Text); f != nil {
			index = 0
		}
	} else {
		t := c.types.Get(s).(ir.StructLikeType)
		_, index = t.Field(m.Name.Token.Text)
	}

	// Method
	if index == -1 {
		f := c.exprInfos[m].Node.(*ast.Func)
		typ := c.ExprType(m).(*types.Func)
		in := c.GetFuncInterface(f)

		call := false
		if c, ok := m.Parent().(*ast.Call); ok && c.Callee == m {
			call = true
		}
		c.AddSummaryCallee(f, typ, in, call)

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
	value = c.emitter.ExtractValue(value, uint32(index))

	if s.Layout == types.Union {
		//goland:noinspection GoMaybeNil
		fieldType := s.Field(m.Name.Token.Text).Type

		value = c.BitCast(value, c.types.Get(fieldType))
	}

	return value
}

func (c *codegen) VisitCall(e *ast.Call) ir.Value {
	funcNode, isDecl := c.exprInfos[e.Callee].Node.(*ast.Func)
	typ := c.ExprType(e.Callee).(*types.Func)

	if instTyp, ok := c.nodeTypes[e].(*types.Func); ok {
		typ = c.ResolveType(instTyp).(*types.Func)
	}

	var callee ir.Value
	var sig *ir.Signature
	var receiver ir.Value

	// Indirect call through a function value
	if !isDecl {
		callee = c.Load(e.Callee)
		sig = c.BuildCallSignature(typ, false)
		return c.EmitCallExpr(callee, sig, typ, nil, e.Args, c.UnderlyingExprType(e))
	}

	f := funcNode
	m, isMember := e.Callee.(*ast.Member)
	hasReceiver := isMember && f.Receiver != nil

	isInterfaceDispatch := false
	if hasReceiver {
		_, isInterfaceDispatch = c.ExprType(m.Expr).(*types.Interface)
	}

	if hasReceiver && isInterfaceDispatch {
		// Interface dispatch
		callee, receiver = c.LookupInterfaceMethod(c.ExprType(m.Expr).(*types.Interface), c.Load(m.Expr), m.Name.Token.Text, m.Name)
		sig = c.BuildCallSignature(typ, true)
	} else if hasReceiver && isInterfaceMethod(f) {
		// Method from interface, resolved to concrete impl
		callee, sig, typ = c.ResolveInterfaceMethod(c.ExprType(m.Expr), m.Name.Token.Text, false)
		receiver = c.ResolveReceiver(m.Expr)
	} else if hasReceiver {
		// Method from impl block
		in := c.GetFuncInterface(f)
		callee = c.GetFunction(f, typ, in)
		sig = callee.(*ir.Function).Signature
		c.AddSummaryCallee(f, typ, in, true)
		receiver = c.ResolveReceiver(m.Expr)
	} else if isInterfaceStatic(f, e.Callee) {
		// Static method from interface, resolved to concrete impl
		ident := e.Callee.(*ast.Identifier)
		typeLeaf := ident.Path[len(ident.Path)-2]
		concreteTyp := c.ResolveType(c.nodeTypes[typeLeaf])
		callee, sig, typ = c.ResolveInterfaceMethod(concreteTyp, f.Name().Token.Text, true)
	} else {
		// Static method from impl block
		in := c.GetFuncInterface(f)
		callee = c.GetFunction(f, typ, in)
		sig = callee.(*ir.Function).Signature
		c.AddSummaryCallee(f, typ, in, true)
	}

	return c.EmitCallExpr(callee, sig, typ, receiver, e.Args, c.UnderlyingExprType(e))
}

func (c *codegen) VisitCast(e *ast.Cast) ir.Value {
	to := c.ExprType(e)

	kind, _ := sema.GetExplicitCast(c.typeEnv, c.ExprInfo(e.Expr), to)

	// sema.ArrayToSlice
	if kind == sema.ArrayToSlice {
		return c.CastArrayToSlice(e.Expr, to)
	}

	// Normal
	value := c.Load(e.Expr)

	return c.Cast(value, kind, c.ExprInfo(e.Expr), to, e)
}

func (c *codegen) VisitBadExpr(_ *ast.BadExpr) ir.Value {
	panic("codegen.codegen.VisitBadExpr() - Shouldn't ever get here")
}

// Utils

func (c *codegen) StringView(runes []rune) ir.Value {
	// Global

	literal := ir.NewString(runes, true)

	global := c.module.NewGlobalVar(fmt.Sprintf("string.%s.%d", c.uid, c.stringCount), literal.Type())
	c.stringCount++

	global.Flags = ir.Private | ir.UnnamedAddr | ir.Constant
	global.Initializer = literal

	// Value

	sb := c.Struct(c.builtins.StringView)

	sb.Set("ptr", global)
	sb.Set("size", &ir.Integer{Typ: ir.I32, Value: core.Unsigned(false, uint64(literal.Size))})

	value := sb.Build()

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

func (c *codegen) CallMethodExpr(fTyp *types.Func, fName string, calleeExpr ast.Expr, args []ast.Expr) ir.Value {
	var callee ir.Value
	var sig *ir.Signature
	var receiver ir.Value

	typ := c.ExprType(calleeExpr)
	_, isInterfaceDispatch := typ.(*types.Interface)

	if isInterfaceDispatch {
		// Interface dispatch
		callee, receiver = c.LookupInterfaceMethod(typ.(*types.Interface), c.Load(calleeExpr), fName, calleeExpr)
		sig = c.BuildCallSignature(fTyp, true)
	} else {
		// Method from interface, resolved to concrete impl
		callee, sig, fTyp = c.ResolveInterfaceMethod(typ, fName, false)
		receiver = c.ResolveReceiver(calleeExpr)
	}

	return c.EmitCallExpr(callee, sig, fTyp, receiver, args, fTyp.Returns)
}

func (c *codegen) CallMethod(fTyp *types.Func, fName string, calleeExpr ast.Expr, irArgs []ir.Value, argTypes []types.Type) ir.Value {
	var callee ir.Value
	var sig *ir.Signature
	var receiver ir.Value

	typ := c.ExprType(calleeExpr)
	_, isInterfaceDispatch := typ.(*types.Interface)

	if isInterfaceDispatch {
		// Interface dispatch
		callee, receiver = c.LookupInterfaceMethod(typ.(*types.Interface), c.Load(calleeExpr), fName, calleeExpr)
		sig = c.BuildCallSignature(fTyp, true)
	} else {
		// Method from interface, resolved to concrete impl
		callee, sig, fTyp = c.ResolveInterfaceMethod(typ, fName, false)
		receiver = c.ResolveReceiver(calleeExpr)
	}

	return c.EmitCall(callee, sig, fTyp, receiver, irArgs, argTypes, fTyp.Returns)
}

func (c *codegen) LoadImplicitCast(expr ast.Expr, typ types.Type) ir.Value {
	from := c.ExprInfo(expr)

	kind, ok := sema.GetImplicitCast(c.typeEnv, from, typ)
	if !ok {
		return c.Load(expr)
	}

	// sema.ArrayToSlice
	if kind == sema.ArrayToSlice {
		return c.CastArrayToSlice(expr, typ)
	}

	// Normal
	value := c.Load(expr)
	return c.Cast(value, kind, from, typ, expr)
}

func (c *codegen) ImplicitCast(value ir.Value, from sema.ExprInfo, to types.Type, errNode ast.Node) ir.Value {
	if kind, ok := sema.GetImplicitCast(c.typeEnv, from, to); ok {
		value = c.Cast(value, kind, from, to, errNode)
	}

	return value
}

func (c *codegen) CastArrayToSlice(expr ast.Expr, to types.Type) ir.Value {
	from := c.ExprType(expr)

	if !c.exprInfos[expr].Address {
		panic("codegen.codegen.CastArrayToSlice() - Expression is not addressable")
	}

	s := to.(*types.Struct)

	sizeField := s.Field("size")
	if sizeField == nil {
		panic("codegen.codegen.CastArrayToSlice() - Failed to find 'size' field on '" + from.String() + "'")
	}

	sizeTyp := c.types.Get(sizeField.Type)

	sb := c.Struct(s)

	sb.Set("ptr", c.GenerateExpr(expr))
	sb.Set("size", &ir.Integer{Typ: sizeTyp, Value: core.Unsigned(false, uint64(from.(*types.Array).Size))})

	return sb.Build()
}

func (c *codegen) Cast(value ir.Value, kind sema.CastKind, from sema.ExprInfo, to types.Type, errNode ast.Node) ir.Value {
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

			switch typ := from.Type.(type) {
			case *types.Integer:
				if typ.Negative {
					floatValue = float64(value.Value.Signed())
				} else {
					floatValue = float64(value.Value.Raw())
				}

			case *types.Primitive:
				if types.IsSigned(typ.Kind) {
					floatValue = float64(value.Value.Signed())
				} else {
					floatValue = float64(value.Value.Raw())
				}
			}

			if toTyp == ir.Float {
				return &ir.FloatV{Value: float32(floatValue)}
			}
			return &ir.DoubleV{Value: floatValue}

		case sema.IntToPointer:
			// fall through

		case sema.TypeToOption:
			sb := c.Struct(to.(*types.Struct))

			sb.Set("has_value", ir.True)
			sb.Set("value", c.ImplicitCast(value, from, to.(*types.Struct).Substitutions[0].Type, errNode))

			return sb.Build()

		default:
			panic("codegen.codegen.Cast() - Invalid cast kind for integer literal")
		}

	case *ir.FloatV:
		switch kind {
		case sema.FloatToInt:
			return &ir.Integer{Typ: toTyp, Value: core.Signed(int64(value.Value))}
		case sema.FloatExtend:
			return &ir.DoubleV{Value: float64(value.Value)}

		case sema.TypeToOption:
			sb := c.Struct(to.(*types.Struct))

			sb.Set("has_value", ir.True)
			sb.Set("value", c.ImplicitCast(value, from, to.(*types.Struct).Substitutions[0].Type, errNode))

			return sb.Build()

		default:
			panic("codegen.codegen.Cast() - Invalid cast kind for float literal")
		}

	case *ir.DoubleV:
		switch kind {
		case sema.FloatToInt:
			return &ir.Integer{Typ: toTyp, Value: core.Signed(int64(value.Value))}
		case sema.FloatTruncate:
			return &ir.FloatV{Value: float32(value.Value)}

		case sema.TypeToOption:
			sb := c.Struct(to.(*types.Struct))

			sb.Set("has_value", ir.True)
			sb.Set("value", c.ImplicitCast(value, from, to.(*types.Struct).Substitutions[0].Type, errNode))

			return sb.Build()

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
		var signed bool

		switch typ := from.Type.(type) {
		case *types.Integer:
			signed = typ.Negative
		case *types.Primitive:
			signed = types.IsSignedInteger(typ.Kind)
		}

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

	case sema.PointerToReference:
		c.CheckNull(value, errNode, "encountered a null pointer when converting '%s' to '%s'", from.Type, to)

	case sema.ReferenceToInterface, sema.PointerToInterface:
		typ := c.types.Get(to)

		if pointee, ok := getPointee(from.Type); ok && pointee == types.PrimitiveVoid {
			value = &ir.ZeroInitializer{Typ: typ}
		} else {
			// Check pointer for null
			null := c.fun.NewBlock("ptr_to_interface.null")
			valid := c.fun.NewBlock("ptr_to_interface.valid")
			exit := c.fun.NewBlock("ptr_to_interface.exit")

			isNull := c.emitter.ICmp(ir.Eq, false, value, &ir.Null{})
			c.emitter.BrCond(isNull, null, valid)

			// Null
			c.emitter.Begin(null)
			nullValue := &ir.ZeroInitializer{Typ: typ}
			c.emitter.Br(exit)

			// Valid
			c.emitter.Begin(valid)

			pointee, _ := getPointee(from.Type)

			validSb := c.Struct(types.InterfaceUnderlying)
			validSb.Set("data", value)
			validSb.Set("vtable", c.GetVTable(to.(*types.Interface), pointee))
			validValue := validSb.Build()

			c.emitter.Br(exit)

			// Exit
			c.emitter.Begin(exit)
			value = c.emitter.Phi(ir.PhiPair{Block: null, Value: nullValue}, ir.PhiPair{Block: valid, Value: validValue})
		}

	case sema.InterfaceToPointer:
		_, dataI := c.types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
		if dataI < 0 {
			panic("codegen.codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
		}

		// Check pointer for null
		start := c.emitter.Block()
		valid := c.fun.NewBlock("interface_to_ptr.valid")
		exit := c.fun.NewBlock("interface_to_ptr.exit")

		dataPtr := c.emitter.ExtractValue(value, uint32(dataI))

		isNull := c.emitter.ICmp(ir.Eq, false, dataPtr, &ir.Null{})
		c.emitter.BrCond(isNull, exit, valid)

		// Valid
		c.emitter.Begin(valid)
		value = c.SafeInterfaceToPointer(value, to.(*types.Pointer).Pointee)
		valid = c.emitter.Block()
		c.emitter.Br(exit)

		// Exit
		c.emitter.Begin(exit)
		value = c.emitter.Phi(ir.PhiPair{Block: valid, Value: value}, ir.PhiPair{Block: start, Value: &ir.Null{}})

	case sema.InterfaceToInterface:
		_, dataI := c.types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
		if dataI < 0 {
			panic("codegen.codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
		}

		// Check pointer for null
		start := c.emitter.Block()
		valid := c.fun.NewBlock("interface_to_pointer.valid")
		exit := c.fun.NewBlock("interface_to_pointer.exit")

		dataPtr := c.emitter.ExtractValue(value, uint32(dataI))

		isNull := c.emitter.ICmp(ir.Eq, false, dataPtr, &ir.Null{})
		c.emitter.BrCond(isNull, exit, valid)

		// Valid
		c.emitter.Begin(valid)
		{
			// Get type info pointers
			vtablePtr := c.emitter.ExtractValue(value, uint32(1-dataI))
			vtableTyp := &ir.StructType{Fields: []ir.Field{{Name: "type_info", Type: ir.Pointer}}}

			srcTypeInfoPtrPtr := c.emitter.GetElementPtrConst(vtableTyp, vtablePtr, 0, 0)
			srcTypeInfoPtr := c.emitter.Load(ir.Pointer, srcTypeInfoPtrPtr)
			targetTypeInfoPtr := c.GetTypeInfo(to)

			// Call 'src.get_vtable(target)'
			symbol, ok := c.typeEnv.GetInstanceMethod(c.builtins.TypeInfo, "get_vtable")
			if !ok {
				panic("codegen.codegen.Cast() - Failed to find 'get_vtable' method on 'core::TypeInfo'")
			}

			f := symbol.Node.(*ast.Func)
			typ := symbol.Type.(*types.Func)

			callee := c.GetFunction(f, typ, nil)
			sig := callee.Signature
			c.AddSummaryCallee(f, typ, nil, true)
			receiver := srcTypeInfoPtr

			vtable := c.EmitCall(callee, sig, typ, receiver, []ir.Value{targetTypeInfoPtr}, []types.Type{&types.Pointer{Pointee: c.builtins.TypeInfo}}, typ.Returns)
			isTarget := c.emitter.ICmp(ir.Ne, false, vtable, &ir.ZeroInitializer{Typ: ir.I64})

			// Extract pointer or null
			ptr := c.ExtractPointerFromInterfaceOrNull(value, isTarget)

			// Reconstruct interface with the new value and vtable
			sb := c.Struct(types.InterfaceUnderlying)

			sb.Set("data", ptr)
			sb.Set("vtable", vtable)

			value = sb.Build()
		}
		valid = c.emitter.Block()
		c.emitter.Br(exit)

		// Exit
		c.emitter.Begin(exit)
		value = c.emitter.Phi(ir.PhiPair{Block: valid, Value: value}, ir.PhiPair{Block: start, Value: &ir.ZeroInitializer{Typ: toTyp}})

	case sema.TypeToOption:
		to := to.(*types.Struct)
		toInner := to.Substitutions[0].Type

		sb := c.Struct(to)

		sb.Set("has_value", ir.True)
		sb.Set("value", c.ImplicitCast(value, from, toInner, errNode))

		return sb.Build()

	case sema.ImplicitAs:
		callee, sig, fTyp := c.ResolveInterfaceMethod(from.Type, "implicit_as", false)
		receiver := c.ReceiverToPointer(value, from.Type, false)

		return c.EmitCallExpr(callee, sig, fTyp, receiver, nil, to)

	case sema.ArrayToSlice:
		panic("codegen.codegen.Cast() - ArrayToSlice should have been handled before calling Cast()")

	default:
		panic("codegen.codegen.Cast() - Invalid cast kind")
	}

	return value
}

func (c *codegen) CheckNull(value ir.Value, node ast.Node, format string, args ...any) {
	null := c.fun.NewBlock("null_check.null")
	valid := c.fun.NewBlock("null_check.valid")

	isNull := c.emitter.ICmp(ir.Eq, false, value, &ir.Null{})
	c.emitter.BrCond(isNull, null, valid)

	// Null
	c.emitter.Begin(null)
	c.EmitPanic(node, format, args...)
	c.emitter.Br(valid) // no-op terminator instruction

	// Valid
	c.emitter.Begin(valid)
}

func (c *codegen) SafeInterfaceToPointer(value ir.Value, pointee types.Type) ir.Value {
	_, vtableI := c.types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("vtable")
	if vtableI < 0 {
		panic("codegen.codegen.VisitBinary() - Failed to find 'vtable' field on 'core::Interface'")
	}

	// Get type info pointers
	vtablePtr := c.emitter.ExtractValue(value, uint32(vtableI))
	vtableTyp := &ir.StructType{Fields: []ir.Field{{Name: "type_info", Type: ir.Pointer}}}

	srcTypeInfoPtrPtr := c.emitter.GetElementPtrConst(vtableTyp, vtablePtr, 0, 0)
	srcTypeInfoPtr := c.emitter.Load(ir.Pointer, srcTypeInfoPtrPtr)
	targetTypeInfoPtr := c.GetTypeInfo(pointee)

	// Check if pointers are the same
	isTarget := c.emitter.ICmp(ir.Eq, false, srcTypeInfoPtr, targetTypeInfoPtr)

	// Extract pointer or null
	return c.ExtractPointerFromInterfaceOrNull(value, isTarget)
}

func (c *codegen) ExtractPointerFromInterfaceOrNull(value, isTarget ir.Value) ir.Value {
	_, dataI := c.types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
	if dataI < 0 {
		panic("codegen.codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
	}

	start := c.emitter.Block()
	ptr := c.fun.NewBlock("interface_check.ptr")
	exit := c.fun.NewBlock("interface_check.exit")

	c.emitter.BrCond(isTarget, ptr, exit)

	// Ptr
	c.emitter.Begin(ptr)
	value = c.emitter.ExtractValue(value, uint32(dataI))
	c.emitter.Br(exit)

	// Exit
	c.emitter.Begin(exit)
	return c.emitter.Phi(ir.PhiPair{Block: ptr, Value: value}, ir.PhiPair{Block: start, Value: &ir.Null{}})
}

func (c *codegen) EmitCmp(op ir.CmpOp, left, right ast.Expr) ir.Value {
	leftType := c.ExprType(left)
	rightType := c.ExprType(right)

	common := sema.CommonType(c.typeEnv, leftType, rightType)
	if common == nil {
		common = leftType
	}

	leftV := c.LoadImplicitCast(left, common)
	rightV := c.LoadImplicitCast(right, common)

	// Interface
	if _, ok := common.(*types.Interface); ok {
		_, dataI := c.types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
		if dataI < 0 {
			panic("codegen.codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
		}

		leftPtr := leftV
		if s, ok := leftV.Type().(*ir.RefStructType); ok && s.Name == types.InterfaceUnderlying.Name {
			leftPtr = c.emitter.ExtractValue(leftV, uint32(dataI))
		}

		rightPtr := rightV
		if s, ok := rightV.Type().(*ir.RefStructType); ok && s.Name == types.InterfaceUnderlying.Name {
			rightPtr = c.emitter.ExtractValue(rightV, uint32(dataI))
		}

		return c.emitter.ICmp(op, false, leftPtr, rightPtr)
	}

	// Enum
	if t, ok := common.(*types.Enum); ok {
		signed := types.IsSignedInteger(t.CaseType.(*types.Primitive).Kind)
		return c.emitter.ICmp(op, signed, leftV, rightV)
	}

	// Pointer
	if _, ok := common.(*types.Pointer); ok {
		return c.emitter.ICmp(op, false, leftV, rightV)
	}

	// Reference
	if _, ok := common.(*types.Reference); ok {
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

func (c *codegen) ExprInfo(expr ast.Expr) sema.ExprInfo {
	info := c.exprInfos[expr]
	info.Type = c.ResolveType(info.Type)

	return info
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
