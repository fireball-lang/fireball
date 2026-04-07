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

func (c *codegen) VisitBool(b *ast.Bool) {
	if b.Value {
		c.value = ir.True
	} else {
		c.value = ir.False
	}
}

func (c *codegen) VisitNumber(n *ast.Number) {
	// Integer
	if lexer.IsInteger(n.Token.Kind) {
		c.value = &ir.Integer{
			Typ:   c.types.Get(c.UnderlyingExprType(n)),
			Value: core.Unsigned(false, lexer.ParseInteger(n.Token)),
		}

		return
	}

	// Float
	if n.Token.Kind == lexer.Decimal32bit {
		value, err := strconv.ParseFloat(n.Token.Text[:len(n.Token.Text)-1], 32)
		if err != nil {
			panic("codegen.codegen.VisitNumber() - Failed to parse float '" + n.Token.Text + "'")
		}

		c.value = &ir.FloatV{Value: float32(value)}

		return
	}

	// Double
	if n.Token.Kind == lexer.Decimal {
		value, err := strconv.ParseFloat(n.Token.Text, 64)
		if err != nil {
			panic("codegen.codegen.VisitNumber() - Failed to parse double '" + n.Token.Text + "'")
		}

		c.value = &ir.DoubleV{Value: value}

		return
	}

	// Unknown
	panic("codegen.codegen.VisitNumber() - Invalid token kind")
}

func (c *codegen) VisitCharacter(e *ast.Character) {
	c.value = &ir.Integer{
		Typ:   ir.I32,
		Value: core.Unsigned(false, uint64(e.Rune)),
	}
}

func (c *codegen) VisitString(s *ast.String) {
	value := ir.NewString(s.Runes, true)

	global := c.module.NewGlobalVar(fmt.Sprintf("string.%d", c.stringCount), value.Type())
	c.stringCount++

	global.Flags = ir.Private | ir.UnnamedAddr | ir.Constant
	global.Initializer = value

	c.value = global
}

func (c *codegen) VisitPrefix(u *ast.Prefix) {
	switch u.Op {
	case ast.Negate:
		value := c.Load(u.Expr)

		// Floating
		if typ := c.UnderlyingExprType(u.Expr); typ == types.PrimitiveF32 || typ == types.PrimitiveF64 {
			c.value = c.emitter.Fneg(value)
			return
		}

		// Integer
		zero := &ir.Integer{Typ: value.Type(), Value: core.Signed(0)}
		c.value = c.emitter.Sub(zero, value)

	case ast.Not:
		value := c.LoadImplicitCast(u.Expr, types.PrimitiveBool)
		c.value = c.emitter.Xor(value, ir.True)

	default:
		panic("codegen.codegen.VisitPrefix() - Invalid operator")
	}
}

