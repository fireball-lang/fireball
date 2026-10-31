package codegen

import (
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/sema"
	"fireball/types"
	"fmt"
)

// Visitor

func (c *Codegen) VisitBlock(b *ast.Block) {
	c.Emitter.PushScope(c.emitMetaScope(b))
	c.scope.Push()

	for _, stmt := range b.Stmts {
		c.GenerateStmt(stmt)
	}

	c.scope.Pop()
	c.Emitter.PopScope()
}

func (c *Codegen) VisitExpression(e *ast.Expression) {
	c.GenerateExpr(e.Expr)
}

func (c *Codegen) VisitVar(v *ast.Var) {
	// Type
	varTyp := c.ResolveType(c.Sema.NodeTypes[v])
	typ := c.Types.Get(varTyp)

	// Pointer
	ptr := c.Alloca(typ, "var."+v.Name.Token.Text)

	c.emitDbgDeclare(v.Name.Token.Text, varTyp, ptr, 0, v.Name)

	// Value
	var value ir.Value

	if core.IsNil(v.Initializer) {
		value = &ir.ZeroInitializer{Typ: typ}
	} else {
		value = c.LoadImplicitCast(v.Initializer, varTyp)
	}

	// Variable
	c.Emitter.Store(value, ptr)
	c.scope.Add(v.Name.Token.Text, ptr)
}

func (c *Codegen) VisitIf(i *ast.If) {
	bThen := c.fun.NewBlock("if.then")

	var bElse *ir.Block
	if !core.IsNil(i.BranchFalse) {
		bElse = c.fun.NewBlock("if.else")
	}

	bExit := c.fun.NewBlock("if.exit")
	if bElse == nil {
		bElse = bExit
	}

	// Condition
	condition := c.LoadImplicitCast(i.Condition, types.PrimitiveBool)
	c.Emitter.BrCond(condition, bThen, bElse)

	// Then
	c.Emitter.Begin(bThen)
	c.GenerateStmt(i.BranchTrue)
	c.Emitter.Br(bExit)

	// Else
	if !core.IsNil(i.BranchFalse) {
		c.Emitter.Begin(bElse)
		c.GenerateStmt(i.BranchFalse)
		c.Emitter.Br(bExit)
	}

	// Exit
	c.Emitter.Begin(bExit)
}

type armBlocks struct {
	condition *ir.Block
	body      *ir.Block
}

func (c *Codegen) VisitMatch(m *ast.Match) {
	// Empty match
	if len(m.Arms) == 0 {
		c.GenerateExpr(m.Value)
		return
	}

	// Match with a single fallback arm
	if len(m.Arms) == 1 && len(m.Arms[0].Patterns) == 1 && c.Sema.ArmPatterns[m.Arms[0]][0] == sema.Fallback {
		c.GenerateExpr(m.Value)

		c.Emitter.PushScope(c.emitMetaScope(m.Arms[0]))
		c.scope.Push()

		c.GenerateStmt(m.Arms[0].Body)

		c.scope.Pop()
		c.Emitter.PopScope()

		return
	}

	// Create arm blocks
	blocks := make([]armBlocks, len(m.Arms))

	for i, arm := range m.Arms {
		patterns := c.Sema.ArmPatterns[arm]

		if len(patterns) == 1 && patterns[0] == sema.Fallback {
			bBody := c.fun.NewBlock(fmt.Sprintf("match.body.%d", i))
			blocks[i] = armBlocks{condition: bBody, body: bBody}
		} else {
			blocks[i] = armBlocks{
				condition: c.fun.NewBlock(fmt.Sprintf("match.condition.%d", i)),
				body:      c.fun.NewBlock(fmt.Sprintf("match.body.%d", i)),
			}
		}
	}

	bExit := c.fun.NewBlock("match.exit")

	// Prepare receiver / primitive value for comparisons
	value := ir.Value(nil)
	typ := c.ExprType(m.Value)

	if _, ok := typ.(*types.Primitive); !ok {
		if fTyp, _ := sema.GetBinaryMethod(c.Ctx, ast.Equal.InterfaceName(), typ, typ); fTyp != nil {
			if _, ok := typ.(*types.Interface); ok {
				value = c.Load(m.Value)
			} else {
				value = c.ResolveReceiver(m.Value)
			}
		}
	}

	if value == nil {
		value = c.Load(m.Value)
	}

	// Start
	c.Emitter.Br(blocks[0].condition)

	// Arms
	for i, arm := range m.Arms {
		bCondition := blocks[i].condition
		bBody := blocks[i].body

		// Condition
		if bCondition != bBody {
			bNext := bExit
			if i+1 < len(m.Arms) {
				bNext = blocks[i+1].condition
			}

			c.Emitter.Begin(bCondition)
			matched := c.EmitCmpPatterns(m.Value, value, typ, c.Sema.ArmPatterns[arm], i)
			c.Emitter.BrCond(matched, bBody, bNext)
		}

		// Body
		c.Emitter.PushScope(c.emitMetaScope(arm))
		c.scope.Push()

		c.Emitter.Begin(bBody)
		c.GenerateStmt(arm.Body)
		c.Emitter.Br(bExit)

		c.scope.Pop()
		c.Emitter.PopScope()
	}

	// Exit
	c.Emitter.Begin(bExit)
}

