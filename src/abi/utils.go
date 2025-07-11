package abi

import "fireball/ast"

func alignTo(num, align uint32) uint32 {
	if num%align != 0 {
		num += align - (num % align)
	}

	return num
}

// Aggregate type flattening

type field struct {
	offset uint32
	size   uint32
	class  Class
}

func flatten(arch Arch, callConv CallConv, type_ ast.Type, offset uint32, fields []field) []field {
	switch type_ := type_.(type) {
	case *ast.PrimitiveType, *ast.PointerType, ast.FuncType:
		regs := callConv.Classify(type_)

		if len(regs) != 1 {
			panic("abi.flatten() - Classification of a simple type returned more than one register")
		}

		fields = append(fields, field{
			offset: offset,
			size:   regs[0].Size,
			class:  regs[0].Class,
		})

	case *ast.ArrayType:
		size, align := TypeInfo(arch, type_.Element)

		for i := uint32(0); i < type_.Count; i++ {
			offset = alignTo(offset, align)
			fields = flatten(arch, callConv, type_.Element, offset, fields)
			offset += size
		}

	case *ast.DeclType:
		switch decl := type_.Decl.(type) {
		case *ast.Struct:
			layout := StructLayout{Arch: arch}

			for _, field := range decl.Fields {
				relativeOffset := layout.Field(field.Type)
				fields = flatten(arch, callConv, field.Type, offset+relativeOffset, fields)
			}

		case *ast.Enum:
			fields = flatten(arch, callConv, decl.ActualType, offset, fields)

		default:
			panic("abi.flatten() - Invalid DeclType declaration")
		}

	default:
		panic("abi.flatten() - Invalid type")
	}

	return fields
}
