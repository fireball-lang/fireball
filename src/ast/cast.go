package ast

type CastKind uint8

const (
	Nop CastKind = iota
	Extend
	Truncate
	IntegerToFloating
	FloatingToInteger
	IntegerToPointer
	PointerToInteger
)

func GetCastKind(from, to Type, allowExtended bool) (CastKind, bool) {
	switch from := from.(type) {
	case *PrimitiveType:
		if to, ok := to.(*PrimitiveType); ok {
			if (from.Kind.IsInteger() && to.Kind.IsInteger()) || (from.Kind.IsFloating() && to.Kind.IsFloating()) {
				if from.Kind.BitCount() == to.Kind.BitCount() {
					return Nop, true
				}
				if from.Kind.BitCount() > to.Kind.BitCount() {
					return Truncate, true
				}
				return Extend, true
			}

			if from.Kind.IsInteger() && to.Kind.IsFloating() {
				return IntegerToFloating, true
			}

			if from.Kind.IsFloating() && to.Kind.IsInteger() {
				return FloatingToInteger, true
			}

			if allowExtended {
				if from.Kind == Bool && to.Kind.IsInteger() {
					return Extend, true
				}

				if from.Kind.IsInteger() && to.Kind == Bool {
					return Truncate, true
				}
			}
		}

		if allowExtended {
			_, toIsPointer := to.(*PointerType)
			_, toIsFunc := to.(FuncType)

			if from.Kind.IsInteger() && (toIsPointer || toIsFunc) {
				return IntegerToPointer, true
			}
		}

	case *PointerType:
		if _, ok := to.(*PointerType); ok {
			return Nop, true
		}

		if _, ok := to.(FuncType); ok {
			return Nop, true
		}

		if to, ok := to.(*PrimitiveType); allowExtended && ok && to.Kind.IsInteger() {
			return PointerToInteger, true
		}

	case FuncType:
		if to, ok := to.(*PrimitiveType); allowExtended && ok && to.Kind.IsInteger() {
			return PointerToInteger, true
		}
	}

	return Nop, false
}

func GetImplicitCastKind(from, to Type) (CastKind, bool) {
	switch from := from.(type) {
	case *PrimitiveType:
		if to, ok := to.(*PrimitiveType); ok {
			if (from.Kind.IsSignedInteger() && to.Kind.IsSignedInteger()) || (from.Kind.IsUnsignedInteger() && to.Kind.IsUnsignedInteger()) || (from.Kind.IsFloating() && to.Kind.IsFloating()) {
				if from.Kind.BitCount() < to.Kind.BitCount() {
					return Extend, true
				}
			}
		}

	case *PointerType:
		if to, ok := to.(*PointerType); ok && to.Pointee.Equals(VoidType) {
			return Nop, true
		}
	}

	return Nop, false
}
