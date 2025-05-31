package ast

type CastKind uint8

const (
	Nop CastKind = iota
	Extend
	Truncate
	IntegerToFloating
	FloatingToInteger
)

func GetCastKind(from Type, to Type) (CastKind, bool) {
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
		}

	case *PointerType:
		if _, ok := to.(*PointerType); ok {
			return Nop, true
		}

		if _, ok := to.(FuncType); ok {
			return Nop, true
		}
	}

	return Nop, false
}
