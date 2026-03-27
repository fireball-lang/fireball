package llvm

import "fireball/ir"

func (w *writer) typ(typ ir.Type) {
	switch typ := typ.(type) {
	case *ir.SimpleType:
		w.simpleTyp(typ)
	case *ir.IntegerType:
		w.integerTyp(typ)
	case *ir.VectorType:
		w.vectorTyp(typ)
	case *ir.ArrayType:
		w.arrayTyp(typ)
	case *ir.StructType:
		w.structTyp(typ)
	case *ir.RefStructType:
		w.refStructTyp(typ)

	default:
		panic("llvm.writer.writeType() - Invalid type")
	}
}

func (w *writer) simpleTyp(typ *ir.SimpleType) {
	switch typ.Kind {
	case ir.VoidKind:
		w.string("void")
	case ir.FloatKind:
		w.string("float")
	case ir.DoubleKind:
		w.string("double")
	case ir.PointerKind:
		w.string("ptr")
	}
}

func (w *writer) integerTyp(typ *ir.IntegerType) {
	w.rune('i')
	w.uint(uint64(typ.Bits), 10)
}

func (w *writer) vectorTyp(typ *ir.VectorType) {
	w.rune('<')
	w.uint(uint64(typ.Length), 10)
	w.string(" x ")
	w.typ(typ.Element)
	w.rune('>')
}

func (w *writer) arrayTyp(typ *ir.ArrayType) {
	w.rune('[')
	w.uint(uint64(typ.Length), 10)
	w.string(" x ")
	w.typ(typ.Element)
	w.rune(']')
}

func (w *writer) structTyp(typ *ir.StructType) {
	if typ.Packed {
		w.string("<{ ")
	} else {
		w.string("{ ")
	}

	for i, field := range typ.Fields {
		if i > 0 {
			w.string(", ")
		}

		w.typ(field)
	}

	if typ.Packed {
		w.string(" }>")
	} else {
		w.string(" }")
	}
}

func (w *writer) refStructTyp(typ *ir.RefStructType) {
	w.rune('%')
	w.identifier(typ.Name)
}
