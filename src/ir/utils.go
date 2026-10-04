package ir

import "iter"

type FieldRange struct {
	Offset uint64
	Size   uint64
}

func GetStructFieldRanges[T StructLikeType](s T) iter.Seq2[int, FieldRange] {
	if s.IsPacked() {
		return func(yield func(int, FieldRange) bool) {
			offset := uint64(0)

			for i, field := range s.AllFields() {
				size := uint64(field.Type.Info().Size)

				if !yield(i, FieldRange{Offset: offset, Size: size}) {
					return
				}

				offset += size
			}
		}
	}

	return func(yield func(int, FieldRange) bool) {
		offset := uint64(0)

		for i, field := range s.AllFields() {
			info := field.Type.Info()
			offset = AlignTo(offset, uint64(info.Align))

			if !yield(i, FieldRange{Offset: offset, Size: uint64(info.Size)}) {
				return
			}

			offset += uint64(info.Size)
		}
	}
}

func AlignTo(num, align uint64) uint64 {
	if num%align != 0 {
		num += align - (num % align)
	}

	return num
}