func (c *Codegen) EmitCmpPatterns(valueExpr ast.Expr, value ir.Value, typ types.Type, patterns []sema.Pattern, armI int) ir.Value {
	// Single
	if len(patterns) == 1 {
		return c.EmitCmpPattern(valueExpr, value, typ, patterns[0])
	}

	// Multiple
	bExit := c.fun.NewBlock(fmt.Sprintf("match.condition.%d.exit", armI))
	pairs := make([]ir.PhiPair, len(patterns))

	for i, pattern := range patterns {
		matched := c.EmitCmpPattern(valueExpr, value, typ, pattern)
		from := c.Emitter.Block()

		if i == len(patterns)-1 {
			pairs[i] = ir.PhiPair{Block: from, Value: matched}
			c.Emitter.Br(bExit)
		} else {
			bNext := c.fun.NewBlock(fmt.Sprintf("match.condition.%d.pattern.%d", armI, i+1))
			pairs[i] = ir.PhiPair{Block: from, Value: ir.True}

			c.Emitter.BrCond(matched, bExit, bNext)
			c.Emitter.Begin(bNext)
		}
	}

	c.Emitter.Begin(bExit)
	return c.Emitter.Phi(pairs...)
}

func (c *Codegen) EmitCmpPattern(valueExpr ast.Expr, value ir.Value, typ types.Type, pattern sema.Pattern) ir.Value {
	expr := pattern.(*sema.ExprPattern).Expr
	target := c.GetIrValue(c.Sema.Evaluations[expr], typ)

	// core::Eq
	if _, ok := typ.(*types.Primitive); !ok {
		if fTyp, fName := sema.GetBinaryMethod(c.Ctx, ast.Equal.InterfaceName(), typ, typ); fTyp != nil {
			var callee ir.Value
			var sig *ir.Signature
			var receiver ir.Value

			_, isInterfaceDispatch := typ.(*types.Interface)

			if isInterfaceDispatch {
				// Interface dispatch
				callee, receiver = c.LookupInterfaceMethod(typ.(*types.Interface), value, fName, valueExpr)
				sig = c.BuildCallSignature(fTyp, true)
			} else {
				// Method from interface, resolved to concrete impl
				callee, sig, fTyp = c.ResolveInterfaceMethod(typ, fName, false)
				receiver = value
			}

			rhsType := fTyp.Params[1]
			target = c.ImplicitCast(target, sema.ExprInfo{Type: typ}, rhsType, expr)

			return c.EmitCall(callee, sig, fTyp, receiver, []ir.Value{target}, []types.Type{rhsType}, types.PrimitiveBool)
		}
	}

	// Native
	if sema.HasNativeEquality(typ) {
		return c.EmitCmpRaw(ir.Eq, typ, value, target)
	}

	// Invalid
	panic("codegen.Codegen.EmitCmpPattern - Invalid type")
}

