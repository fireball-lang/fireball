package abi

import "fireball/ast"

func TypeInfo(arch Arch, type_ ast.Type) (uint32, uint32) {
	switch type_ := type_.(type) {
	case *ast.PrimitiveType:
		size := alignTo(type_.Kind.BitCount(), 8) / 8
		return size, size

	case *ast.ArrayType:
		size, align := TypeInfo(arch, type_.Element)
		return size * type_.Count, align

	case *ast.PointerType, ast.FuncType:
		return arch.WordSize, arch.WordSize

	case *ast.DeclType:
		s := type_.Decl.(*ast.Struct)
		return getStructInfo(arch, s)

	default:
		panic("abi.TypeInfo() - Invalid type")
	}
}

func getStructInfo(arch Arch, s *ast.Struct) (uint32, uint32) {
	layout := StructLayout{Arch: arch}

	for _, field := range s.Fields {
		layout.Field(field.Type)
	}

	return layout.Info()
}
