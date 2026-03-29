package codegen

import (
	"fireball/ir"
	"fireball/types"
)

type typeEntry struct {
	Type   types.Type
	IrType ir.Type
}

type TypeCache struct {
	Module *ir.Module

	entries []typeEntry
}

func (t *TypeCache) Add(typ types.Type, irTyp ir.Type) ir.Type {
	t.entries = append(t.entries, typeEntry{typ, irTyp})
	return irTyp
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

func (t *TypeCache) createPointerType(_ *types.Pointer) ir.Type {
	return ir.Pointer
}

func (t *TypeCache) createArrayType(typ *types.Array) ir.Type {
	return &ir.ArrayType{
		Length:  typ.Size,
		Element: t.Get(typ.Element),
	}
}

func (t *TypeCache) createStructType(typ *types.Struct) ir.Type {
	fields := make([]ir.Type, len(typ.Fields))

	for i, field := range typ.Fields {
		fields[i] = t.Get(field.Type)
	}

	irTyp := ir.StructType{
		Packed: typ.Packed,
		Fields: fields,
	}

	return t.Module.NamedStruct("struct."+typ.Name, irTyp)
}
