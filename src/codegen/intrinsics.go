package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/ir"
	"fireball/types"
	"fmt"
)

func (c *codegen) EmitIntrinsic(kind ast.IntrinsicKind, typ *types.Func, args []ast.Expr) ir.Value {
	switch kind {
	case ast.Syscall:
		values := c.PrepareIntrinsicArgs(typ, args, 0)

		sig := &ir.Signature{
			Returns: ir.I64,
			Params:  make([]ir.Type, len(args)),
		}

		asm := &ir.Assembly{
			SideEffect:  true,
			Constraints: make([]string, 0, 1+len(args)+3),
			Template:    c.GetSyscallInstruction(),
		}

		asm.Constraints = append(asm.Constraints, fmt.Sprintf("={%s}", c.GetSyscallReturnRegister()))

		for i := range args {
			sig.Params[i] = ir.I64
			asm.Constraints = append(asm.Constraints, fmt.Sprintf("{%s}", c.GetSyscallRegister(i)))
		}

		if c.arch == abi.AMD64 {
			asm.Constraints = append(asm.Constraints, "~{rcx}", "~{r11}")
		}

		asm.Constraints = append(asm.Constraints, "~{memory}")

		return c.emitter.Call(sig, asm, values)

	case ast.Memcpy:
		fun := c.GetIntrinsicFunction(ir.Memcpy)

		values := c.PrepareIntrinsicArgs(typ, args, 1)
		values = append(values, ir.False)

		return c.emitter.Call(fun.Signature, fun, values)

	case ast.Memmove:
		fun := c.GetIntrinsicFunction(ir.Memmove)

		values := c.PrepareIntrinsicArgs(typ, args, 1)
		values = append(values, ir.False)

		return c.emitter.Call(fun.Signature, fun, values)

	case ast.Memset:
		fun := c.GetIntrinsicFunction(ir.Memset)

		values := c.PrepareIntrinsicArgs(typ, args, 1)
		values = append(values, ir.False)

		return c.emitter.Call(fun.Signature, fun, values)

	default:
		panic("codegen.codegen.EmitIntrinsic() - Invalid intrinsic")
	}
}

func (c *codegen) GetSyscallInstruction() string {
	// amd64
	if c.arch == abi.AMD64 {
		return "syscall"
	}

	// arm64
	if c.arch == abi.ARM64 {
		return "svc #0"
	}

	// <invalid>
	panic("codegen.codegen.GetSyscallInstruction() - Invalid ABI")
}

func (c *codegen) GetSyscallReturnRegister() string {
	// amd64
	if c.arch == abi.AMD64 {
		return "rax"
	}

	// arm64
	if c.arch == abi.ARM64 {
		return "x0"
	}

	// <invalid>
	panic("codegen.codegen.GetSyscallReturnRegister() - Invalid ABI")
}

func (c *codegen) GetSyscallRegister(i int) string {
	// amd64
	if c.arch == abi.AMD64 {
		switch i {
		case 0:
			return "rax"
		case 1:
			return "rdi"
		case 2:
			return "rsi"
		case 3:
			return "rdx"
		case 4:
			return "r10"
		case 5:
			return "r8"
		case 6:
			return "r9"

		default:
			panic("codegen.codegen.GetSyscallRegister() - Too many registers")
		}
	}

	// arm64
	if c.arch == abi.ARM64 {
		switch i {
		case 0:
			return "x8"
		case 1:
			return "x0"
		case 2:
			return "x1"
		case 3:
			return "x2"
		case 4:
			return "x3"
		case 5:
			return "x4"
		case 6:
			return "x5"

		default:
			panic("codegen.codegen.GetSyscallRegister() - Too many registers")
		}
	}

	// <invalid>
	panic("codegen.codegen.GetSyscallRegister() - Invalid ABI")
}

func (c *codegen) PrepareIntrinsicArgs(typ *types.Func, args []ast.Expr, additional int) []ir.Value {
	values := make([]ir.Value, 0, len(args)+additional)

	for i, arg := range args {
		values = append(values, c.LoadImplicitCast(arg, typ.Params[i]))
	}

	return values
}

func (c *codegen) GetIntrinsicFunction(intrinsic ir.Intrinsic) *ir.Function {
	// Check already existing functions
	for fun := range c.module.Functions() {
		if fun.Name == intrinsic.Name {
			return fun
		}
	}

	// Create extern function
	fun := c.module.NewFunction(intrinsic.Name, intrinsic.Signature, intrinsic.ParamNames)
	fun.Flags = ir.Declare

	return fun
}
