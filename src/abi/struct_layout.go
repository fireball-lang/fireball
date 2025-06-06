package abi

import "fireball/ast"

type StructLayout struct {
	Arch Arch

	biggestAlign uint32
	offset       uint32
}

func (s *StructLayout) Field(type_ ast.Type) uint32 {
	size, align := TypeInfo(s.Arch, type_)

	s.biggestAlign = max(s.biggestAlign, align)

	offset := alignTo(s.offset, align)
	s.offset = offset + size

	return offset
}

func (s *StructLayout) Info() (uint32, uint32) {
	if s.offset == 0 {
		return 0, 0
	}

	return alignTo(s.offset, s.biggestAlign), s.biggestAlign
}
