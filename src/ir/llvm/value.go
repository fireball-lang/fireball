package llvm

import (
	"fireball/ir"
	"math"
	"unicode/utf8"
)

func (w *writer) typValue(value ir.Value) {
	w.typ(value.Type())
	w.rune(' ')
	w.value(value)
}

func (w *writer) value(value ir.Value) {
	switch value := value.(type) {
	// Constants

	case *ir.ZeroInitializer:
		w.string("zeroinitializer")

	case *ir.Null:
		w.string("null")

	case *ir.Integer:
		if value.Value.Negative() {
			w.rune('-')
		}
		w.uint(value.Value.Raw(), 10)

	case *ir.FloatV:
		bits := math.Float64bits(float64(value.Value))
		w.string("0x")
		w.uint(bits, 16)

	case *ir.DoubleV:
		bits := math.Float64bits(value.Value)
		w.string("0x")
		w.uint(bits, 16)

	case *ir.String:
		w.string("c\"")

		var buf [4]byte

		for _, ch := range value.Runes {
			if ch >= 0x20 && ch <= 0x7E && ch != '"' && ch != '\\' {
				w.rune(ch)
			} else {
				size := utf8.EncodeRune(buf[:], ch)

				for i := 0; i < size; i++ {
					w.rune('\\')
					w.hex(uint64(buf[i]), 2)
				}
			}
		}

		if value.NullTerminated {
			w.string("\\00")
		}

		w.rune('"')

	case *ir.Vector:
		w.arrayLikeValue("< ", " >", value.Elements)

	case *ir.Array:
		w.arrayLikeValue("[ ", " ]", value.Elements)

	case *ir.Struct:
		w.arrayLikeValue("{ ", " }", value.Fields)

	case *ir.Assembly:
		w.string("asm")

		if value.SideEffect {
			w.string(" sideeffect")
		}

		w.string(" \"")
		w.string(value.Template)
		w.string("\", \"")

		for i, constraint := range value.Constraints {
			if i > 0 {
				w.rune(',')
			}

			w.string(constraint)
		}

		w.rune('"')

	// Identifiers

	case *ir.GlobalVar:
		w.rune('@')
		w.identifier(value.Name)

	case *ir.Param:
		w.rune('%')
		w.identifier(value.Name)

	case *ir.Function:
		w.rune('@')
		w.identifier(value.Name)

	case ir.Instruction:
		w.rune('%')

		if value.Name() != "" {
			if name, ok := w.valueNameMap[value]; ok {
				w.identifier(name)
			} else {
				w.identifier(value.Name())
			}
		} else {
			w.uint(uint64(w.instructionIds[value]), 10)
		}

	// Invalid

	default:
		panic("llvm.writer.value() - Invalid value")
	}
}

func (w *writer) arrayLikeValue(start, stop string, elements []ir.Value) {
	w.string(start)

	for i, field := range elements {
		if i > 0 {
			w.string(", ")
		}

		w.typValue(field)
	}

	w.string(stop)
}

func (w *writer) identifier(text string) {
	ok := true

	for _, ch := range text {
		if !(ch == '-' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '$' || ch == '.' || ch == '_' || (ch >= '0' && ch <= '9')) {
			ok = false
			break
		}
	}

	if ok {
		w.string(text)
	} else {
		w.rune('"')
		w.string(text)
		w.rune('"')
	}
}
