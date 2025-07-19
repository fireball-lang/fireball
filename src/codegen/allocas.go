package codegen

import (
	"fireball/abi"
	"fireball/ast"
)

func (c *codegen) collectAllocas(expr ast.Expr) {
	switch expr := expr.(type) {
	case *ast.Var:
		c.collectAllocasVar(expr)
	case *ast.Call:
		c.collectAllocasCall(expr)
	case *ast.Return:
		c.collectAllocasReturn(expr)
	}

	for node := range expr.Children() {
		if expr, ok := node.(ast.Expr); ok {
			c.collectAllocas(expr)
		}
	}
}

func (c *codegen) collectAllocasVar(v *ast.Var) {
	type_ := v.Type

	if !ast.IsValid(type_) {
		type_ = v.Value.Result().Type
	}

	c.emitter.SetDebugLocation(v.Range().Start)

	alloca := c.emitter.Alloca(c.types.Get(type_), 1)
	alloca.SetName("var." + v.Name.Token.Text)

	c.allocas[v] = alloca
}

func (c *codegen) collectAllocasCall(call *ast.Call) {
	f, _ := call.Callee.Result().Type.(ast.FuncType)
	regs := c.callConv.Classify(f.ReturnType())

	if (len(regs) == 1 && regs[0].Class == abi.Memory) || isAggregateType(f.ReturnType()) {
		t := c.types.Get(f.ReturnType())

		alloca := c.emitter.Alloca(t, 1)
		alloca.SetName("abi.call")

		c.allocas[call] = alloca
	}

	if _, ok := f.Parent().(*ast.Impl); ok {
		expr := call.Callee.(*ast.Member).Value

		if expr.Result().Kind == ast.Value {
			t := c.types.Get(expr.Result().Type)

			alloca := c.emitter.Alloca(t, 1)
			alloca.SetName("abi.this")

			c.allocas2[call] = alloca
		}
	}

	for _, arg := range call.Args {
		c.collectAllocasArg(arg)
	}
}
