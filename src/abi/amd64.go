package abi

import (
	"fireball/types"
)

type amd64 struct{}

var AMD64 Arch = &amd64{}

func (a *amd64) Info(typ types.Type) Info {
	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	switch typ := typ.(type) {
	case *types.Primitive:
		s := typ.Kind.Size()
		return Info{Size: s, Align: s}

	case *types.Reference, *types.Pointer:
		return Info{Size: 8, Align: 8}

	case *types.Array:
		elem := a.Info(typ.Element)
		return Info{Size: elem.Size * typ.Size, Align: elem.Align}

	case *types.Struct:
		return getStructLayout(a, typ)

	default:
		panic("abi.amd64.Info() - Invalid type")
	}
}
