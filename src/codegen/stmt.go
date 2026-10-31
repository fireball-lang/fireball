package codegen

import (
	"fireball/ast"
	"fireball/core"
	"fireball/ir"
	"fireball/types"
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
