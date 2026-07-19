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
	switch a := a.(type) {
	case *types.Primitive:
		switch b := b.(type) {
		case *types.Primitive:
			if types.IsInteger(a.Kind) && types.IsInteger(b.Kind) && types.IsSigned(a.Kind) == types.IsSigned(b.Kind) {
				if a.Kind.Size() >= b.Kind.Size() {
					return a
				}
				return b
			}

			if types.IsFloating(a.Kind) && types.IsFloating(b.Kind) {
				if a.Kind.Size() >= b.Kind.Size() {
					return a
				}
				return b
			}
		}

	case *types.Pointer:
		switch b := b.(type) {
		case *types.Pointer:
			if a.Pointee == types.PrimitiveVoid {
				return a
			}
			if b.Pointee == types.PrimitiveVoid {
				return b
			}

		case *types.Func:
			if a.Pointee == types.PrimitiveVoid {
				return a
			}
		}

	case *types.Func:
		switch b := b.(type) {
		case *types.Pointer:
			if b.Pointee == types.PrimitiveVoid {
				return b
			}
		}
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
				return getCastFromToInteger(from.Kind, to.Kind)
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

		case *types.Enum:
			if types.IsInteger(from.Kind) {
				return getCastFromToInteger(from.Kind, to.CaseType.(*types.Primitive).Kind)
			}
		}

	case *types.Pointer:
		switch to := to.(type) {
		case *types.Primitive:
			if to.Kind == types.U64 {
				return PointerToInt, true
			}

		case *types.Pointer, *types.Func:
			return Noop, true
		}

	case *types.Func:
		switch to.(type) {
		case *types.Pointer, *types.Func:
			return Noop, true
		}

	case *types.Enum:
		if to, ok := to.(*types.Primitive); ok && types.IsInteger(to.Kind) {
			return getCastFromToInteger(from.CaseType.(*types.Primitive).Kind, to.Kind)
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
		if to, ok := to.(*types.Pointer); ok && (from.Pointee.Equals(to.Pointee) || to.Pointee == types.PrimitiveVoid) && (from.Mutable == to.Mutable || (from.Mutable && !to.Mutable)) {
			return Noop, true
		}

		if to, ok := to.(*types.Interface); ok && slices.Contains(env.GetConformances(from.Pointee), to.AsImmutable()) {
			if to.Mutable && !from.Mutable {
				break
			}

			return PointerToInterface, true
		}

	case *types.Func:
		if to, ok := to.(*types.Pointer); ok && to.Pointee == types.PrimitiveVoid {
			return Noop, true
		}

	case *types.Interface:
		if to, ok := to.(*types.Interface); ok && !to.Mutable && from.Mutable && from.AsImmutable() == to {
			return Noop, true
		}
	}

	return Noop, false
}

func getCastFromToInteger(from, to types.PrimitiveKind) (CastKind, bool) {
	fromSize := from.Size()
	toSize := to.Size()

	if fromSize == toSize {
		return Noop, true
	}

	if fromSize < toSize {
		if types.IsSigned(from) {
			return SignExtend, true
		}

		return ZeroExtend, true
	}

	if fromSize > toSize {
		return Truncate, true
	}

	return Noop, false
}
