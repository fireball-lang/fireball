package sema

import (
	"fireball/types"
	"slices"
)

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

	PointerToInterface
	InterfaceToPointer
)

func CommonType(a, b types.Type) types.Type {
	// Pointer
	if pa, ok := a.(*types.Pointer); ok {
		if pb, ok := b.(*types.Pointer); ok {
			if pa.Pointee == types.PrimitiveVoid {
				return pa
			}
			if pb.Pointee == types.PrimitiveVoid {
				return pb
			}

			return nil
		}

		return nil
	}

	// Primitive
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

func GetExplicitCast(env *TypeEnvironment, from, to types.Type) (CastKind, bool) {
	if kind, ok := GetImplicitCast(env, from, to); ok {
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

	case *types.Interface:
		if to, ok := to.(*types.Pointer); ok && slices.Contains(env.GetConformances(to.Pointee), from.AsImmutable()) {
			if to.Mutable && !from.Mutable {
				break
			}

			return InterfaceToPointer, true
		}
	}

	return Noop, false
}

func GetImplicitCast(env *TypeEnvironment, from, to types.Type) (CastKind, bool) {
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

	case *types.Pointer:
		if to, ok := to.(*types.Pointer); ok && (from.Mutable == to.Mutable || (from.Mutable && !to.Mutable)) {
			return Noop, true
		}

		if to, ok := to.(*types.Interface); ok && slices.Contains(env.GetConformances(from.Pointee), to.AsImmutable()) {
			if to.Mutable && !from.Mutable {
				break
			}

			return PointerToInterface, true
		}

	case *types.Interface:
		if to, ok := to.(*types.Interface); ok && !to.Mutable && from.Mutable && from.AsImmutable() == to {
			return Noop, true
		}
	}

	return Noop, false
}