func (c *codegen) VisitBinary(b *ast.Binary) {
	switch b.Op {
	// Math

	case ast.Add:
		typ := c.exprInfos[b].Type
		c.value = c.emitter.Add(c.LoadImplicitCast(b.Left, typ), c.LoadImplicitCast(b.Right, typ))

	case ast.Subtract:
		typ := c.exprInfos[b].Type
		c.value = c.emitter.Sub(c.LoadImplicitCast(b.Left, typ), c.LoadImplicitCast(b.Right, typ))

	case ast.Multiply:
		typ := c.exprInfos[b].Type
		c.value = c.emitter.Mul(c.LoadImplicitCast(b.Left, typ), c.LoadImplicitCast(b.Right, typ))

	case ast.Divide:
		typ := c.exprInfos[b].Type
		kind := c.GetDivKind(b.Left)
		c.value = c.emitter.Div(kind, c.LoadImplicitCast(b.Left, typ), c.LoadImplicitCast(b.Right, typ))

	case ast.Modulo:
		typ := c.exprInfos[b].Type
		kind := c.GetDivKind(b.Left)
		c.value = c.emitter.Rem(kind, c.LoadImplicitCast(b.Left, typ), c.LoadImplicitCast(b.Right, typ))

	// Bitwise

	case ast.BitOr:
		typ := c.exprInfos[b].Type
		c.value = c.emitter.Or(c.LoadImplicitCast(b.Left, typ), c.LoadImplicitCast(b.Right, typ))

	case ast.BitXor:
		typ := c.exprInfos[b].Type
		c.value = c.emitter.Xor(c.LoadImplicitCast(b.Left, typ), c.LoadImplicitCast(b.Right, typ))

	case ast.BitAnd:
		typ := c.exprInfos[b].Type
		c.value = c.emitter.And(c.LoadImplicitCast(b.Left, typ), c.LoadImplicitCast(b.Right, typ))

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

		c.value = c.emitter.Phi(
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

		c.value = c.emitter.Phi(
			ir.PhiPair{Block: bLeft, Value: ir.True},
			ir.PhiPair{Block: bRight, Value: right},
		)

	// Equality

	case ast.Equal:
		c.value = c.EmitCmp(ir.Eq, b.Left, b.Right)

	case ast.NotEqual:
		c.value = c.EmitCmp(ir.Ne, b.Left, b.Right)

	// Relational

	case ast.Less:
		c.value = c.EmitCmp(ir.Lt, b.Left, b.Right)

	case ast.LessEqual:
		c.value = c.EmitCmp(ir.Le, b.Left, b.Right)

	case ast.Greater:
		c.value = c.EmitCmp(ir.Gt, b.Left, b.Right)

	case ast.GreaterEqual:
		c.value = c.EmitCmp(ir.Ge, b.Left, b.Right)

	// Assignment

	case ast.Assign:
		ptr := c.GenerateExpr(b.Left)
		value := c.LoadImplicitCast(b.Right, c.exprInfos[b.Left].Type)

		c.emitter.Store(value, ptr)
		c.value = value

	default:
		panic("codegen.codegen.VisitBinary() - Invalid operator kind")
	}
}

func (c *codegen) VisitIdentifier(i *ast.Identifier) {
	switch node := c.exprInfos[i].Node.(type) {
	case *ast.Func:
		c.value = c.GetFunction(node)

	case *ast.NameType:
		c.value = c.scope.Get(node.Name.Token.Text)

	case *ast.Var:
		c.value = c.scope.Get(node.Name.Token.Text)

	default:
		panic("codegen.codegen.VisitIdentifier() - Invalid node")
	}
}

func (c *codegen) VisitIndex(i *ast.Index) {
	typ := c.types.Get(c.UnderlyingExprType(i.Expr))

	var ptr ir.Value

	// Get pointer to expression
	if c.exprInfos[i].Address {
		ptr = c.GenerateExpr(i.Expr)
	} else {
		value := c.GenerateExpr(i.Expr)

		ptr = c.Alloca(typ, "index")
		c.emitter.Store(value, ptr)
	}

	// Index
	index := c.Load(i.Index)
	c.value = c.emitter.GetElementPtrDyn(typ, ptr, ir.False, index)
}

func (c *codegen) VisitMember(m *ast.Member) {
	s := c.UnderlyingExprType(m.Expr).(*types.Struct)
	_, index := s.Field(m.Name.Token.Text)

	// Pointer
	if c.exprInfos[m].Address {
		ptr := c.GenerateExpr(m.Expr)
		typ := c.types.Get(s)

		c.value = c.emitter.GetElementPtrConst(typ, ptr, 0, uint32(index))
		return
	}

	// Value
	value := c.GenerateExpr(m.Expr)
	c.value = c.emitter.ExtractValue(value, uint32(index))
}

func (c *codegen) VisitCall(e *ast.Call) {
	f := c.exprInfos[e.Callee].Type.(*types.Func)
	callee := c.Load(e.Callee).(*ir.Function)
	args := make([]ir.Value, 0, len(e.Args))

	// Arguments
	for i, arg := range e.Args {
		var valueType types.Type
		var value ir.Value

		if i < len(f.Params) {
			valueType = f.Params[i]
			value = c.LoadImplicitCast(arg, valueType)
		} else {
			valueType = c.UnderlyingExprType(arg)
			value = c.Load(arg)
		}

		classes, info := c.callConv.Classify(c.arch, valueType)

		// Pointer
		if len(classes) == 1 && classes[0] == abi.Memory {
			ptr := c.Alloca(value.Type(), "call.param")
			c.emitter.Store(value, ptr)

			args = append(args, ptr)
			continue
		}

		// Value
		typ := getTypeForClasses(classes, info.Size)
		value = c.BitCast(value, typ)

		args = append(args, value)
	}

	// Return value
	returnTyp := c.UnderlyingExprType(e)
	returnClasses, _ := c.callConv.Classify(c.arch, returnTyp)

	var returnPtr ir.Value

	if len(returnClasses) == 1 && returnClasses[0] == abi.Memory {
		returnPtr = c.Alloca(c.types.Get(returnTyp), "call.sret")
		args = slices.Insert(args, 0, returnPtr)
	}

	// Call
	value := c.emitter.Call(callee.Signature, callee, args)

	// Return value
	if core.IsNil(returnPtr) {
		typ := c.types.Get(returnTyp)
		c.value = c.BitCast(value, typ)
	} else {
		typ := c.types.Get(returnTyp)
		c.value = c.emitter.Load(typ, returnPtr)
	}
}

func (c *codegen) VisitCast(e *ast.Cast) {
	value := c.Load(e.Expr)

	from := c.exprInfos[e.Expr].Type
	to := c.exprInfos[e].Type

	kind, _ := sema.GetExplicitCast(from, to)

	c.value = c.Cast(value, kind, from, to)
}

func (c *codegen) VisitBadExpr(_ *ast.BadExpr) {}

// Utils

func (c *codegen) LoadImplicitCast(expr ast.Expr, typ types.Type) ir.Value {
	value := c.Load(expr)
	return c.ImplicitCast(value, c.exprInfos[expr].Type, typ)
}

func (c *codegen) ImplicitCast(value ir.Value, from, to types.Type) ir.Value {
	if kind, ok := sema.GetImplicitCast(from, to); ok {
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

	default:
		panic("codegen.codegen.Cast() - Invalid cast kind")
	}

	return value
}

func (c *codegen) EmitCmp(op ir.CmpOp, left, right ast.Expr) ir.Value {
	leftType := c.exprInfos[left].Type
	rightType := c.exprInfos[right].Type

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
	typ := c.exprInfos[expr].Type

	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	return typ
}

func (c *codegen) Load(expr ast.Expr) ir.Value {
	value := c.GenerateExpr(expr)

	if info := c.exprInfos[expr]; info.Address {
		typ := info.Type
		if t, ok := typ.(types.Composed); ok {
			typ = t.Underlying()
		}

		value = c.emitter.Load(c.types.Get(typ), value)
	}

	return value
}

func (c *codegen) GenerateExpr(expr ast.Expr) ir.Value {
	//c.emitter.SetDebugLocation(expr.Range().Start)
	expr.VisitExpr(c)

	value := c.value
	c.value = nil

	return value
}
