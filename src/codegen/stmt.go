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

func (c *Codegen) VisitBlock(b *ast.Block) int {
	c.Emitter.PushScope(c.emitMetaScope(b))
	c.scope.Push()

	for _, stmt := range b.Stmts {
		c.GenerateStmt(stmt)

		if !c.CanFallthrough(stmt) {
			break
		}
	}

	c.scope.Pop()
	c.Emitter.PopScope()

	return 0
}

func (c *Codegen) VisitExpression(e *ast.Expression) int {
	c.GenerateExpr(e.Expr)

	return 0
}

func (c *Codegen) VisitVar(v *ast.Var) int {
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

	return 0
}

func (c *Codegen) VisitIf(i *ast.If) int {
	// Handle literal ast.Bool condition
	if condition, ok := i.Condition.(*ast.Bool); ok {
		branch := i.BranchFalse
		if condition.Value {
			branch = i.BranchTrue
		}

		if !core.IsNil(branch) {
			c.GenerateStmt(branch)
		}

		return 0
	}

	// Create blocks
	bThen := c.fun.NewBlock("if.then")

	var bElse *ir.Block
	if !core.IsNil(i.BranchFalse) {
		bElse = c.fun.NewBlock("if.else")
	}

	var bExit *ir.Block
	if c.CanFallthrough(i) {
		bExit = c.fun.NewBlock("if.exit")
	}

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
	if bExit != nil {
		c.Emitter.Begin(bExit)
	}

	return 0
}

type armBlocks struct {
	condition *ir.Block
	body      *ir.Block
}

