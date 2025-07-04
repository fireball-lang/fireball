package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/llvm"
	"strings"
)

type TypeCache struct {
	Arch     abi.Arch
	CallConv abi.CallConv
	Module   *llvm.Module

	types []typeMapping
}

type typeMapping struct {
	astType  ast.Type
	llvmType llvm.Type
}

func (t *TypeCache) Get(type_ ast.Type) llvm.Type {
	for _, mapping := range t.types {
		if mapping.astType.Equals(type_) {
			return mapping.llvmType
		}
	}

	var llvmType llvm.Type

	switch type_ := type_.(type) {
	case *ast.PrimitiveType:
		llvmType = t.createPrimitiveType(type_)
	case *ast.DeclType:
		llvmType = t.createDeclType(type_)
	case *ast.ArrayType:
		llvmType = t.createArrayType(type_)
	case *ast.PointerType:
		llvmType = t.createPointerType(type_)
	case ast.FuncType:
		llvmType = t.createFuncType(type_)

	default:
		panic("codegen.TypeCache.Get() - Invalid type")
	}

	t.types = append(t.types, typeMapping{
		astType:  type_,
		llvmType: llvmType,
	})

	return llvmType
}

func (t *TypeCache) createPrimitiveType(type_ *ast.PrimitiveType) llvm.Type {
	size, align := abi.TypeInfo(t.Arch, type_)

	switch type_.Kind {
	case ast.Void:
		return llvm.NewVoidType()
	case ast.Bool:
		return t.Module.NewIntegerType(size, align, false, 1)
	case ast.U8:
		return t.Module.NewIntegerType(size, align, false, 8)
	case ast.U16:
		return t.Module.NewIntegerType(size, align, false, 16)
	case ast.U32:
		return t.Module.NewIntegerType(size, align, false, 32)
	case ast.U64:
		return t.Module.NewIntegerType(size, align, false, 64)
	case ast.I8:
		return t.Module.NewIntegerType(size, align, true, 8)
	case ast.I16:
		return t.Module.NewIntegerType(size, align, true, 16)
	case ast.I32:
		return t.Module.NewIntegerType(size, align, true, 32)
	case ast.I64:
		return t.Module.NewIntegerType(size, align, true, 64)
	case ast.F32:
		return t.Module.NewFloatingType(size, align, false)
	case ast.F64:
		return t.Module.NewFloatingType(size, align, true)

	default:
		panic("codegen.TypeCache.createPrimitiveType() - Invalid primitive kind")
	}
}

func (t *TypeCache) createDeclType(type_ *ast.DeclType) llvm.Type {
	switch decl := type_.Decl.(type) {
	case *ast.Struct:
		layout := abi.StructLayout{Arch: t.Arch}
		fields := make([]llvm.Field, len(decl.Fields))

		for i, field := range decl.Fields {
			fields[i] = llvm.Field{
				Name:   field.Name.Token.Text,
				Type:   t.Get(field.Type),
				Offset: layout.Field(field.Type),
			}
		}

		modPath := ast.Root(decl).ModulePath()
		var name strings.Builder

		for i := 0; i < modPath.SegmentCount(); i++ {
			name.WriteString(modPath.SegmentAt(i))
			name.WriteRune('.') // TODO: replace with :
		}

		name.WriteString(decl.Name())

		size, align := layout.Info()
		return t.Module.NewStructType(name.String(), fields, size, align)

	default:
		panic("codegen.TypeCache.createDeclType() - Invalid declaration")
	}
}

func (t *TypeCache) createArrayType(type_ *ast.ArrayType) llvm.Type {
	size, align := abi.TypeInfo(t.Arch, type_)
	return t.Module.NewArrayType(size, align, type_.Count, t.Get(type_.Element))
}

func (t *TypeCache) createPointerType(type_ *ast.PointerType) llvm.Type {
	size, align := abi.TypeInfo(t.Arch, type_)
	return t.Module.NewPointerType(size, align, t.Get(type_.Pointee))
}

func (t *TypeCache) createFuncType(type_ ast.FuncType) llvm.Type {
	returns := getClassifiedLlvmType(t, type_.ReturnType(), false)
	debugReturns := t.Get(type_.ReturnType())

	var params []llvm.Type
	var debugParams []llvm.Type

	regs := t.CallConv.Classify(type_.ReturnType())

	if len(regs) == 1 && regs[0].Class == abi.Memory {
		params = append(params, returns)
		returns = llvm.NewVoidType()
	}

	if impl, ok := type_.Parent().(*ast.Impl); ok {
		type_ := t.Get(ast.GetStructPointerType(impl.Struct))

		params = append(params, type_)
		debugParams = append(debugParams, type_)
	}

	for param := range type_.ParamTypes() {
		params = append(params, getClassifiedLlvmType(t, param, false))
		debugParams = append(debugParams, t.Get(param))
	}

	size, align := abi.TypeInfo(t.Arch, type_)
	funcType := t.Module.NewFunctionType(size, align, returns, debugReturns, params, debugParams, type_.VarArgs())

	ptr := ast.PointerType{Pointee: type_}
	size, align = abi.TypeInfo(t.Arch, &ptr)

	return t.Module.NewPointerType(size, align, funcType)
}
