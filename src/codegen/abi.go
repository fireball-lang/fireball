package codegen

import (
	"fireball/abi"
	"fireball/ir"
)

func getTypeForClasses(classes []abi.Class, size uint32) ir.Type {
	if len(classes) == 0 {
		return ir.Void
	}
	if len(classes) == 1 {
		return getTypeForClass(classes[0], size)
	}

	fields := make([]ir.Field, 0, len(classes))

	for _, class := range classes {
		fields = append(fields, ir.Field{Type: getTypeForClass(class, min(size, 8))})
		size -= min(size, 8)
	}

	if size != 0 {
		panic("codegen.getTypeForClasses() - Size not zero")
	}

	return &ir.StructType{
		Packed: true,
		Fields: fields,
	}
}

func getTypeForClass(class abi.Class, size uint32) ir.Type {
	switch class {
	case abi.Integer:
		switch size {
		case 1:
			return ir.I8
		case 2:
			return ir.I16
		case 4:
			return ir.I32
		case 8:
			return ir.I64

		default:
			panic("codegen.getTypeForClass() - Invalid integer size")
		}

	case abi.Float:
		switch size {
		case 4:
			return ir.Float
		case 8:
			return ir.Double

		default:
			panic("codegen.getTypeForClass() - Invalid float size")
		}

	default:
		panic("codegen.getTypeForClass() - Invalid class")
	}
}
