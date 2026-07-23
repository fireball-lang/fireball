package abi

import "fireball/types"

const invalid Class = 0

func alignTo(num, align uint32) uint32 {
	if num%align != 0 {
		num += align - (num % align)
	}

	return num
}

func ceilDiv(x, y uint32) uint32 {
	return (x + y - 1) / y
}

type register struct {
	offset uint32
	class  Class
}

func flatten(arch Arch, typ types.Type, offset uint32, regs []register) []register {
	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	switch typ := typ.(type) {
	case *types.Primitive:
		if typ.Kind != types.Void {
			class := Integer
			if typ.Kind == types.F32 || typ.Kind == types.F64 {
				class = Float
			}

			regs = append(regs, register{offset, class})
		}

	case *types.Pointer:
		regs = append(regs, register{offset, Integer})

	case *types.Array:
		info := arch.Info(typ.Element)

		for i := uint32(0); i < typ.Size; i++ {
			regs = flatten(arch, typ.Element, offset, regs)
			offset += info.Size
		}

	case *types.Struct:
		info := arch.Info(typ)

		for _, field := range info.Fields {
			regs = flatten(arch, typ.Fields[field.Index].Type, offset+field.Offset, regs)
		}

	default:
		panic("abi.flatten() - Invalid type")
	}

	return regs
}
