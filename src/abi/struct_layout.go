package abi

import (
	"cmp"
	"fireball/types"
	"slices"
)

type structLayout struct {
	arch   Arch
	packed bool

	biggestAlign uint32
	offset       uint32

	fields []Field
}

type indexInfo struct {
	index uint32
	Info
}

func getStructLayout[T Arch](arch T, s *types.Struct) Info {
	layout := structLayout{arch: arch, packed: s.Packed}

	switch s.Layout {
	case types.Fireball:
		fields := make([]indexInfo, len(s.Fields))

		for i, field := range s.Fields {
			fields[i] = indexInfo{
				index: uint32(i),
				Info:  arch.Info(field.Type),
			}
		}

		// Sort from biggest to smallest alignment
		slices.SortStableFunc(fields, func(a, b indexInfo) int {
			return cmp.Compare(b.Align, a.Align)
		})

		for _, field := range fields {
			layout.field(field.index, field.Info)
		}

	case types.C:
		for i, field := range s.Fields {
			info := arch.Info(field.Type)
			layout.field(uint32(i), info)
		}

	default:
		panic("abi.getStructLayout() - Invalid struct layout")
	}

	return layout.info()
}

func (s *structLayout) field(index uint32, info Info) uint32 {
	// Packed
	if s.packed {
		offset := s.offset
		s.offset = offset + info.Size

		s.fields = append(s.fields, Field{Index: index, Offset: offset})
		return offset
	}

	// Normal
	s.biggestAlign = max(s.biggestAlign, info.Align)

	offset := alignTo(s.offset, info.Align)
	s.offset = offset + info.Size

	s.fields = append(s.fields, Field{Index: index, Offset: offset})
	return offset
}

func (s *structLayout) info() Info {
	if s.offset == 0 {
		return Info{}
	}

	if s.packed {
		s.biggestAlign = 1
	}

	return Info{
		Size:   alignTo(s.offset, s.biggestAlign),
		Align:  s.biggestAlign,
		Fields: s.fields,
	}
}
