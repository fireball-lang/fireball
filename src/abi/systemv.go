package abi

import (
	"fireball/types"
	"slices"
)

type systemV struct{}

var SystemV CallConv = &systemV{}

func (s *systemV) Classify(arch Arch, typ types.Type) ([]Class, Info) {
	info := arch.Info(typ)
	if info.Size > 16 {
		return []Class{Memory}, info
	}

	regs := flatten(arch, typ, 0, nil)
	classes := make([]Class, ceilDiv(info.Size, 8))

	for _, reg := range regs {
		class := &classes[reg.offset/8]
		*class = s.merge(*class, reg.class)
	}

	if slices.Contains(classes, Memory) {
		return []Class{Memory}, info
	}

	return classes, info
}

func (*systemV) merge(a, b Class) Class {
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
