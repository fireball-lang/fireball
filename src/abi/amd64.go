package abi

import (
	"fireball/types"
	"slices"
)

type amd64 struct{}

var AMD64 ABI = &amd64{}

func (a *amd64) Info(typ types.Type) Info {
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
		layout := structLayout{abi: a, packed: typ.Packed}

		for _, field := range typ.Fields {
			layout.Field(field.Type)
		}

		return layout.Info()

	default:
		panic("abi.amd64.Info() - Invalid type")
	}
}

func (a *amd64) Classify(typ types.Type) ([]Class, Info) {
	info := a.Info(typ)
	if info.Size > 16 {
		return []Class{Memory}, info
	}

	regs := flatten(a, typ, 0, nil)
	classes := make([]Class, ceilDiv(info.Size, 8))

	for _, reg := range regs {
		class := &classes[reg.offset/8]
		*class = a.merge(*class, reg.class)
	}

	if slices.Contains(classes, Memory) {
		return []Class{Memory}, info
	}

	return classes, info
}

func (*amd64) merge(a, b Class) Class {
	if a == invalid {
		return b
	}
	if b == invalid {
		return a
	}

	if a == Memory || b == Memory {
		return Memory
	}

	if a == Integer || b == Integer {
		return Integer
	}

	return Float
}
