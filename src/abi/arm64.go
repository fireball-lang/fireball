package abi

import (
	"fireball/types"
)

type arm64 struct{}

var ARM64 Arch = &arm64{}

func (a *arm64) Info(typ types.Type) Info {
	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	switch typ := typ.(type) {
	case *types.Primitive:
		s := typ.Kind.Size()
		return Info{Size: s, Align: s}

	case *types.Pointer:
		return Info{Size: 8, Align: 8}

	case *types.Array:
		elem := a.Info(typ.Element)
		return Info{Size: elem.Size * typ.Size, Align: elem.Align}

	case *types.Struct:
		layout := structLayout{arch: a, packed: typ.Packed}

		for _, field := range typ.Fields {
			layout.Field(field.Type)
		}

		return layout.Info()

	default:
		panic("abi.arm64.Info() - Invalid type")
	}
}
