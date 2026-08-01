package sema

import (
	"fireball/core"
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
	InterfaceToInterface

	TypeToOption
	ImplicitAs
	ArrayToSlice
)

func CommonType(env *TypeEnvironment, a, b types.Type) types.Type {
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

		case *types.Interface:
			if a.Pointee == types.PrimitiveVoid || slices.Contains(env.GetConformances(a.Pointee), b.AsImmutable()) {
				common := b.AsImmutable()
				if a.Mutable {
					common = b.AsMutable()
				}
				return common
			}
		}

	case *types.Func:
		switch b := b.(type) {
		case *types.Pointer:
			if b.Pointee == types.PrimitiveVoid {
				return b
			}
		}

	case *types.Interface:
		switch b := b.(type) {
		case *types.Pointer:
			if b.Pointee == types.PrimitiveVoid || slices.Contains(env.GetConformances(b.Pointee), a.AsImmutable()) {
				common := a.AsImmutable()
				if b.Mutable {
					common = a.AsMutable()
				}
				return common
			}
		}
	}

	return nil
}

func GetExplicitCast(env *TypeEnvironment, from ExprInfo, to types.Type) (CastKind, bool) {
	if kind, ok := GetImplicitCast(env, from, to); ok {
		return kind, true
	}

	switch fromT := from.Type.(type) {
	case *types.Primitive:
		switch to := to.(type) {
		case *types.Primitive:
			if types.IsInteger(fromT.Kind) && types.IsInteger(to.Kind) {
				return getCastFromToInteger(fromT.Kind, to.Kind)
			}

			if types.IsInteger(fromT.Kind) && types.IsFloating(to.Kind) {
				return IntToFloat, true
			}

			if types.IsFloating(fromT.Kind) && types.IsInteger(to.Kind) {
				return FloatToInt, true
			}

			if types.IsFloating(fromT.Kind) && types.IsFloating(to.Kind) {
				fromSize := fromT.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize > toSize {
					return FloatTruncate, true
				}
			}

		case *types.Pointer:
			if fromT.Kind == types.U64 {
				return IntToPointer, true
			}

		case *types.Enum:
			if types.IsInteger(fromT.Kind) {
				return getCastFromToInteger(fromT.Kind, to.CaseType.(*types.Primitive).Kind)
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
			return getCastFromToInteger(fromT.CaseType.(*types.Primitive).Kind, to.Kind)
		}

	case *types.Interface:
		switch to := to.(type) {
		case *types.Pointer:
			if slices.Contains(env.GetConformances(to.Pointee), fromT.AsImmutable()) {
				if !fromT.Mutable && to.Mutable {
					break
				}

				return InterfaceToPointer, true
			}

		case *types.Interface:
			if !fromT.Mutable && to.Mutable {
				break
			}

			return InterfaceToInterface, true
		}
	}

	return Noop, false
}

func GetImplicitCast(env *TypeEnvironment, from ExprInfo, to types.Type) (CastKind, bool) {
	if from.Type.Equals(to) {
		return Noop, true
	}

	switch fromT := from.Type.(type) {
	case *types.Primitive:
		switch to := to.(type) {
		case *types.Primitive:
			if types.IsInteger(fromT.Kind) && types.IsInteger(to.Kind) && types.IsSigned(fromT.Kind) == types.IsSigned(to.Kind) {
				fromSize := fromT.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize < toSize {
					if types.IsSigned(fromT.Kind) {
						return SignExtend, true
					}

					return ZeroExtend, true
				}
			}

			if types.IsInteger(fromT.Kind) && types.IsFloating(to.Kind) {
				fromSize := fromT.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize < toSize {
					return IntToFloat, true
				}
			}

			if types.IsFloating(fromT.Kind) && types.IsFloating(to.Kind) {
				fromSize := fromT.Kind.Size()
				toSize := to.Kind.Size()

				if fromSize < toSize {
					return FloatExtend, true
				}
			}
		}

	case *types.Array:
		if to, ok := to.(*types.Struct); ok && from.Address && (to.Name == "core::Slice" || (from.Mutable && to.Name == "core::MutSlice")) && len(to.Substitutions) == 1 && to.Substitutions[0].Type.Equals(fromT.Element) {
			return ArrayToSlice, true
		}

	case *types.Pointer:
		switch to := to.(type) {
		case *types.Pointer:
			if (fromT.Pointee.Equals(to.Pointee) || to.Pointee == types.PrimitiveVoid) && (fromT.Mutable == to.Mutable || (fromT.Mutable && !to.Mutable)) {
				return Noop, true
			}

		case *types.Interface:
			if fromT.Pointee == types.PrimitiveVoid || slices.Contains(env.GetConformances(fromT.Pointee), to.AsImmutable()) {
				if to.Mutable && !fromT.Mutable {
					break
				}

				return PointerToInterface, true
			}
		}

	case *types.Func:
		if to, ok := to.(*types.Pointer); ok && to.Pointee == types.PrimitiveVoid {
			return Noop, true
		}

	case *types.Interface:
		if to, ok := to.(*types.Interface); ok && !to.Mutable && fromT.Mutable && fromT.AsImmutable() == to {
			return Noop, true
		}
	}

	if toInner := getOptionInnerType(to); !core.IsNil(toInner) {
		if _, ok := GetImplicitCast(env, from, toInner); ok {
			return TypeToOption, true
		}
	}

	for _, in := range env.GetConformances(from.Type) {
		if in.Name == "core::ImplicitAs" && len(in.Substitutions) == 1 && in.Substitutions[0].Type.Equals(to) {
			return ImplicitAs, true
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
