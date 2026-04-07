package sema

import "fireball/types"

type CastKind uint8

const (
	Noop CastKind = iota

	ZeroExtend
	SignExtend
	Truncate

	IntToFloat
	FloatToInt

	FloatExtend
	FloatTruncate

	IntToPointer
	PointerToInt
)

func CommonType(a, b types.Type) types.Type {
	pa, aOk := a.(*types.Primitive)
	pb, bOk := b.(*types.Primitive)

	if !aOk || !bOk {
		return nil
	}

	if types.IsInteger(pa.Kind) && types.IsInteger(pb.Kind) && types.IsSigned(pa.Kind) == types.IsSigned(pb.Kind) {
		if pa.Kind.Size() >= pb.Kind.Size() {
			return a
		}
		return b
	}

	if types.IsFloating(pa.Kind) && types.IsFloating(pb.Kind) {
		if pa.Kind.Size() >= pb.Kind.Size() {
			return a
		}
		return b
	}

	return nil
}

func GetExplicitCast(from, to types.Type) (CastKind, bool) {
	if kind, ok := GetImplicitCast(from, to); ok {
		return kind, true
	}

	switch from := from.(type) {
	case *types.Primitive:
		switch to := to.(type) {
		case *types.Primitive:
			if types.IsInteger(from.Kind) && types.IsInteger(to.Kind) {
				fromSize := from.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize == toSize {
					return Noop, true
				}

				if fromSize < toSize {
					if types.IsSigned(from.Kind) {
						return SignExtend, true
					}

					return ZeroExtend, true
				}

				if fromSize > toSize {
					return Truncate, true
				}
			}

			if types.IsInteger(from.Kind) && types.IsFloating(to.Kind) {
				return IntToFloat, true
			}

			if types.IsFloating(from.Kind) && types.IsInteger(to.Kind) {
				return FloatToInt, true
			}

			if types.IsFloating(from.Kind) && types.IsFloating(to.Kind) {
				fromSize := from.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize > toSize {
					return FloatTruncate, true
				}
			}

		case *types.Pointer:
			if from.Kind == types.U64 {
				return IntToPointer, true
			}
		}

	case *types.Pointer:
		switch to := to.(type) {
		case *types.Primitive:
			if to.Kind == types.U64 {
				return PointerToInt, true
			}

		case *types.Pointer:
			return Noop, true
		}
	}

	return Noop, false
}

func GetImplicitCast(from, to types.Type) (CastKind, bool) {
	if from.Equals(to) {
		return Noop, true
	}

	switch from := from.(type) {
	case *types.Primitive:
		switch to := to.(type) {
		case *types.Primitive:
			if types.IsInteger(from.Kind) && types.IsInteger(to.Kind) && types.IsSigned(from.Kind) == types.IsSigned(to.Kind) {
				fromSize := from.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize < toSize {
					if types.IsSigned(from.Kind) {
						return SignExtend, true
					}

					return ZeroExtend, true
				}
			}

			if types.IsInteger(from.Kind) && types.IsFloating(to.Kind) {
				fromSize := from.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize < toSize {
					return IntToFloat, true
				}
			}

			if types.IsFloating(from.Kind) && types.IsFloating(to.Kind) {
				fromSize := from.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize < toSize {
					return FloatExtend, true
				}
			}
		}
	}

	return Noop, false
}
