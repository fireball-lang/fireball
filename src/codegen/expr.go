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

func (c *Codegen) VisitBool(b *ast.Bool) ir.Value {
	if b.Value {
		return ir.True
	}

	return ir.False
}

func (c *Codegen) VisitNumber(n *ast.Number) ir.Value {
	// Integer
	if lexer.IsInteger(n.Token.Kind) {
		return &ir.Integer{
			Typ:   c.Types.Get(c.UnderlyingExprType(n)),
			Value: lexer.ParseInteger(n.Token),
		}
	}

	// Float
	if n.Token.Kind == lexer.Decimal32bit {
		value, _ := lexer.ParseDecimal(n.Token)
		return &ir.FloatV{Value: float32(value)}
	}

	// Double
	if n.Token.Kind == lexer.Decimal {
		value, _ := lexer.ParseDecimal(n.Token)
		return &ir.DoubleV{Value: value}
	}

	// Unknown
	panic("codegen.Codegen.VisitNumber() - Invalid token kind")
}

func (c *Codegen) VisitCharacter(e *ast.Character) ir.Value {
	return &ir.Integer{
		Typ:   c.Types.Get(c.ExprType(e)),
		Value: core.Unsigned(false, uint64(e.Rune)),
	}
}

func (c *Codegen) VisitString(s *ast.String) ir.Value {
	return c.StringView(s.Runes)
}

func (c *Codegen) VisitNull(_ *ast.Null) ir.Value {
	return &ir.Null{}
}

func (c *Codegen) VisitStructInitializer(s *ast.StructInitializer) ir.Value {
	typ := c.ExprType(s).(*types.Struct)
	t := c.Types.Get(typ).(*ir.RefStructType)
	info := c.Arch.Info(typ)

	sb := c.Struct(typ)

	for _, field := range s.Fields {
		value, _ := c.VisitFieldInitializer(typ, t, info, field)
		sb.Set(field.Name.Token.Text, value)
	}

	return sb.Build()
}

func (c *Codegen) VisitWith(w *ast.With) ir.Value {
	typ := c.ExprType(w).(*types.Struct)
	t := c.Types.Get(typ).(*ir.RefStructType)
	info := c.Arch.Info(typ)

	structValue := c.Load(w.Expr)

	for _, field := range w.Fields {
		value, fieldI := c.VisitFieldInitializer(typ, t, info, field)
		structValue = c.Emitter.InsertValue(structValue, value, fieldI)
	}

	return structValue
}

func (c *Codegen) VisitFieldInitializer(typ *types.Struct, t *ir.RefStructType, info abi.Info, field *ast.FieldInitializer) (ir.Value, uint32) {
	name := field.Name.Token.Text
	expr := field.Value

	var fieldTyp types.Type
	var fieldI uint32

	for i, f := range info.Fields {
		if typ.Fields[f.Index].Name == name {
			fieldI = uint32(i)
			fieldTyp = typ.Fields[f.Index].Type
			break
		}
	}

	value := c.LoadImplicitCast(expr, fieldTyp)

	if typ.Layout == types.Union {
		value = c.BitCast(value, t.Struct.Fields[0].Type)
		fieldI = 0
	}

	return value, fieldI
}

func (c *Codegen) VisitArrayInitializer(a *ast.ArrayInitializer) ir.Value {
	typ := c.ExprType(a).(*types.Array)

	ab := c.Array(typ)

	for _, element := range a.Elements {
		value := c.LoadImplicitCast(element, typ.Element)
		ab.Add(value)
	}

	return ab.Build()
}

func (c *Codegen) VisitSizeOf(s *ast.SizeOf) ir.Value {
	typ := c.ResolveType(c.NodeTypes[s.Type])
	info := c.Arch.Info(typ)

	return &ir.Integer{
		Typ:   ir.I32,
		Value: core.Unsigned(false, uint64(info.Size)),
	}
}

func (c *Codegen) VisitAlignOf(e *ast.AlignOf) ir.Value {
	typ := c.ResolveType(c.NodeTypes[e.Type])
	info := c.Arch.Info(typ)

	return &ir.Integer{
		Typ:   ir.I32,
		Value: core.Unsigned(false, uint64(info.Align)),
	}
}

