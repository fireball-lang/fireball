package abi

import (
	"fireball/types"
)

type win64 struct{}

var Win64 CallConv = &win64{}

func (w *win64) Classify(arch Arch, typ types.Type) ([]Class, Info) {
	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	info := arch.Info(typ)

	switch typ := typ.(type) {
	case *types.Primitive:
		if typ.Kind == types.Void {
			return nil, info
		}

		class := Float
		if typ.Kind == types.Bool || types.IsInteger(typ.Kind) {
			class = Integer
		}

		return []Class{class}, info

	case *types.Pointer:
		return []Class{Integer}, info

	case *types.Array, *types.Struct:
		if info.Size != 1 && info.Size != 2 && info.Size != 4 && info.Size != 8 {
			return []Class{Memory}, info
		}

		return []Class{Integer}, info

	default:
		panic("abi.win64.Classify() - Invalid type")
	}
}
