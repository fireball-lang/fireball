package abi

import "fireball/types"

type structLayout struct {
	abi    ABI
	packed bool

	biggestAlign uint32
	offset       uint32

	offsets []uint32
}

func (s *structLayout) Field(typ types.Type) uint32 {
	info := s.abi.Info(typ)

	// Packed
	if s.packed {
		offset := s.offset
		s.offset = offset + info.Size

		s.offsets = append(s.offsets, offset)
		return offset
	}

	// Normal
	s.biggestAlign = max(s.biggestAlign, info.Align)

	offset := alignTo(s.offset, info.Align)
	s.offset = offset + info.Size

	s.offsets = append(s.offsets, offset)
	return offset
}

func (s *structLayout) Info() Info {
	if s.offset == 0 {
		return Info{}
	}

	if s.packed {
		s.biggestAlign = 1
	}

	return Info{
		Size:    alignTo(s.offset, s.biggestAlign),
		Align:   s.biggestAlign,
		Offsets: s.offsets,
	}
}
