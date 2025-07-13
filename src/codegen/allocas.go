package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/llvm"
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

	c.setSourceLocation(v)
	name := c.getNamedIdentifierString("var." + v.Name.Token.Text)

	_, align := abi.TypeInfo(c.arch, type_)
	c.allocas[v] = llvm.Alloca(c.fun, c.types.Get(type_), 1, align, name)
}

func (c *codegen) collectAllocasCall(call *ast.Call) {
	f, _ := call.Callee.Result().Type.(ast.FuncType)
	regs := c.callConv.Classify(f.ReturnType())

	if (len(regs) == 1 && regs[0].Class == abi.Memory) || isAggregateType(f.ReturnType()) {
		t := c.types.Get(f.ReturnType())
		c.allocas[call] = llvm.Alloca(c.fun, t, 1, t.Align()/8, c.getNamedIdentifierString("abi.call"))
	}

	if _, ok := f.Parent().(*ast.Impl); ok {
		expr := call.Callee.(*ast.Member).Value

		if expr.Result().Kind == ast.Value {
			t := c.types.Get(expr.Result().Type)
			c.allocas2[call] = llvm.Alloca(c.fun, t, 1, t.Align()/8, c.getNamedIdentifierString("call.this"))
		}
	}

	for _, arg := range call.Args {
		c.collectAllocasArg(arg)
	}
}
