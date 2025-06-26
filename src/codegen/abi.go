package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/lexer"
	"fireball/llvm"
	"strconv"
	"strings"
)

func isPointerType(type_ ast.Type) bool {
	_, isPtrType := type_.(*ast.PointerType)
	_, isFuncType := type_.(ast.FuncType)

	return isPtrType || isFuncType
}

func isAggregateType(type_ ast.Type) bool {
	switch type_ := type_.(type) {
	case *ast.DeclType:
		_, isStruct := type_.Decl.(*ast.Struct)
		return isStruct

	case *ast.ArrayType:
		return true

	default:
		return false
	}
}

func (c *codegen) collectAllocasArg(arg ast.Expr) {
	if arg.Result().Kind == ast.Value {
		regs := c.callConv.Classify(arg.Result().Type)

		if (len(regs) == 1 && regs[0].Class == abi.Memory) || isAggregateType(arg.Result().Type) {
			astType := getClassifiedType(c.callConv, arg.Result().Type)
			if p, ok := astType.(*ast.PointerType); ok {
				astType = p.Pointee
			}

			type_ := c.types.Get(astType)
			name := c.getNamedIdentifierString("abi.arg")

			c.allocas[arg] = llvm.Alloca(c.fun, type_, 1, type_.Align()/8, name)
		}
	}
}

func (c *codegen) collectAllocasReturn(r *ast.Return) {
	if ast.IsValid(r.Value) && r.Value.Result().Kind == ast.Value && isAggregateType(r.Value.Result().Type) {
		regs := c.callConv.Classify(r.Value.Result().Type)

		if len(regs) != 1 || regs[0].Class != abi.Memory {
			astType := getClassifiedType(c.callConv, r.Value.Result().Type)
			if p, ok := astType.(*ast.PointerType); ok {
				astType = p.Pointee
			}

			type_ := c.types.Get(astType)
			name := c.getNamedIdentifierString("abi.return")

			c.allocas[r.Value] = llvm.Alloca(c.fun, type_, 1, type_.Align()/8, name)
		}
	}
}

func (c *codegen) visitLoadClassified(expr ast.Expr, memoryClassPtrGetter func() llvm.Value) llvm.Value {
	value := c.visit(expr)
	regs := c.callConv.Classify(expr.Result().Type)

	if len(regs) == 1 && regs[0].Class == abi.Memory {
		if expr.Result().Kind == ast.Value {
			ptr := memoryClassPtrGetter()
			llvm.Store(c.fun, value, ptr)

			value = ptr
		}

		return value
	}

	astType := getClassifiedType(c.callConv, expr.Result().Type)
	llvmType := c.types.Get(astType)

	if isPointerType(expr.Result().Type) {
		value = c.load(expr, value)
		return llvm.PtrToInt(c.fun, value, llvmType, "")
	}

	if isAggregateType(expr.Result().Type) {
		if expr.Result().Kind == ast.Value {
			ptr := c.allocas[expr]
			llvm.Store(c.fun, value, ptr)

			value = ptr
		}

		return llvm.LoadAs(c.fun, value, llvmType, "")
	}

	value = c.load(expr, value)
	return c.cast(value, expr.Result().Type, astType, true)
}

func (c *codegen) declassify(call *ast.Call, type_ ast.Type, value llvm.Value) llvm.Value {
	regs := c.callConv.Classify(type_)

	if len(regs) == 1 && regs[0].Class == abi.Memory {
		return llvm.LoadAs(c.fun, value, c.types.Get(type_), "")
	}

	if isPointerType(type_) {
		return llvm.IntToPtr(c.fun, value, c.types.Get(type_), "")
	}

	if isAggregateType(type_) {
		ptr := c.allocas[call]
		llvm.Store(c.fun, value, ptr)

		return llvm.LoadAs(c.fun, ptr, c.types.Get(type_), "")
	}

	astType := getClassifiedType(c.callConv, type_)
	return c.cast(value, astType, type_, true)
}

func getClassifiedType(callConv abi.CallConv, type_ ast.Type) ast.Type {
	if p, ok := type_.(*ast.PrimitiveType); ok && p.Kind == ast.Void {
		return ast.VoidType
	}

	regs := callConv.Classify(type_)

	if len(regs) == 1 {
		if regs[0].Class == abi.Memory {
			return &ast.PointerType{Pointee: type_}
		}

		return getTypeFromReg(regs[0])
	}

	var name strings.Builder
	var fields []*ast.Field

	name.WriteString("__abi_struct")

	for i, reg := range regs {
		type_ := getTypeFromReg(reg)

		name.WriteRune('_')
		name.WriteString(type_.String())

		fields = append(fields, &ast.Field{
			Name: &ast.Leaf{Token: lexer.Token{Kind: lexer.Identifier, Text: "reg" + strconv.Itoa(i)}},
			Type: type_,
		})
	}

	nameLeaf := &ast.Leaf{Token: lexer.Token{Kind: lexer.Identifier, Text: name.String()}}

	return &ast.DeclType{
		Name: nameLeaf,
		Decl: &ast.Struct{NameN: nameLeaf, Fields: fields},
	}
}

func getTypeFromReg(reg abi.Reg) ast.Type {
	switch reg.Class {
	case abi.Integer:
		if reg.Size <= 1 {
			return ast.I8Type
		}
		if reg.Size <= 2 {
			return ast.I16Type
		}
		if reg.Size <= 4 {
			return ast.I32Type
		}
		return ast.I64Type

	case abi.SSE:
		if reg.Size <= 4 {
			return ast.F32Type
		}
		return ast.F64Type

	default:
		panic("codegen.getTypeFromReg() - Invalid register class")
	}
}