func (c *Codegen) VisitMatch(m *ast.Match) int {
	// Empty match
	if len(m.Arms) == 0 {
		c.GenerateExpr(m.Value)
		return 0
	}

	// Match with a single fallback arm
	if len(m.Arms) == 1 && len(m.Arms[0].Patterns) == 1 && c.Sema.ArmPatterns[m.Arms[0]][0] == sema.Fallback {
		c.GenerateExpr(m.Value)

		c.Emitter.PushScope(c.emitMetaScope(m.Arms[0]))
		c.scope.Push()

		c.GenerateStmt(m.Arms[0].Body)

		c.scope.Pop()
		c.Emitter.PopScope()

		return 0
	}

	// Create arm blocks
	blocks := make([]armBlocks, len(m.Arms))

	for i := range m.Arms {
		if i == len(m.Arms)-1 {
			bBody := c.fun.NewBlock(fmt.Sprintf("match.body.%d", i))
			blocks[i] = armBlocks{condition: bBody, body: bBody}
		} else {
			blocks[i] = armBlocks{
				condition: c.fun.NewBlock(fmt.Sprintf("match.condition.%d", i)),
				body:      c.fun.NewBlock(fmt.Sprintf("match.body.%d", i)),
			}
		}
	}

	var bExit *ir.Block
	if c.CanFallthrough(m) {
		bExit = c.fun.NewBlock("match.exit")
	}

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
			bNext := blocks[i+1].condition

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
	if bExit != nil {
		c.Emitter.Begin(bExit)
	}

	return 0
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

func (c *Codegen) VisitWhile(w *ast.While) int {
	// Handle literal ast.Bool condition
	literal, isLiteral := w.Condition.(*ast.Bool)

	if isLiteral && !literal.Value {
		return 0
	}

	// Create blocks
	bCondition := c.fun.NewBlock("while.condition")
	bBody := c.fun.NewBlock("while.body")

	var bExit *ir.Block
	if c.CanFallthrough(w) {
		bExit = c.fun.NewBlock("while.exit")
	}

	prevBLoopBreak := c.bLoopBreak
	prevBLoopContinue := c.bLoopContinue
	defer func() {
		c.bLoopBreak = prevBLoopBreak
		c.bLoopContinue = prevBLoopContinue
	}()

	c.bLoopBreak = bExit
	c.bLoopContinue = bCondition

	// Condition
	c.Emitter.Br(bCondition)
	c.Emitter.Begin(bCondition)

	if isLiteral {
		c.Emitter.Br(bBody)
	} else {
		condition := c.LoadImplicitCast(w.Condition, types.PrimitiveBool)
		c.Emitter.BrCond(condition, bBody, bExit)
	}

	// Body
	c.Emitter.Begin(bBody)
	c.GenerateStmt(w.Body)
	c.Emitter.Br(bCondition)

	// Exit
	if bExit != nil {
		c.Emitter.Begin(bExit)
	}

	return 0
}

func (c *Codegen) VisitFor(f *ast.For) int {
	// Scope
	c.Emitter.PushScope(c.emitMetaScope(f))
	c.scope.Push()

	defer func() {
		c.scope.Pop()
		c.Emitter.PopScope()
	}()

	// Initializer
	if !core.IsNil(f.Initializer) {
		c.GenerateStmt(f.Initializer)

		if !c.CanFallthrough(f.Initializer) {
			return 0
		}
	}

	// Handle literal ast.Bool condition
	condition := f.Condition

	if literal, ok := condition.(*ast.Bool); ok {
		if !literal.Value {
			return 0
		}

		condition = nil
	}

	// Create blocks
	var bCondition *ir.Block
	if !core.IsNil(condition) {
		bCondition = c.fun.NewBlock("for.condition")
	}

	bBody := c.fun.NewBlock("for.body")

	reachesIncrement := c.CanFallthrough(f.Body) || c.Sema.StmtExits[f.Body]&sema.ExitContinue != 0

	var bIncrement *ir.Block
	if !core.IsNil(f.Increment) && reachesIncrement {
		bIncrement = c.fun.NewBlock("for.increment")
	}

	var bExit *ir.Block
	if c.CanFallthrough(f) {
		bExit = c.fun.NewBlock("for.exit")
	}

	prevBLoopBreak := c.bLoopBreak
	prevBLoopContinue := c.bLoopContinue

	defer func() {
		c.bLoopBreak = prevBLoopBreak
		c.bLoopContinue = prevBLoopContinue
	}()

	bNext := bBody
	if bCondition != nil {
		bNext = bCondition
	}

	c.bLoopBreak = bExit
	c.bLoopContinue = bNext
	if bIncrement != nil {
		c.bLoopContinue = bIncrement
	}

	// Condition
	c.Emitter.Br(bNext)

	if bCondition != nil {
		c.Emitter.Begin(bCondition)
		value := c.LoadImplicitCast(condition, types.PrimitiveBool)
		c.Emitter.BrCond(value, bBody, bExit)
	}

	// Body
	c.Emitter.Begin(bBody)
	c.GenerateStmt(f.Body)
	c.Emitter.Br(c.bLoopContinue)

	// Increment
	if bIncrement != nil {
		c.Emitter.Begin(bIncrement)
		c.GenerateExpr(f.Increment)
		c.Emitter.Br(bNext)
	}

	// Exit
	if bExit != nil {
		c.Emitter.Begin(bExit)
	}

	return 0
}

func (c *Codegen) VisitReturn(r *ast.Return) int {
	var value ir.Value

	if !core.IsNil(r.Value) {
		if c.ExprType(r.Value) == types.NeverType {
			c.GenerateExpr(r.Value)
			return 0
		}

		value = c.LoadImplicitCast(r.Value, c.ResolveType(c.funcTyp.Returns))
	}

	c.ReturnValue(value)

	return 0
}

func (c *Codegen) VisitBreak(_ *ast.Break) int {
	c.Emitter.Br(c.bLoopBreak)

	return 0
}

func (c *Codegen) VisitContinue(_ *ast.Continue) int {
	c.Emitter.Br(c.bLoopContinue)

	return 0
}

func (c *Codegen) VisitBadStmt(_ *ast.BadStmt) int {
	return 0
}

// Utils

func (c *Codegen) CanFallthrough(stmt ast.Stmt) bool {
	if core.IsNil(stmt) {
		return true
	}

	exits, ok := c.Sema.StmtExits[stmt]
	if !ok {
		panic("codegen.Codegen.CanFallthrough - Statement doesn't have flow analysis results")
	}

	return exits&sema.ExitFallthrough != 0
}

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
