package abi

import "fireball/types"

type aapcs64 struct{}

var AAPCS64 CallConv = &aapcs64{}

func (a *aapcs64) Classify(arch Arch, typ types.Type) ([]Class, Info) {
	info := arch.Info(typ)
	if info.Size > 16 {
		return []Class{Memory}, info
	}

	if t, ok := typ.(types.Composed); ok {
		typ = t.Underlying()
	}

	switch typ := typ.(type) {
	case *types.Primitive:
		if typ.Kind == types.Void {
			return nil, info
		}

		if types.IsFloating(typ.Kind) {
			return []Class{Float}, info
		}

		return []Class{Integer}, info

	case *types.Pointer:
		return []Class{Integer}, info

	case *types.Array, *types.Struct:
		hfaCount := a.IsHFA(typ)

		if hfaCount > 0 && hfaCount <= 4 {
			classes := make([]Class, hfaCount)

			for i := 0; i < hfaCount; i++ {
				classes[i] = Float
			}

			return classes, info
		}

		if info.Size <= 8 {
			return []Class{Integer}, info
		}

		return []Class{Integer, Integer}, info

	default:
		panic("abi.aapcs64.Classify() - Invalid type")
	}
}

func (a *aapcs64) IsHFA(typ types.Type) int {
	switch typ := typ.(type) {
	case *types.Array:
		if typ.Element == types.PrimitiveF32 || typ.Element == types.PrimitiveF64 {
			return int(typ.Size)
		}

		return -1

	case *types.Struct:
		if len(typ.Fields) == 0 {
			return -1
		}

		baseType := typ.Fields[0].Type
		if baseType != types.PrimitiveF32 && baseType != types.PrimitiveF64 {
			return -1
		}

		for _, field := range typ.Fields[1:] {
			if field.Type != baseType {
				return -1
			}
		}

		return len(typ.Fields)

	default:
		panic("abi.aapcs64.IsHFA() - Invalid type")
	}
}
