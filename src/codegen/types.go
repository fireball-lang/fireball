package codegen

import (
	"fireball/abi"
	"fireball/ast"
	"fireball/ir"
)

type TypeCache struct {
	Arch     abi.Arch
	CallConv abi.CallConv
	Module   *ir.Module

	types []typeMapping
	metas []metaMapping
}

type typeMapping struct {
	astType ast.Type
	irType  ir.Type
}

type metaMapping struct {
	astType ast.Type
	ref     ir.MetaRef
}

func (t *TypeCache) Get(type_ ast.Type) ir.Type {
	for _, mapping := range t.types {
		if mapping.astType.Equals(type_) {
			return mapping.irType
		}
	}

	var itType ir.Type

	switch type_ := type_.(type) {
	case *ast.PrimitiveType:
		itType = t.createPrimitiveType(type_)
	case *ast.DeclType:
		itType = t.createDeclType(type_)
	case *ast.ArrayType:
		itType = t.createArrayType(type_)
	case *ast.PointerType:
		itType = t.createPointerType(type_)
	case ast.FuncType:
		itType = t.createFuncType(type_)

	default:
		panic("codegen.TypeCache.Get() - Invalid type")
	}

	t.types = append(t.types, typeMapping{
		astType: type_,
		irType:  itType,
	})

	return itType
}

func (t *TypeCache) GetMeta(type_ ast.Type) ir.MetaRef {
	for _, mapping := range t.metas {
		if mapping.astType.Equals(type_) {
			return mapping.ref
		}
	}

	var ref ir.MetaRef

	switch type_ := type_.(type) {
	case *ast.PrimitiveType:
		ref = t.createPrimitiveMeta(type_)
	case *ast.DeclType:
		ref = t.createDeclMeta(type_)
	case *ast.ArrayType:
		ref = t.createArrayMeta(type_)
	case *ast.PointerType:
		ref = t.createPointerMeta(type_)
	case ast.FuncType:
		ref = t.createFuncMeta(type_)

	default:
		panic("codegen.TypeCache.Get() - Invalid type")
	}

	t.metas = append(t.metas, metaMapping{
		astType: type_,
		ref:     ref,
	})

	return ref
}

// ast.PrimitiveType

func (t *TypeCache) createPrimitiveType(type_ *ast.PrimitiveType) ir.Type {
	switch type_.Kind {
	case ast.Void:
		return ir.Void
	case ast.Bool:
		return ir.I1
	case ast.U8, ast.I8:
		return ir.I8
	case ast.U16, ast.I16:
		return ir.I16
	case ast.U32, ast.I32:
		return ir.I32
	case ast.U64, ast.I64:
		return ir.I64
	case ast.F32:
		return ir.Float
	case ast.F64:
		return ir.Double

	default:
		panic("codegen.TypeCache.createPrimitiveType() - Invalid primitive kind")
	}
}

func (t *TypeCache) createPrimitiveMeta(type_ *ast.PrimitiveType) ir.MetaRef {
	if type_.Kind == ast.Void {
		return ir.MetaRef(0)
	}

	size, align := abi.TypeInfo(t.Arch, type_)
	var encoding ir.MetaBasicTypeEncoding

	switch type_.Kind {
	case ast.Bool:
		encoding = ir.MetaBoolean
	case ast.U8:
		encoding = ir.MetaUnsignedChar
	case ast.U16, ast.U32, ast.U64:
		encoding = ir.MetaUnsigned
	case ast.I8:
		encoding = ir.MetaSignedChar
	case ast.I16, ast.I32, ast.I64:
		encoding = ir.MetaSigned
	case ast.F32, ast.F64:
		encoding = ir.MetaFloat

	default:
		panic("codegen.TypeCache.createPrimitiveMeta() - Invalid primitive kind")
	}

	return t.Module.AddMeta(&ir.BasicTypeMeta{
		Name:     type_.String(),
		Encoding: encoding,
		Size:     size * 8,
		Align:    align * 8,
	})
}

// ast.DeclType

func (t *TypeCache) createDeclType(type_ *ast.DeclType) ir.Type {
	switch decl := type_.Decl.(type) {
	case *ast.Struct:
		fields := make([]ir.Type, len(decl.Fields))

		for i, field := range decl.Fields {
			fields[i] = t.Get(field.Type)
		}

		return t.Module.NamedStruct(GetDeclLinkName(decl), ir.StructType{
			Packed: false,
			Fields: fields,
		})

	case *ast.Enum:
		return t.Get(decl.ActualType)

	default:
		panic("codegen.TypeCache.createDeclType() - Invalid declaration")
	}
}