func (c *Codegen) VisitOffsetOf(o *ast.OffsetOf) ir.Value {
	typ := c.ResolveType(c.NodeTypes[o.Type]).(*types.Struct)
	info := c.Arch.Info(typ)

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

func (c *Codegen) VisitTypeOf(t *ast.TypeOf) ir.Value {
	typ := c.ResolveType(c.NodeTypes[t.Type])
	return c.GetTypeInfo(typ)
}

func (c *Codegen) VisitPrefix(p *ast.Prefix) ir.Value {
	// core::<interface>
	typ_ := c.ExprType(p.Expr)

	if _, ok := typ_.(*types.Primitive); !ok {
		if name := p.Op.InterfaceName(); name != "" {
			if fTyp, fName := sema.GetUnaryMethod(c.TypeEnv, c.instantiations, name, typ_); fTyp != nil {
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
			return c.Emitter.Fneg(value)
		}

		// Integer
		zero := &ir.Integer{Typ: value.Type(), Value: core.Signed(0)}
		return c.Emitter.Sub(zero, value)

	case ast.Not:
		value := c.LoadImplicitCast(p.Expr, types.PrimitiveBool)
		return c.Emitter.Xor(value, ir.True)

	case ast.BitNot:
		value := c.Load(p.Expr)
		return c.Emitter.Xor(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(-1)})

	case ast.IncrementE:
		ptr := c.GenerateExpr(p.Expr)

		typ := c.Types.Get(c.UnderlyingExprType(p.Expr))
		value := c.Emitter.Load(typ, ptr)

		value = c.Emitter.Add(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(1)})
		c.Emitter.Store(value, ptr)

		return value

	case ast.DecrementE:
		ptr := c.GenerateExpr(p.Expr)

		typ := c.Types.Get(c.UnderlyingExprType(p.Expr))
		value := c.Emitter.Load(typ, ptr)

		value = c.Emitter.Sub(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(1)})
		c.Emitter.Store(value, ptr)

		return value

	case ast.AddressOf:
		return c.GenerateExpr(p.Expr)

	case ast.Dereference:
		return c.Load(p.Expr)

	default:
		panic("codegen.Codegen.VisitPrefix() - Invalid operator")
	}
}

func (c *Codegen) VisitPostfix(p *ast.Postfix) ir.Value {
	switch p.Op {
	case ast.IncrementO:
		ptr := c.GenerateExpr(p.Expr)

		typ := c.Types.Get(c.UnderlyingExprType(p.Expr))
		value := c.Emitter.Load(typ, ptr)

		newValue := c.Emitter.Add(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(1)})
		c.Emitter.Store(newValue, ptr)

		return value

	case ast.DecrementO:
		ptr := c.GenerateExpr(p.Expr)

		typ := c.Types.Get(c.UnderlyingExprType(p.Expr))
		value := c.Emitter.Load(typ, ptr)

		newValue := c.Emitter.Sub(value, &ir.Integer{Typ: value.Type(), Value: core.Signed(1)})
		c.Emitter.Store(newValue, ptr)

		return value

	case ast.PropagateO:
		typ := c.ExprType(p.Expr)
		t := c.Types.Get(typ).(ir.StructLikeType)

		_, hasI := t.Field("has_value")
		if hasI < 0 {
			panic("codegen.Codegen.VisitPostfix() - Failed to find 'has_value' field on 'core::Option'")
		}

		bNone := c.fun.NewBlock("propagate.none")
		bSome := c.fun.NewBlock("propagate.some")

		// Condition
		value := c.Load(p.Expr)

		some := c.Emitter.ExtractValue(value, uint32(hasI))
		c.Emitter.BrCond(some, bSome, bNone)

		// None
		c.Emitter.Begin(bNone)
		c.ReturnValue(&ir.ZeroInitializer{Typ: c.Types.Get(c.funcTyp.Returns)})

		// Some
		c.Emitter.Begin(bSome)
		value = c.Emitter.ExtractValue(value, uint32(1-hasI))

		return value

	default:
		panic(fmt.Sprintf("codegen.Codegen.VisitPostfix() - Invalid operator '%T'", p.Op))
	}
}