func (c *Codegen) VisitWhile(w *ast.While) {
	bCondition := c.fun.NewBlock("while.condition")
	bBody := c.fun.NewBlock("while.body")
	bExit := c.fun.NewBlock("while.exit")

	prevBLoopBreak := c.bLoopBreak
	c.bLoopBreak = bExit

	prevBLoopContinue := c.bLoopContinue
	c.bLoopContinue = bCondition

	// Condition
	c.Emitter.Br(bCondition)
	c.Emitter.Begin(bCondition)

	condition := c.LoadImplicitCast(w.Condition, types.PrimitiveBool)
	c.Emitter.BrCond(condition, bBody, bExit)

	// Body
	c.Emitter.Begin(bBody)
	c.GenerateStmt(w.Body)
	c.Emitter.Br(bCondition)

	// Exit
	c.Emitter.Begin(bExit)

	c.bLoopBreak = prevBLoopBreak
	c.bLoopContinue = prevBLoopContinue
}

func (c *Codegen) VisitFor(f *ast.For) {
	var bCondition *ir.Block
	if !core.IsNil(f.Condition) {
		bCondition = c.fun.NewBlock("for.condition")
	}

	bBody := c.fun.NewBlock("for.body")

	var bIncrement *ir.Block
	if !core.IsNil(f.Increment) {
		bIncrement = c.fun.NewBlock("for.increment")
	}

	bExit := c.fun.NewBlock("for.exit")

	prevBLoopBreak := c.bLoopBreak
	c.bLoopBreak = bExit

	prevBLoopContinue := c.bLoopContinue
	c.bLoopContinue = bBody
	if bCondition != nil {
		c.bLoopContinue = bCondition
	}
	if bIncrement != nil {
		c.bLoopContinue = bIncrement
	}

	c.Emitter.PushScope(c.emitMetaScope(f))
	c.scope.Push()

	// Initializer
	if !core.IsNil(f.Initializer) {
		c.GenerateStmt(f.Initializer)
	}

	// Condition
	if bCondition != nil {
		c.Emitter.Br(bCondition)
		c.Emitter.Begin(bCondition)

		condition := c.LoadImplicitCast(f.Condition, types.PrimitiveBool)
		c.Emitter.BrCond(condition, bBody, bExit)
	} else {
		c.Emitter.Br(bBody)
	}

	// Body
	c.Emitter.Begin(bBody)
	c.GenerateStmt(f.Body)
	c.Emitter.Br(c.bLoopContinue)

	// Increment
	if bIncrement != nil {
		c.Emitter.Begin(bIncrement)
		c.GenerateExpr(f.Increment)

		next := bBody
		if bCondition != nil {
			next = bCondition
		}

		c.Emitter.Br(next)
	}

	// Exit
	c.Emitter.Begin(bExit)

	c.scope.Pop()
	c.Emitter.PopScope()

	c.bLoopBreak = prevBLoopBreak
	c.bLoopContinue = prevBLoopContinue
}

func (c *Codegen) VisitReturn(r *ast.Return) {
	var value ir.Value

	if !core.IsNil(r.Value) {
		value = c.LoadImplicitCast(r.Value, c.ResolveType(c.funcTyp.Returns))
	}

	c.ReturnValue(value)
}

func (c *Codegen) VisitBreak(_ *ast.Break) {
	c.Emitter.Br(c.bLoopBreak)
}

func (c *Codegen) VisitContinue(_ *ast.Continue) {
	c.Emitter.Br(c.bLoopContinue)
}

func (c *Codegen) VisitBadStmt(_ *ast.BadStmt) {}

// Utils

func (c *Codegen) ReturnValue(value ir.Value) {
	if !core.IsNil(value) {
		if core.IsNil(c.returnPtr) {
			value = c.BitCast(value, c.fun.Signature.Returns)
		} else {
			c.Emitter.Store(value, c.returnPtr)
			value = nil
		}
	}

	c.Emitter.Ret(value)
}

func (c *Codegen) GenerateStmt(stmt ast.Stmt) {
	c.Emitter.SetDebugLocation(stmt.Range().Start)
	ast.VisitStmt(c, stmt)
}