func (t *TypeCache) createDeclMeta(type_ *ast.DeclType) ir.MetaRef {
	switch decl := type_.Decl.(type) {
	case *ast.Struct:
		layout := abi.StructLayout{Arch: t.Arch}
		fields := make([]ir.MetaRef, len(decl.Fields))

		for i, field := range decl.Fields {
			size, align := abi.TypeInfo(t.Arch, field.Type)

			fields[i] = t.Module.AddMeta(&ir.DerivedTypeMeta{
				Kind:   ir.MetaMember,
				Base:   t.GetMeta(field.Type),
				Offset: layout.Field(field.Type),
				Size:   size * 8,
				Align:  align * 8,
			})
		}

		size, align := abi.TypeInfo(t.Arch, type_)

		return t.Module.AddMeta(&ir.CompositeTypeMeta{
			Name:     GetDeclLinkName(decl),
			Kind:     ir.MetaStructureType,
			Elements: fields,
			File:     0,
			Line:     type_.Range().Start.Line,
			Size:     size * 8,
			Align:    align * 8,
		})

	case *ast.Enum:
		size, align := abi.TypeInfo(t.Arch, type_)
		cases := make([]ir.MetaRef, len(decl.Cases))

		for i, c := range decl.Cases {
			cases[i] = t.Module.AddMeta(&ir.EnumeratorMeta{
				Name:  c.Name.Token.Text,
				Value: c.ActualValue,
			})
		}

		return t.Module.AddMeta(&ir.CompositeTypeMeta{
			Name:     GetDeclLinkName(decl),
			Kind:     ir.MetaEnumerationType,
			Elements: cases,
			File:     0,
			Line:     type_.Range().Start.Line,
			Size:     size * 8,
			Align:    align * 8,
		})

	default:
		panic("codegen.TypeCache.createDeclMeta() - Invalid declaration")
	}
}

// ast.ArrayType

func (t *TypeCache) createArrayType(type_ *ast.ArrayType) ir.Type {
	return &ir.ArrayType{
		Length:  type_.Count,
		Element: t.Get(type_.Element),
	}
}

func (t *TypeCache) createArrayMeta(type_ *ast.ArrayType) ir.MetaRef {
	size, align := abi.TypeInfo(t.Arch, type_)

	subrange := t.Module.AddMeta(&ir.SubrangeMeta{
		Count: type_.Count,
	})

	return t.Module.AddMeta(&ir.CompositeTypeMeta{
		Name:     type_.String(),
		Kind:     ir.MetaArrayType,
		BaseType: t.GetMeta(type_.Element),
		Elements: []ir.MetaRef{subrange},
		File:     0,
		Line:     type_.Range().Start.Line,
		Size:     size * 8,
		Align:    align * 8,
	})
}

// ast.PointerType

func (t *TypeCache) createPointerType(type_ *ast.PointerType) ir.Type {
	return ir.Pointer
}

func (t *TypeCache) createPointerMeta(type_ *ast.PointerType) ir.MetaRef {
	size, align := abi.TypeInfo(t.Arch, type_)

	return t.Module.AddMeta(&ir.DerivedTypeMeta{
		Kind:  ir.MetaPointerType,
		Base:  t.GetMeta(type_.Pointee),
		Size:  size * 8,
		Align: align * 8,
	})
}

// ast.FuncType

func (t *TypeCache) createFuncType(type_ ast.FuncType) ir.Type {
	returns := getClassifiedIrType(t, type_.ReturnType(), false)
	var params []ir.Type

	regs := t.CallConv.Classify(type_.ReturnType())

	if len(regs) == 1 && regs[0].Class == abi.Memory {
		params = append(params, returns)
		returns = ir.Void
	}

	if impl, ok := type_.Parent().(*ast.Impl); ok {
		type_ := t.Get(ast.GetDeclPointerType(impl.Decl))

		params = append(params, type_)
	}

	for param := range type_.ParamTypes() {
		params = append(params, getClassifiedIrType(t, param, false))
	}

	return &ir.FunctionType{
		Returns: returns,
		Params:  params,
		VarArgs: type_.VarArgs(),
	}
}

func (t *TypeCache) createFuncMeta(type_ ast.FuncType) ir.MetaRef {
	// Function type

	returns := t.GetMeta(type_.ReturnType())
	params := make([]ir.MetaRef, 0, type_.ParamTypeCount())

	for param := range type_.ParamTypes() {
		params = append(params, t.GetMeta(param))
	}

	ref := t.Module.AddMeta(&ir.SubroutineTypeMeta{
		Returns: returns,
		Params:  params,
	})

	// Pointer type

	return t.Module.AddMeta(&ir.DerivedTypeMeta{
		Kind:  ir.MetaPointerType,
		Base:  ref,
		Size:  t.Arch.WordSize * 8,
		Align: t.Arch.WordSize * 8,
	})
}
