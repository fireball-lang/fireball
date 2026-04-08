package codegen

import (
	"fireball/abi"
	"fireball/ir"
	"fireball/types"
)

type typeEntry struct {
	Type   types.Type
	IrType ir.Type
}

type metaEntry struct {
	Type types.Type
	Ref  ir.MetaRef
}

type TypeCache struct {
	Arch abi.Arch

	Module  *ir.Module
	FileRef ir.MetaRef

	entries     []typeEntry
	metaEntries []metaEntry
}

func (t *TypeCache) Add(typ types.Type, irTyp ir.Type) ir.Type {
	t.entries = append(t.entries, typeEntry{typ, irTyp})
	return irTyp
}

func (t *TypeCache) AddMeta(typ types.Type, ref ir.MetaRef) ir.MetaRef {
	t.metaEntries = append(t.metaEntries, metaEntry{typ, ref})
	return ref
}

func (t *TypeCache) Get(typ types.Type) ir.Type {
	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	// Check cache
	for _, entry := range t.entries {
		if entry.Type.Equals(typ) {
			return entry.IrType
		}
	}

	// Create
	var irTyp ir.Type

	switch typ := typ.(type) {
	case *types.Primitive:
		irTyp = t.createPrimitiveType(typ)
	case *types.Pointer:
		irTyp = t.createPointerType(typ)
	case *types.Array:
		irTyp = t.createArrayType(typ)
	case *types.Struct:
		irTyp = t.createStructType(typ)

	default:
		panic("codegen.TypeCache.Get() - Invalid type")
	}

	return t.Add(typ, irTyp)
}

func (t *TypeCache) GetMeta(typ types.Type) ir.MetaRef {
	if t, ok := typ.(types.Composed); ok {
		if _, ok := typ.(*types.Func); !ok {
			typ = t.Underlying()
		}
	}

	// Check cache
	for _, entry := range t.metaEntries {
		if entry.Type.Equals(typ) {
			return entry.Ref
		}
	}

	// Create
	var ref ir.MetaRef

	switch typ := typ.(type) {
	case *types.Primitive:
		ref = t.createPrimitiveMeta(typ)
	case *types.Pointer:
		ref = t.createPointerMeta(typ)
	case *types.Array:
		ref = t.createArrayMeta(typ)
	case *types.Struct:
		ref = t.createStructMeta(typ)

	case *types.Func:
		ref = t.createFuncMeta(typ)

	default:
		panic("codegen.TypeCache.GetMeta() - Invalid type")
	}

	return t.AddMeta(typ, ref)
}

// types.Primitive

func (t *TypeCache) createPrimitiveType(typ *types.Primitive) ir.Type {
	switch typ.Kind {
	case types.Void:
		return ir.Void
	case types.Bool:
		return ir.I1

	case types.U8:
		return ir.I8
	case types.U16:
		return ir.I16
	case types.U32:
		return ir.I32
	case types.U64:
		return ir.I64

	case types.I8:
		return ir.I8
	case types.I16:
		return ir.I16
	case types.I32:
		return ir.I32
	case types.I64:
		return ir.I64

	case types.F32:
		return ir.Float
	case types.F64:
		return ir.Double

	default:
		panic("codegen.TypeCache.createPrimitiveType() - Invalid primitive kind")
	}
}

func (t *TypeCache) createPrimitiveMeta(typ *types.Primitive) ir.MetaRef {
	if typ.Kind == types.Void {
		return ir.MetaRef(0)
	}

	info := t.Arch.Info(typ)
	var encoding ir.MetaBasicTypeEncoding

	switch typ.Kind {
	case types.Bool:
		encoding = ir.MetaBoolean
	case types.U8:
		encoding = ir.MetaUnsignedChar
	case types.U16, types.U32, types.U64:
		encoding = ir.MetaUnsigned
	case types.I8:
		encoding = ir.MetaSignedChar
	case types.I16, types.I32, types.I64:
		encoding = ir.MetaSigned
	case types.F32, types.F64:
		encoding = ir.MetaFloat

	default:
		panic("codegen.TypeCache.createPrimitiveMeta() - Invalid primitive kind")
	}

	return t.Module.AddMeta(&ir.BasicTypeMeta{
		Name:     typ.String(),
		Encoding: encoding,
		Size:     info.Size * 8,
		Align:    info.Align * 8,
	})
}

// types.Pointer

func (t *TypeCache) createPointerType(_ *types.Pointer) ir.Type {
	return ir.Pointer
}

func (t *TypeCache) createPointerMeta(typ *types.Pointer) ir.MetaRef {
	info := t.Arch.Info(typ)

	return t.Module.AddMeta(&ir.DerivedTypeMeta{
		Name:  typ.String(),
		Kind:  ir.MetaPointerType,
		Base:  t.GetMeta(typ.Pointee),
		Size:  info.Size * 8,
		Align: info.Align * 8,
	})
}

// types.Array

func (t *TypeCache) createArrayType(typ *types.Array) ir.Type {
	return &ir.ArrayType{
		Length:  typ.Size,
		Element: t.Get(typ.Element),
	}
}

func (t *TypeCache) createArrayMeta(typ *types.Array) ir.MetaRef {
	info := t.Arch.Info(typ)
	subrange := t.Module.AddMeta(&ir.SubrangeMeta{Count: typ.Size})

	return t.Module.AddMeta(&ir.CompositeTypeMeta{
		Name:     typ.String(),
		Kind:     ir.MetaArrayType,
		BaseType: t.GetMeta(typ.Element),
		Elements: []ir.MetaRef{subrange},
		File:     t.FileRef,
		Size:     info.Size * 8,
		Align:    info.Align * 8,
	})
}

// types.Struct

func (t *TypeCache) createStructType(typ *types.Struct) ir.Type {
	fields := make([]ir.Type, len(typ.Fields))

	for i, field := range typ.Fields {
		fields[i] = t.Get(field.Type)
	}

	irTyp := ir.StructType{
		Packed: typ.Packed,
		Fields: fields,
	}

	return t.Module.NamedStruct(typ.Name, irTyp)
}

func (t *TypeCache) createStructMeta(typ *types.Struct) ir.MetaRef {
	info := t.Arch.Info(typ)
	fields := make([]ir.MetaRef, len(typ.Fields))

	for i, field := range typ.Fields {
		fieldInfo := t.Arch.Info(field.Type)

		fields[i] = t.Module.AddMeta(&ir.DerivedTypeMeta{
			Name:   field.Name,
			Kind:   ir.MetaMember,
			Base:   t.GetMeta(field.Type),
			Offset: info.Offsets[i] * 8,
			Size:   fieldInfo.Size * 8,
			Align:  fieldInfo.Align * 8,
		})
	}

	return t.Module.AddMeta(&ir.CompositeTypeMeta{
		Name:     typ.String(),
		Kind:     ir.MetaStructureType,
		Elements: fields,
		File:     t.FileRef,
		Size:     info.Size * 8,
		Align:    info.Align * 8,
	})
}

// types.Func

func (t *TypeCache) createFuncMeta(typ *types.Func) ir.MetaRef {
	params := make([]ir.MetaRef, len(typ.Params))

	for i, param := range typ.Params {
		params[i] = t.GetMeta(param)
	}

	return t.Module.AddMeta(&ir.SubroutineTypeMeta{
		Returns: t.GetMeta(typ.Returns),
		Params:  params,
	})
}
