package codegen

import (
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/types"
)

// Visitor

func (c *codegen) VisitBlock(b *ast.Block) {
	c.emitter.PushScope(c.emitMetaScope(b))
	c.scope.Push()

	for _, stmt := range b.Stmts {
		c.GenerateStmt(stmt)
	}

	c.scope.Pop()
	c.emitter.PopScope()
}

func (c *codegen) VisitExpression(e *ast.Expression) {
	c.GenerateExpr(e.Expr)
}

func (c *codegen) VisitVar(v *ast.Var) {
	// Type
	typ := c.types.Get(c.nodeTypes[v])

	// Pointer
	ptr := c.Alloca(typ, "var."+v.Name.Token.Text)

	c.emitDbgDeclare(v.Name.Token.Text, c.nodeTypes[v], ptr, 0, v.Name)

	// Value
	var value ir.Value

	if core.IsNil(v.Initializer) {
		value = &ir.ZeroInitializer{Typ: typ}
	} else {
		value = c.LoadImplicitCast(v.Initializer, c.nodeTypes[v])
	}

	// Variable
	c.emitter.Store(value, ptr)
	c.scope.Add(v.Name.Token.Text, ptr)
}

func (c *codegen) VisitIf(i *ast.If) {
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
	c.emitter.BrCond(condition, bThen, bElse)

	// Then
	c.emitter.Begin(bThen)
	c.GenerateStmt(i.BranchTrue)
	c.emitter.Br(bExit)

	// Else
	if !core.IsNil(i.BranchFalse) {
		c.emitter.Begin(bElse)
		c.GenerateStmt(i.BranchFalse)
		c.emitter.Br(bExit)
	}

	// Exit
	c.emitter.Begin(bExit)
}

func (c *codegen) VisitWhile(w *ast.While) {
	bCondition := c.fun.NewBlock("while.condition")
	bBody := c.fun.NewBlock("while.body")
	bExit := c.fun.NewBlock("while.exit")

	prevBLoopBreak := c.bLoopBreak
	c.bLoopBreak = bExit

	prevBLoopContinue := c.bLoopContinue
	c.bLoopContinue = bCondition

	// Condition
	c.emitter.Br(bCondition)
	c.emitter.Begin(bCondition)

	condition := c.LoadImplicitCast(w.Condition, types.PrimitiveBool)
	c.emitter.BrCond(condition, bBody, bExit)

	// Body
	c.emitter.Begin(bBody)
	c.GenerateStmt(w.Body)
	c.emitter.Br(bCondition)

	// Exit
	c.emitter.Begin(bExit)

	c.bLoopBreak = prevBLoopBreak
	c.bLoopContinue = prevBLoopContinue
}

func (c *codegen) VisitFor(f *ast.For) {
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

	c.emitter.PushScope(c.emitMetaScope(f))
	c.scope.Push()

	// Initializer
	if !core.IsNil(f.Initializer) {
		c.GenerateStmt(f.Initializer)
	}

	// Condition
	if bCondition != nil {
		c.emitter.Br(bCondition)
		c.emitter.Begin(bCondition)

		condition := c.LoadImplicitCast(f.Condition, types.PrimitiveBool)
		c.emitter.BrCond(condition, bBody, bExit)
	} else {
		c.emitter.Br(bBody)
	}

	// Body
	c.emitter.Begin(bBody)
	c.GenerateStmt(f.Body)
	c.emitter.Br(c.bLoopContinue)

	// Increment
	if bIncrement != nil {
		c.emitter.Begin(bIncrement)
		c.GenerateExpr(f.Increment)

		next := bBody
		if bCondition != nil {
			next = bCondition
		}

		c.emitter.Br(next)
	}

	// Exit
	c.emitter.Begin(bExit)

	c.scope.Pop()
	c.emitter.PopScope()

	c.bLoopBreak = prevBLoopBreak
	c.bLoopContinue = prevBLoopContinue
}

func (c *codegen) VisitReturn(r *ast.Return) {
	var value ir.Value

	if !core.IsNil(r.Value) {
		f := c.nodeTypes[ast.GetClosestParent[*ast.Func](r)].(*types.Func)
		value = c.LoadImplicitCast(r.Value, f.Returns)

		if core.IsNil(c.returnPtr) {
			value = c.BitCast(value, c.fun.Signature.Returns)
		} else {
			c.emitter.Store(value, c.returnPtr)
			value = nil
		}
	}

	c.emitter.Ret(value)
}

func (c *codegen) VisitBreak(_ *ast.Break) {
	c.emitter.Br(c.bLoopBreak)
}

func (c *codegen) VisitContinue(_ *ast.Continue) {
	c.emitter.Br(c.bLoopContinue)
}

func (c *codegen) VisitBadStmt(_ *ast.BadStmt) {}

// Utils

func (c *codegen) GenerateStmt(stmt ast.Stmt) {
	c.emitter.SetDebugLocation(stmt.Range().Start)
	ast.VisitStmt(c, stmt)
}
