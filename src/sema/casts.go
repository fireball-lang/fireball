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

	PointerToReference
	ReferenceToInterface

	PointerToInterface
	InterfaceToPointer
	InterfaceToInterface

	TypeToOption
	ImplicitAs
	ArrayToSlice
)

func CommonType(env *TypeEnvironment, a, b types.Type) types.Type {
	if ia, ok := a.(*types.Integer); ok {
		if pb, ok := b.(*types.Primitive); ok && (types.IsInteger(pb.Kind) || types.IsFloating(pb.Kind)) {
			if _, ok := GetImplicitCast(env, ExprInfo{Type: ia}, pb); ok {
				return pb
			}
		}
	}
	if ib, ok := b.(*types.Integer); ok {
		if pa, ok := a.(*types.Primitive); ok && (types.IsInteger(pa.Kind) || types.IsFloating(pa.Kind)) {
			if _, ok := GetImplicitCast(env, ExprInfo{Type: ib}, pa); ok {
				return pa
			}
		}
	}

	if i, ok := a.(*types.Integer); ok {
		a = i.ToPrimitive()
	}
	if i, ok := b.(*types.Integer); ok {
		b = i.ToPrimitive()
	}

	switch a := a.(type) {
	case *types.Null:
		switch b.(type) {
		case *types.Null, *types.Pointer:
			return a.Underlying()

		case *types.Interface:
			return b
		}

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

	case *types.Reference:
		switch b := b.(type) {
		case *types.Reference:
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
			if slices.Contains(env.GetConformances(a.Pointee), b.AsImmutable()) {
				common := b.AsImmutable()
				if a.Mutable {
					common = b.AsMutable()
				}
				return common
			}
		}

	case *types.Pointer:
		switch b := b.(type) {
		case *types.Null:
			return b.Underlying()

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
			if slices.Contains(env.GetConformances(a.Pointee), b.AsImmutable()) {
				common := b.AsImmutable()
				if a.Mutable {
					common = b.AsMutable()
				}
				return common
			}
		}

	case *types.Func:
		switch b := b.(type) {
		case *types.Reference:
			if b.Pointee == types.PrimitiveVoid {
				return b
			}

		case *types.Pointer:
			if b.Pointee == types.PrimitiveVoid {
				return b
			}
		}

	case *types.Interface:
		switch b := b.(type) {
		case *types.Null:
			return a

		case *types.Reference:
			if slices.Contains(env.GetConformances(b.Pointee), a.AsImmutable()) {
				common := a.AsImmutable()
				if b.Mutable {
					common = a.AsMutable()
				}
				return common
			}

		case *types.Pointer:
			if slices.Contains(env.GetConformances(b.Pointee), a.AsImmutable()) {
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
	case *types.Integer:
		switch to := to.(type) {
		case *types.Primitive:
			if types.IsInteger(to.Kind) {
				return getCastFromToInteger(fromT.ToPrimitive().Kind, to.Kind)
			}

			if types.IsFloating(to.Kind) {
				return IntToFloat, true
			}

		case *types.Enum:
			return getCastFromToInteger(fromT.ToPrimitive().Kind, to.CaseType.(*types.Primitive).Kind)
		}

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

	case *types.Reference:
		switch to := to.(type) {
		case *types.Primitive:
			if to.Kind == types.U64 {
				return PointerToInt, true
			}

		case *types.Reference, *types.Pointer, *types.Func:
			return Noop, true
		}

	case *types.Pointer:
		switch to := to.(type) {
		case *types.Primitive:
			if to.Kind == types.U64 {
				return PointerToInt, true
			}

		case *types.Reference, *types.Func:
			return PointerToReference, true

		case *types.Pointer:
			return Noop, true
		}

	case *types.Func:
		switch to.(type) {
		case *types.Reference, *types.Pointer, *types.Func:
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
	case *types.Null:
		switch to.(type) {
		case *types.Pointer:
			return Noop, true

		case *types.Interface:
			return PointerToInterface, true
		}

	case *types.Integer:
		switch to := to.(type) {
		case *types.Primitive:
			if types.IsInteger(to.Kind) {
				if fromT.Unsigned && types.IsSignedInteger(to.Kind) {
					return Noop, false
				}
				if fromT.Negative && types.IsUnsignedInteger(to.Kind) {
					return Noop, false
				}

				// Precise raw-bit value check
				toBits := to.Kind.Size() * 8
				maxRaw := toBits

				if types.IsSignedInteger(to.Kind) {
					maxRaw = toBits - 1
				}
				if fromT.RawBits > maxRaw {
					return Noop, false
				}

				// Cast kind from materialized width
				fromPrim := fromT.ToPrimitive()

				if fromPrim.Kind.Size() == to.Kind.Size() {
					return Noop, true
				}
				if fromPrim.Kind.Size() < to.Kind.Size() {
					if fromT.Negative {
						return SignExtend, true
					}
					return ZeroExtend, true
				}

				// Materialized width wider, but value still fits target -> re-type literal
				return Truncate, true
			}

			if types.IsFloating(to.Kind) {
				fromBits := fromT.Bits()
				toBits := to.Kind.Size() * 8

				if fromBits < toBits {
					return IntToFloat, true
				}
			}
		}

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

	case *types.Reference:
		switch to := to.(type) {
		case *types.Reference:
			if (fromT.Pointee.Equals(to.Pointee) || to.Pointee == types.PrimitiveVoid) && (fromT.Mutable == to.Mutable || (fromT.Mutable && !to.Mutable)) {
				return Noop, true
			}

		case *types.Pointer:
			if (fromT.Pointee.Equals(to.Pointee) || to.Pointee == types.PrimitiveVoid) && (fromT.Mutable == to.Mutable || (fromT.Mutable && !to.Mutable)) {
				return Noop, true
			}

		case *types.Interface:
			if slices.Contains(env.GetConformances(fromT.Pointee), to.AsImmutable()) {
				if to.Mutable && !fromT.Mutable {
					break
				}

				return ReferenceToInterface, true
			}
		}

	case *types.Pointer:
		switch to := to.(type) {
		case *types.Pointer:
			if (fromT.Pointee.Equals(to.Pointee) || to.Pointee == types.PrimitiveVoid) && (fromT.Mutable == to.Mutable || (fromT.Mutable && !to.Mutable)) {
				return Noop, true
			}

		case *types.Interface:
			if slices.Contains(env.GetConformances(fromT.Pointee), to.AsImmutable()) {
				if to.Mutable && !fromT.Mutable {
					break
				}

				return PointerToInterface, true
			}
		}

	case *types.Func:
		switch to := to.(type) {
		case *types.Reference:
			if to.Pointee == types.PrimitiveVoid {
				return Noop, true
			}

		case *types.Pointer:
			if to.Pointee == types.PrimitiveVoid {
				return Noop, true
			}
		}
	case *types.Interface:
		if to, ok := to.(*types.Interface); ok && !to.Mutable && fromT.Mutable && fromT.AsImmutable() == to {
			return Noop, true
		}

	case *types.Param:
		for _, in := range fromT.Constraints {
			if in.Name == "core::ImplicitAs" && len(in.Substitutions) == 1 && in.Substitutions[0].Type.Equals(to) {
				return ImplicitAs, true
			}
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
