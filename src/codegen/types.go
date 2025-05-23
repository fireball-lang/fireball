package codegen

import (
	"fireball/ast"
	"fireball/llvm"
)

type typeMapping struct {
	astType  ast.Type
	llvmType llvm.Type
}

func (c *codegen) getType(type_ ast.Type) llvm.Type {
	for _, mapping := range c.types {
		if mapping.astType.Equals(type_) {
			return mapping.llvmType
		}
	}

	var t llvm.Type

	switch type_ := type_.(type) {
	case *ast.PrimitiveType:
		t = c.createPrimitiveType(type_)
	case *ast.PointerType:
		t = c.createPointerType(type_)
	case *ast.DeclType:
		t = c.createDeclType(type_)
	case ast.FuncType:
		t = c.createFuncType(type_)

	default:
		panic("codegen.codegen.getType() - Invalid type")
	}

	c.types = append(c.types, typeMapping{
		astType:  type_,
		llvmType: t,
	})

	return t
}

func (c *codegen) createPrimitiveType(type_ *ast.PrimitiveType) llvm.Type {
	switch type_.Kind {
	case ast.Void:
		return c.module.NewVoidType()
	case ast.Bool:
		return c.module.NewIntegerType(false, 1)
	case ast.U8:
		return c.module.NewIntegerType(false, 8)
	case ast.U16:
		return c.module.NewIntegerType(false, 16)
	case ast.U32:
		return c.module.NewIntegerType(false, 32)
	case ast.U64:
		return c.module.NewIntegerType(false, 64)
	case ast.I8:
		return c.module.NewIntegerType(true, 8)
	case ast.I16:
		return c.module.NewIntegerType(true, 16)
	case ast.I32:
		return c.module.NewIntegerType(true, 32)
	case ast.I64:
		return c.module.NewIntegerType(true, 64)
	case ast.F32:
		return c.module.NewFloatingType(false)
	case ast.F64:
		return c.module.NewFloatingType(true)

	default:
		panic("codegen.codegen.createPrimitiveType() - Invalid primitive kind")
	}
}

func (c *codegen) createPointerType(type_ *ast.PointerType) llvm.Type {
	return c.module.NewPointerType(c.getType(type_.Pointee))
}

func (c *codegen) createDeclType(type_ *ast.DeclType) llvm.Type {
	switch decl := type_.Decl.(type) {
	case *ast.Struct:
		fields := make([]llvm.Field, len(decl.Fields))
		size := uint32(0)
		align := uint32(0)

		for i, field := range decl.Fields {
			fields[i] = llvm.Field{
				Name: field.Name.Token.Text,
				Type: c.getType(field.Type),
			}
		}

		return c.module.NewStructType(type_.Name.Token.Text, fields, size, align)

	default:
		panic("codegen.codegen.createDeclType() - Invalid declaration")
	}
}

func (c *codegen) createFuncType(type_ ast.FuncType) llvm.Type {
	var params []llvm.Type

	for param := range type_.ParamTypes() {
		params = append(params, c.getType(param))
	}

	return c.module.NewFunctionType(c.getType(type_.ReturnType()), params, type_.VarArgs())
}