func (c *Codegen) VisitBinary(b *ast.Binary) ir.Value {
	// Compound assignment
	if b.Op.IsCompoundAssign() {
		ptr := c.GenerateExpr(b.Left)
		typ := c.ExprType(b)

		leftVal := c.Emitter.Load(c.Types.Get(c.UnderlyingExprType(b.Left)), ptr)
		left := c.ImplicitCast(leftVal, c.ExprInfo(b.Left), typ, b)
		right := c.LoadImplicitCast(b.Right, typ)

		op := b.Op.CompoundAssignBase()
		value := c.VisitCompoundBaseBinaryOp(b, left, right, op)
		c.Emitter.Store(value, ptr)

		return value
	}

	// Assignment
	if b.Op == ast.Assign {
		ptr := c.GenerateExpr(b.Left)
		value := c.LoadImplicitCast(b.Right, c.ExprType(b.Left))

		c.Emitter.Store(value, ptr)
		return value
	}

	// core::<interface>
	typ := c.ExprType(b.Left)

	if _, ok := typ.(*types.Primitive); !ok {
		if name := b.Op.InterfaceName(); name != "" {
			if fTyp, fName := sema.GetBinaryMethod(c.TypeEnv, c.instantiations, name, typ, c.ExprType(b.Right)); fTyp != nil {
				value := c.CallMethodExpr(fTyp, fName, b.Left, []ast.Expr{b.Right})

				switch b.Op {
				case ast.NotEqual:
					return c.Emitter.Xor(value, ir.True)

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

					return c.Emitter.ICmp(ir.Eq, caseValue.Negative(), value, &ir.Integer{Typ: c.Types.Get(fTyp.Returns), Value: caseValue})

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

					nameOk := c.Emitter.ICmp(ir.Eq, nameValue.Negative(), value, &ir.Integer{Typ: c.Types.Get(fTyp.Returns), Value: nameValue})
					equalOk := c.Emitter.ICmp(ir.Eq, equalValue.Negative(), value, &ir.Integer{Typ: c.Types.Get(fTyp.Returns), Value: equalValue})

					return c.Emitter.Or(nameOk, equalOk)

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
		bLeft := c.Emitter.Block()
		c.Emitter.BrCond(left, bRight, bExit)

		// Right
		c.Emitter.Begin(bRight)
		right := c.LoadImplicitCast(b.Right, types.PrimitiveBool)
		bRight = c.Emitter.Block()
		c.Emitter.Br(bExit)

		// Exit
		c.Emitter.Begin(bExit)

		return c.Emitter.Phi(
			ir.PhiPair{Block: bLeft, Value: ir.False},
			ir.PhiPair{Block: bRight, Value: right},
		)

	case ast.BoolOr:
		bRight := c.fun.NewBlock("or.right")
		bExit := c.fun.NewBlock("or.exit")

		// Left
		left := c.LoadImplicitCast(b.Left, types.PrimitiveBool)
		bLeft := c.Emitter.Block()
		c.Emitter.BrCond(left, bExit, bRight)

		// Right
		c.Emitter.Begin(bRight)
		right := c.LoadImplicitCast(b.Right, types.PrimitiveBool)
		bRight = c.Emitter.Block()
		c.Emitter.Br(bExit)

		// Exit
		c.Emitter.Begin(bExit)

		return c.Emitter.Phi(
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
		t := c.Types.Get(typ).(ir.StructLikeType)

		_, hasI := t.Field("has_value")
		if hasI < 0 {
			panic("codegen.Codegen.VisitBinary() - Failed to find 'has_value' field on 'core::Option'")
		}

		bLeft := c.fun.NewBlock("or.left")
		bRight := c.fun.NewBlock("or.right")
		bExit := c.fun.NewBlock("or.exit")

		// Entry
		left := c.Load(b.Left)
		some := c.Emitter.ExtractValue(left, uint32(hasI))
		c.Emitter.BrCond(some, bLeft, bRight)

		// Left
		c.Emitter.Begin(bLeft)
		leftValue := c.Emitter.ExtractValue(left, uint32(1-hasI))
		bLeft = c.Emitter.Block()
		c.Emitter.Br(bExit)

		// Right
		c.Emitter.Begin(bRight)
		right := c.LoadImplicitCast(b.Right, c.ExprType(b))
		bRight = c.Emitter.Block()
		c.Emitter.Br(bExit)

		// Exit
		c.Emitter.Begin(bExit)

		return c.Emitter.Phi(
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

func (c *Codegen) VisitCompoundBaseBinaryOp(b *ast.Binary, left, right ir.Value, op ast.BinaryOp) ir.Value {
	// core::<interface>
	typ := c.ExprType(b.Left)

	if _, ok := typ.(*types.Primitive); !ok {
		if name := op.InterfaceName(); name != "" {
			if fTyp, fName := sema.GetBinaryMethod(c.TypeEnv, c.instantiations, name, typ, c.ExprType(b.Right)); fTyp != nil {
				return c.CallMethodExpr(fTyp, fName, b.Left, []ast.Expr{b.Right})
			}
		}
	}

	// Built-in
	switch op {
	// Math

	case ast.Add:
		return c.Emitter.Add(left, right)

	case ast.Subtract:
		return c.Emitter.Sub(left, right)

	case ast.Multiply:
		return c.Emitter.Mul(left, right)

	case ast.Divide:
		kind := c.GetDivKind(b.Left)
		return c.Emitter.Div(kind, left, right)

	case ast.Modulo:
		kind := c.GetDivKind(b.Left)
		return c.Emitter.Rem(kind, left, right)

	// Bitwise

	case ast.ShiftLeft:
		return c.Emitter.Shl(left, right)

	case ast.ShiftRightSignExt:
		return c.Emitter.Shr(true, left, right)

	case ast.ShiftRightZeroExt:
		return c.Emitter.Shr(false, left, right)

	case ast.BitOr:
		return c.Emitter.Or(left, right)

	case ast.BitXor:
		return c.Emitter.Xor(left, right)

	case ast.BitAnd:
		return c.Emitter.And(left, right)

	// Invalid

	default:
		panic("codegen.Codegen.VisitCompoundBaseBinaryOp() - Invalid compound base operator")
	}
}

func (c *Codegen) VisitIdentifier(i *ast.Identifier) ir.Value {
	switch node := c.ExprInfos[i].Node.(type) {
	case *ast.AssociatedConst:
		implConst, subs := c.resolveAssociatedConst(node, i)

		if c.CompTime {
			return c.materializeConstValue(ast.GetFile(implConst), implConst.Type, implConst.Value, subs)
		}

		return c.GetAssociatedConst(implConst, subs)

	case *ast.Const:
		if c.CompTime {
			return c.materializeConstValue(ast.GetFile(node), node.Type, node.Value, nil)
		}

		return c.GetConst(node)

	case *ast.GlobalVar:
		typ := c.ExprType(i)
		return c.GetGlobalVar(node, typ)

	case *ast.Func:
		typ := c.ExprType(i).(*types.Func)
		in := c.GetFuncInterface(node)

		return c.GetFunction(node, typ, in)

	case *ast.Case:
		typ := c.ExprType(i).(*types.Enum)
		value := typ.Cases[slices.Index(node.Parent().(*ast.Enum).Cases, node)].Value

		return &ir.Integer{
			Typ:   c.Types.Get(typ),
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
		panic(fmt.Sprintf("codegen.Codegen.VisitIdentifier() - Invalid node '%T' at %s:%d:%d (info: %+v)", c.ExprInfos[i].Node, c.Module.Path, i.Range().Start.Line, i.Range().Start.Column, c.ExprInfos[i]))
	}
}

func (c *Codegen) VisitIndex(i *ast.Index) ir.Value {
	typ := c.ExprType(i.Expr)

	// core::Index[T]
	rawType := c.ExprInfos[i.Expr].Type
	rawIndexType := c.ExprInfos[i.Index].Type

	if fTyp, fName := sema.GetBinaryMethod(c.TypeEnv, c.instantiations, "core::Index", rawType, rawIndexType); fTyp != nil {
		return c.CallMethodExpr(fTyp, fName, i.Expr, []ast.Expr{i.Index})
	}

	// Pointer indexing
	if p, ok := typ.(*types.Pointer); ok {
		irTyp := c.Types.Get(p.Pointee)
		ptr := c.Load(i.Expr)
		index := c.Load(i.Index)
		return c.Emitter.GetElementPtrDyn(irTyp, ptr, index)
	}

	// Array indexing
	irTyp := c.Types.Get(typ)

	var ptr ir.Value

	// Get pointer to expression
	if c.ExprInfos[i].Address {
		ptr = c.GenerateExpr(i.Expr)
	} else {
		value := c.GenerateExpr(i.Expr)

		ptr = c.Alloca(irTyp, "index")
		c.Emitter.Store(value, ptr)
	}

	index := c.Load(i.Index)
	value := c.Emitter.GetElementPtrDyn(irTyp, ptr, ir.False, index)

	if !c.ExprInfos[i].Address {
		typ := c.Types.Get(typ.(*types.Array).Element)
		value = c.Emitter.Load(typ, value)
	}

	return value
}

func (c *Codegen) VisitMember(m *ast.Member) ir.Value {
	typ := c.UnderlyingExprType(m.Expr)

	var dereference bool
	var pointer bool
	var s *types.Struct

	if r, ok := typ.(*types.Reference); ok {
		dereference = true
		s = r.Pointee.(*types.Struct)
	} else if p, ok := typ.(*types.Pointer); ok {
		dereference = true
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
		t := c.Types.Get(s).(ir.StructLikeType)
		_, index = t.Field(m.Name.Token.Text)
	}

	// Method
	if index == -1 {
		f := c.ExprInfos[m].Node.(*ast.Func)
		typ := c.ExprType(m).(*types.Func)
		in := c.GetFuncInterface(f)

		return c.GetFunction(f, typ, in)
	}

	// Get struct value
	var value ir.Value

	if dereference {
		value = c.Load(m.Expr)
	} else {
		value = c.GenerateExpr(m.Expr)
	}

	// Pointer
	if c.ExprInfos[m].Address {
		if pointer {
			c.CheckNull(value, m, "encountered a null pointer when accessing field '%s' on '%s'", m.Name.Token.Text, s)
		}

		typ := c.Types.Get(s)
		return c.Emitter.GetElementPtrConst(typ, value, 0, uint32(index))
	}

	// Value
	value = c.Emitter.ExtractValue(value, uint32(index))

	if s.Layout == types.Union {
		//goland:noinspection GoMaybeNil
		fieldType := s.Field(m.Name.Token.Text).Type

		value = c.BitCast(value, c.Types.Get(fieldType))
	}

	return value
}

func (c *Codegen) VisitCall(e *ast.Call) ir.Value {
	funcNode, isDecl := c.ExprInfos[e.Callee].Node.(*ast.Func)
	typ := c.ExprType(e.Callee).(*types.Func)

	if instTyp, ok := c.NodeTypes[e].(*types.Func); ok {
		typ = c.ResolveType(instTyp).(*types.Func)
	}

	// Intrinsic
	if isDecl {
		if intrinsic := ast.GetAttribute[*ast.Intrinsic](funcNode); intrinsic != nil {
			return c.EmitIntrinsic(intrinsic.Kind, typ, e.Args)
		}
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
		receiver = c.ResolveReceiver(m.Expr)
	} else if isInterfaceStatic(f, e.Callee) {
		// Static method from interface, resolved to concrete impl
		ident := e.Callee.(*ast.Identifier)
		typeLeaf := ident.Path[len(ident.Path)-2]
		concreteTyp := c.ResolveType(c.NodeTypes[typeLeaf])
		callee, sig, typ = c.ResolveInterfaceMethod(concreteTyp, f.Name().Token.Text, true)
	} else {
		// Static method from impl block
		in := c.GetFuncInterface(f)
		callee = c.GetFunction(f, typ, in)
		sig = callee.(*ir.Function).Signature
	}

	return c.EmitCallExpr(callee, sig, typ, receiver, e.Args, c.UnderlyingExprType(e))
}

func (c *Codegen) VisitCast(e *ast.Cast) ir.Value {
	to := c.ResolveType(c.NodeTypes[e.Type])
	final := c.ExprType(e)

	kind, _ := sema.GetExplicitCast(c.TypeEnv, c.ExprInfo(e.Expr), to)

	// sema.ArrayToSlice
	if kind == sema.ArrayToSlice {
		return c.CastArrayToSlice(e.Expr, to)
	}

	// Normal
	value := c.Load(e.Expr)

	return c.Cast(value, kind, c.ExprInfo(e.Expr), to, final, e)
}

func (c *Codegen) VisitBadExpr(_ *ast.BadExpr) ir.Value {
	panic("codegen.Codegen.VisitBadExpr() - Shouldn't ever get here")
}

// Utils

func (c *Codegen) StringView(runes []rune) ir.Value {
	// Global

	init := ir.NewString(runes, true)
	gVar := c.StringPtr(init)

	// Value

	sb := c.Struct(c.Builtins.StringView)

	sb.Set("ptr", gVar)
	sb.Set("size", &ir.Integer{Typ: ir.I32, Value: core.Unsigned(false, uint64(init.Size))})

	value := sb.Build()

	return value
}

func (c *Codegen) StringPtr(init *ir.String) *ir.GlobalVar {
	gVar := c.GlobalVar(fmt.Sprintf("string.%s.%d", c.Uid, c.stringCount), ir.Private|ir.UnnamedAddr|ir.Constant, init)
	gVar.Initializer = init

	c.stringCount++

	return gVar
}

func (c *Codegen) CallMethodExpr(fTyp *types.Func, fName string, calleeExpr ast.Expr, args []ast.Expr) ir.Value {
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

func (c *Codegen) CallMethod(fTyp *types.Func, fName string, calleeExpr ast.Expr, irArgs []ir.Value, argTypes []types.Type) ir.Value {
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

func (c *Codegen) LoadImplicitCast(expr ast.Expr, typ types.Type) ir.Value {
	from := c.ExprInfo(expr)

	kind, ok := sema.GetImplicitCast(c.TypeEnv, from, typ)
	if !ok {
		return c.Load(expr)
	}

	// sema.ArrayToSlice
	if kind == sema.ArrayToSlice {
		return c.CastArrayToSlice(expr, typ)
	}

	// Normal
	value := c.Load(expr)
	return c.Cast(value, kind, from, typ, typ, expr)
}

func (c *Codegen) ImplicitCast(value ir.Value, from sema.ExprInfo, to types.Type, errNode ast.Node) ir.Value {
	if kind, ok := sema.GetImplicitCast(c.TypeEnv, from, to); ok {
		value = c.Cast(value, kind, from, to, to, errNode)
	}

	return value
}

func (c *Codegen) CastArrayToSlice(expr ast.Expr, to types.Type) ir.Value {
	from := c.ExprType(expr)

	if !c.ExprInfos[expr].Address {
		panic("codegen.Codegen.CastArrayToSlice() - Expression is not addressable")
	}

	s := to.(*types.Struct)

	sizeField := s.Field("size")
	if sizeField == nil {
		panic("codegen.Codegen.CastArrayToSlice() - Failed to find 'size' field on '" + from.String() + "'")
	}

	sizeTyp := c.Types.Get(sizeField.Type)

	sb := c.Struct(s)

	sb.Set("ptr", c.GenerateExpr(expr))
	sb.Set("size", &ir.Integer{Typ: sizeTyp, Value: core.Unsigned(false, uint64(from.(*types.Array).Size))})

	return sb.Build()
}

func (c *Codegen) Cast(value ir.Value, kind sema.CastKind, from sema.ExprInfo, to, final types.Type, errNode ast.Node) ir.Value {
	if kind == sema.Noop {
		return value
	}

	toTyp := c.Types.Get(to)

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
			panic("codegen.Codegen.Cast() - Invalid cast kind for integer literal")
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
			panic("codegen.Codegen.Cast() - Invalid cast kind for float literal")
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
			panic("codegen.Codegen.Cast() - Invalid cast kind for double literal")
		}
	}

	// Runtime conversion
	switch kind {
	case sema.ZeroExtend:
		value = c.Emitter.Ext(ir.Unsigned, value, toTyp)

	case sema.SignExtend:
		value = c.Emitter.Ext(ir.Signed, value, toTyp)

	case sema.Truncate:
		value = c.Emitter.Trunc(value, toTyp)

	case sema.IntToFloat:
		var signed bool

		switch typ := from.Type.(type) {
		case *types.Integer:
			signed = typ.Negative
		case *types.Primitive:
			signed = types.IsSignedInteger(typ.Kind)
		}

		value = c.Emitter.IntToFp(signed, value, toTyp)

	case sema.FloatToInt:
		signed := types.IsSigned(to.(*types.Primitive).Kind)
		value = c.Emitter.FpToInt(signed, value, toTyp)

	case sema.FloatExtend:
		value = c.Emitter.Ext(ir.Floating, value, toTyp)

	case sema.FloatTruncate:
		value = c.Emitter.Trunc(value, toTyp)

	case sema.IntToPointer:
		value = c.Emitter.IntToPtr(value)

	case sema.PointerToInt:
		value = c.Emitter.PtrToInt(value, toTyp)

	case sema.PointerToReference:
		c.CheckNull(value, errNode, "encountered a null pointer when converting '%s' to '%s'", from.Type, to)

	case sema.ReferenceToInterface:
		pointee, _ := getPointee(from.Type)
		sb := c.Struct(types.InterfaceUnderlying)

		sb.Set("data", value)
		sb.Set("vtable", c.GetVTable(to.(*types.Interface), pointee))

		value = sb.Build()

	case sema.PointerToInterface:
		c.CheckNull(value, errNode, "encountered a null pointer when converting '%s' to '%s'", from.Type, to)

		pointee, _ := getPointee(from.Type)
		sb := c.Struct(types.InterfaceUnderlying)

		sb.Set("data", value)
		sb.Set("vtable", c.GetVTable(to.(*types.Interface), pointee))

		value = sb.Build()

	case sema.InterfaceToPointer:
		_, dataI := c.Types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
		if dataI < 0 {
			panic("codegen.Codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
		}

		// Check pointer for null
		start := c.Emitter.Block()
		valid := c.fun.NewBlock("interface_to_ptr.valid")
		exit := c.fun.NewBlock("interface_to_ptr.exit")

		dataPtr := c.Emitter.ExtractValue(value, uint32(dataI))

		isNull := c.Emitter.ICmp(ir.Eq, false, dataPtr, &ir.Null{})
		c.Emitter.BrCond(isNull, exit, valid)

		// Valid
		c.Emitter.Begin(valid)
		value = c.SafeInterfaceToPointer(value, to.(*types.Pointer).Pointee)
		valid = c.Emitter.Block()
		c.Emitter.Br(exit)

		// Exit
		c.Emitter.Begin(exit)
		value = c.Emitter.Phi(ir.PhiPair{Block: valid, Value: value}, ir.PhiPair{Block: start, Value: &ir.Null{}})

	case sema.InterfaceToOptionReference:
		_, dataI := c.Types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
		if dataI < 0 {
			panic("codegen.Codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
		}

		// Check pointer for null
		start := c.Emitter.Block()
		valid := c.fun.NewBlock("interface_to_ref.valid")
		exit := c.fun.NewBlock("interface_to_ref.exit")

		dataPtr := c.Emitter.ExtractValue(value, uint32(dataI))

		isNull := c.Emitter.ICmp(ir.Eq, false, dataPtr, &ir.Null{})
		c.Emitter.BrCond(isNull, exit, valid)

		// Valid
		c.Emitter.Begin(valid)
		pointee, _ := getPointee(to)
		value = c.SafeInterfaceToPointer(value, pointee)
		valid = c.Emitter.Block()
		c.Emitter.Br(exit)

		// Exit
		c.Emitter.Begin(exit)
		value = c.Emitter.Phi(ir.PhiPair{Block: valid, Value: value}, ir.PhiPair{Block: start, Value: &ir.Null{}})

		// Create an option
		sb := c.Struct(final.(*types.Struct))

		sb.Set("has_value", c.Emitter.ICmp(ir.Ne, false, value, &ir.Null{}))
		sb.Set("value", value)

		value = sb.Build()

	case sema.InterfaceToOptionInterface:
		_, dataI := c.Types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
		if dataI < 0 {
			panic("codegen.Codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
		}

		_, valueI := c.Types.Get(final).(ir.StructLikeType).Field("value")
		if valueI < 0 {
			panic("codegen.Codegen.VisitBinary() - Failed to find 'value' field on 'core::Option'")
		}

		// Check pointer for null
		start := c.Emitter.Block()
		valid := c.fun.NewBlock("interface_to_interface.valid")
		exit := c.fun.NewBlock("interface_to_interface.exit")

		dataPtr := c.Emitter.ExtractValue(value, uint32(dataI))

		isNull := c.Emitter.ICmp(ir.Eq, false, dataPtr, &ir.Null{})
		c.Emitter.BrCond(isNull, exit, valid)

		// Valid
		c.Emitter.Begin(valid)
		{
			// Get type info pointers
			vtablePtr := c.Emitter.ExtractValue(value, uint32(1-dataI))
			vtableTyp := &ir.StructType{Fields: []ir.Field{{Name: "type_info", Type: ir.Pointer}}}

			srcTypeInfoPtrPtr := c.Emitter.GetElementPtrConst(vtableTyp, vtablePtr, 0, 0)
			srcTypeInfoPtr := c.Emitter.Load(ir.Pointer, srcTypeInfoPtrPtr)
			targetTypeInfoPtr := c.GetTypeInfo(to)

			// Call 'src.get_vtable(target)'
			symbol, ok := c.TypeEnv.GetInstanceMethod(c.Builtins.TypeInfo, "get_vtable")
			if !ok {
				panic("codegen.Codegen.Cast() - Failed to find 'get_vtable' method on 'core::TypeInfo'")
			}

			f := symbol.Node.(*ast.Func)
			typ := symbol.Type.(*types.Func)

			callee := c.GetFunction(f, typ, nil)
			sig := callee.Signature
			receiver := srcTypeInfoPtr

			vtableOpt := c.EmitCall(callee, sig, typ, receiver, []ir.Value{targetTypeInfoPtr}, []types.Type{&types.Pointer{Pointee: c.Builtins.TypeInfo}}, typ.Returns)
			vtable := c.Emitter.ExtractValue(vtableOpt, uint32(valueI))
			isTarget := c.Emitter.ICmp(ir.Ne, false, vtable, &ir.ZeroInitializer{Typ: ir.I64})

			// Extract pointer or null
			ptr := c.ExtractPointerFromInterfaceOrNull(value, isTarget)

			// Reconstruct interface with the new value and vtable
			sb := c.Struct(types.InterfaceUnderlying)

			sb.Set("data", ptr)
			sb.Set("vtable", vtable)

			value = sb.Build()
		}
		valid = c.Emitter.Block()
		c.Emitter.Br(exit)

		// Exit
		c.Emitter.Begin(exit)
		value = c.Emitter.Phi(ir.PhiPair{Block: valid, Value: value}, ir.PhiPair{Block: start, Value: &ir.ZeroInitializer{Typ: c.Types.Get(types.InterfaceUnderlying)}})

		// Create an option
		sb := c.Struct(final.(*types.Struct))

		dataPtr = c.Emitter.ExtractValue(value, uint32(dataI))

		sb.Set("has_value", c.Emitter.ICmp(ir.Ne, false, dataPtr, &ir.Null{}))
		sb.Set("value", value)

		value = sb.Build()

	case sema.TypeToOption:
		to := to.(*types.Struct)
		toInner := to.Substitutions[0].Type

		sb := c.Struct(to)

		sb.Set("has_value", ir.True)
		sb.Set("value", c.ImplicitCast(value, from, toInner, errNode))

		return sb.Build()

	case sema.ImplicitAs:
		callee, sig, fTyp := c.ResolveInterfaceMethod(from.Type, "implicit_as", false)
		receiver := c.ValueToReceiver(value, from.Type, false, errNode)

		return c.EmitCallExpr(callee, sig, fTyp, receiver, nil, to)

	case sema.ArrayToSlice:
		panic("codegen.Codegen.Cast() - ArrayToSlice should have been handled before calling Cast()")

	default:
		panic("codegen.Codegen.Cast() - Invalid cast kind")
	}

	return value
}

func (c *Codegen) CheckNull(value ir.Value, node ast.Node, format string, args ...any) {
	null := c.fun.NewBlock("null_check.null")
	valid := c.fun.NewBlock("null_check.valid")

	isNull := c.Emitter.ICmp(ir.Eq, false, value, &ir.Null{})
	c.Emitter.BrCond(isNull, null, valid)

	// Null
	c.Emitter.Begin(null)
	c.EmitPanic(node, format, args...)
	c.Emitter.Br(valid) // no-op terminator instruction

	// Valid
	c.Emitter.Begin(valid)
}

func (c *Codegen) SafeInterfaceToPointer(value ir.Value, pointee types.Type) ir.Value {
	_, vtableI := c.Types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("vtable")
	if vtableI < 0 {
		panic("codegen.Codegen.VisitBinary() - Failed to find 'vtable' field on 'core::Interface'")
	}

	// Get type info pointers
	vtablePtr := c.Emitter.ExtractValue(value, uint32(vtableI))
	vtableTyp := &ir.StructType{Fields: []ir.Field{{Name: "type_info", Type: ir.Pointer}}}

	srcTypeInfoPtrPtr := c.Emitter.GetElementPtrConst(vtableTyp, vtablePtr, 0, 0)
	srcTypeInfoPtr := c.Emitter.Load(ir.Pointer, srcTypeInfoPtrPtr)
	targetTypeInfoPtr := c.GetTypeInfo(pointee)

	// Check if pointers are the same
	isTarget := c.Emitter.ICmp(ir.Eq, false, srcTypeInfoPtr, targetTypeInfoPtr)

	// Extract pointer or null
	return c.ExtractPointerFromInterfaceOrNull(value, isTarget)
}

func (c *Codegen) ExtractPointerFromInterfaceOrNull(value, isTarget ir.Value) ir.Value {
	_, dataI := c.Types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
	if dataI < 0 {
		panic("codegen.Codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
	}

	start := c.Emitter.Block()
	ptr := c.fun.NewBlock("interface_check.ptr")
	exit := c.fun.NewBlock("interface_check.exit")

	c.Emitter.BrCond(isTarget, ptr, exit)

	// Ptr
	c.Emitter.Begin(ptr)
	value = c.Emitter.ExtractValue(value, uint32(dataI))
	c.Emitter.Br(exit)

	// Exit
	c.Emitter.Begin(exit)
	return c.Emitter.Phi(ir.PhiPair{Block: ptr, Value: value}, ir.PhiPair{Block: start, Value: &ir.Null{}})
}

func (c *Codegen) EmitCmp(op ir.CmpOp, left, right ast.Expr) ir.Value {
	leftType := c.ExprType(left)
	rightType := c.ExprType(right)

	common := sema.CommonType(c.TypeEnv, leftType, rightType)
	if common == nil {
		common = leftType
	}

	leftV := c.LoadImplicitCast(left, common)
	rightV := c.LoadImplicitCast(right, common)

	// Interface
	if _, ok := common.(*types.Interface); ok {
		_, dataI := c.Types.Get(types.InterfaceUnderlying).(ir.StructLikeType).Field("data")
		if dataI < 0 {
			panic("codegen.Codegen.VisitBinary() - Failed to find 'data' field on 'core::Interface'")
		}

		leftPtr := leftV
		if s, ok := leftV.Type().(*ir.RefStructType); ok && s.Name == types.InterfaceUnderlying.Name {
			leftPtr = c.Emitter.ExtractValue(leftV, uint32(dataI))
		}

		rightPtr := rightV
		if s, ok := rightV.Type().(*ir.RefStructType); ok && s.Name == types.InterfaceUnderlying.Name {
			rightPtr = c.Emitter.ExtractValue(rightV, uint32(dataI))
		}

		return c.Emitter.ICmp(op, false, leftPtr, rightPtr)
	}

	// Enum
	if t, ok := common.(*types.Enum); ok {
		signed := types.IsSignedInteger(t.CaseType.(*types.Primitive).Kind)
		return c.Emitter.ICmp(op, signed, leftV, rightV)
	}

	// Pointer
	if _, ok := common.(*types.Pointer); ok {
		return c.Emitter.ICmp(op, false, leftV, rightV)
	}

	// Reference
	if _, ok := common.(*types.Reference); ok {
		return c.Emitter.ICmp(op, false, leftV, rightV)
	}

	// Primitive
	prim := common.(*types.Primitive).Kind

	if types.IsFloating(prim) {
		return c.Emitter.FCmp(op, false, leftV, rightV)
	}

	signed := types.IsSignedInteger(prim)
	return c.Emitter.ICmp(op, signed, leftV, rightV)
}

func (c *Codegen) GetDivKind(expr ast.Expr) ir.DivKind {
	prim := c.UnderlyingExprType(expr).(*types.Primitive).Kind
	kind := ir.Floating

	if types.IsSignedInteger(prim) {
		kind = ir.Signed
	} else if types.IsUnsignedInteger(prim) {
		kind = ir.Unsigned
	}

	return kind
}

func (c *Codegen) ExprInfo(expr ast.Expr) sema.ExprInfo {
	info := c.ExprInfos[expr]
	info.Type = c.ResolveType(info.Type)

	return info
}

func (c *Codegen) UnderlyingExprType(expr ast.Expr) types.Type {
	typ := c.ExprType(expr)

	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	return typ
}

func (c *Codegen) ExprType(expr ast.Expr) types.Type {
	return c.ResolveType(c.ExprInfos[expr].Type)
}

func (c *Codegen) NodeType(node ast.Node) types.Type {
	typ := c.NodeTypes[node]
	typ = c.ResolveType(typ)

	return typ
}

func (c *Codegen) UnderlyingNodeType(node ast.Node) types.Type {
	typ := c.NodeType(node)

	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	return typ
}

func (c *Codegen) Load(expr ast.Expr) ir.Value {
	value := c.GenerateExpr(expr)

	if info := c.ExprInfos[expr]; info.Address {
		typ := c.ResolveType(info.Type)
		if t, ok := typ.(types.Composed); ok {
			typ = t.Underlying()
		}

		value = c.Emitter.Load(c.Types.Get(typ), value)
	}

	return value
}

func (c *Codegen) ResolveType(typ types.Type) types.Type {
	if len(c.substitutions) == 0 {
		return typ
	}

	return c.instantiations.Substitute(typ, c.substitutions)
}

func (c *Codegen) GenerateExpr(expr ast.Expr) ir.Value {
	c.Emitter.SetDebugLocation(expr.Range().Start)
	return ast.VisitExpr(c, expr)
}
