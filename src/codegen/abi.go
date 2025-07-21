package codegen

import (
	"fireball/abi"
	"fireball/analyzer"
	"fireball/ast"
	"fireball/ir"
	"fireball/lexer"
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
		_, isInterface := type_.Decl.(*ast.Interface)

		return isStruct || isInterface

	case *ast.ArrayType:
		return true

	default:
		return false
	}
}

func (c *codegen) visitLoadClassified(expr ast.Expr, to ast.Type, memoryClassPtrGetter func() ir.Value, alwaysStoreToPtr bool) ir.Value {
	value := c.visit(expr)

	type_ := expr.Result().Type
	exprKind := expr.Result().Kind

	if ast.IsValid(to) {
		if kind, ok := analyzer.GetImplicitCastKind(nil, expr.Result().Type, to); ok && kind != analyzer.Nop {
			value = c.load(expr, value)
			value = c.cast(value, expr.Result().Type, to, kind)

			type_ = to
			exprKind = ast.Value
		}
	}

	regs := c.callConv.Classify(type_)

	if len(regs) == 1 && regs[0].Class == abi.Memory {
		if alwaysStoreToPtr || exprKind == ast.Value {
			if exprKind == ast.Address {
				value = c.emitter.Load(c.types.Get(expr.Result().Type), value)
			}

			ptr := memoryClassPtrGetter()
			c.emitter.Store(value, ptr)

			value = ptr
		}

		return value
	}

	astType := getClassifiedAstType(c.callConv, type_)
	typ := getClassifiedIrType(&c.types, type_, false)

	if isPointerType(type_) {
		value = c.load(expr, value)
		return c.emitter.PtrToInt(value, typ)
	}

	if isAggregateType(type_) {
		if exprKind == ast.Value {
			typ := getClassifiedIrType(&c.types, type_, true)
			ptr := c.emitAlloca("abi.arg", typ)

			c.emitter.Store(value, ptr)
			value = ptr
		}

		return c.emitter.Load(typ, value)
	}

	value = c.load(expr, value)

	kind, ok := analyzer.GetCastKind(nil, type_, astType, true)
	if !ok {
		panic("codegen.codegen.visitLoadClassified() - Invalid cast kind")
	}

	return c.cast(value, type_, astType, kind)
}

func (c *codegen) declassify(call *ast.Call, type_ ast.Type, value ir.Value) ir.Value {
	regs := c.callConv.Classify(type_)

	if len(regs) == 1 && regs[0].Class == abi.Memory {
		return c.emitter.Load(c.types.Get(type_), value)
	}

	if isPointerType(type_) {
		return c.emitter.IntToPtr(value, c.types.Get(type_))
	}

	if isAggregateType(type_) {
		typ := c.types.Get(call.Callee.Result().Type.(ast.FuncType).ReturnType())
		ptr := c.emitAlloca("abi.call.declassify", typ)

		c.emitter.Store(value, ptr)
		return c.emitter.Load(c.types.Get(type_), ptr)
	}

	astType := getClassifiedAstType(c.callConv, type_)

	kind, ok := analyzer.GetCastKind(nil, astType, type_, true)
	if !ok {
		panic("codegen.codegen.declassify() - Invalid cast kind")
	}

	return c.cast(value, astType, type_, kind)
}

func getClassifiedAstType(callConv abi.CallConv, type_ ast.Type) ast.Type {
	if p, ok := type_.(*ast.PrimitiveType); ok && p.Kind == ast.Void {
		return ast.VoidType
	}

	regs := callConv.Classify(type_)

	if len(regs) == 1 {
		if regs[0].Class == abi.Memory {
			return &ast.PointerType{Pointee: type_}
		}

		t, _ := getTypeFromReg(regs[0])
		return t
	}

	var name strings.Builder
	var fields []*ast.Field

	name.WriteString("__abi_struct")

	for i, reg := range regs {
		type_, _ := getTypeFromReg(reg)

		name.WriteRune('_')
		name.WriteString(type_.String())

		fields = append(fields, &ast.Field{
			Name: &ast.Leaf{Token: lexer.Token{Kind: lexer.Identifier, Text: "reg" + strconv.Itoa(i)}},
			Type: type_,
		})
	}

	nameLeaf := &ast.Leaf{Token: lexer.Token{Kind: lexer.Identifier, Text: name.String()}}

	return &ast.DeclType{
		Path: &ast.Path{Segments: []*ast.Leaf{nameLeaf}},
		Decl: &ast.Struct{NameN: nameLeaf, Fields: fields},
	}
}

func getClassifiedIrType(types *TypeCache, type_ ast.Type, skipPointer bool) ir.Type {
	if p, ok := type_.(*ast.PrimitiveType); ok && p.Kind == ast.Void {
		return ir.Void
	}

	regs := types.CallConv.Classify(type_)

	if len(regs) == 1 {
		if regs[0].Class == abi.Memory {
			if skipPointer {
				return types.Get(type_)
			}

			return ir.Pointer
		}

		_, t := getTypeFromReg(regs[0])
		return t
	}

	var fields []ir.Type
	size := uint32(0)
	align := uint32(0)

	for _, reg := range regs {
		_, t := getTypeFromReg(reg)
		fields = append(fields, t)

		size += reg.Size
		align = max(align, reg.Size)
	}

	return &ir.StructType{
		Packed: false,
		Fields: fields,
	}
}

func getTypeFromReg(reg abi.Reg) (ast.Type, ir.Type) {
	switch reg.Class {
	case abi.Integer:
		if reg.Size <= 1 {
			return ast.I8Type, ir.I8
		}
		if reg.Size <= 2 {
			return ast.I16Type, ir.I16
		}
		if reg.Size <= 4 {
			return ast.I32Type, ir.I32
		}
		return ast.I64Type, ir.I64

	case abi.SSE:
		if reg.Size <= 4 {
			return ast.F32Type, ir.Float
		}
		return ast.F64Type, ir.Double

	default:
		panic("codegen.getTypeFromReg() - Invalid register class")
	}
}
