package llvm

import (
	"fireball/core"
	"fireball/ir"
	"math"
)

func (w *writer) instruction(in ir.Instruction) {
	if !isVoid(in.Type()) {
		w.value(in)
		w.string(" = ")
	}

	switch in := in.(type) {
	// Terminator instructions

	case *ir.Ret:
		w.string("ret ")

		if core.IsNil(in.Value) {
			w.string("void")
		} else {
			w.typValue(in.Value)
		}

	case *ir.Br:
		w.string("br ")
		w.label(in.Label)

	case *ir.BrCond:
		w.string("br ")
		w.typValue(in.Condition)
		w.string(", ")
		w.label(in.IfTrue)
		w.string(", ")
		w.label(in.IfFalse)

	// Unary instructions

	case *ir.FNeg:
		w.string("fneg ")
		w.typValue(in.Value)

	// Binary instructions

	case *ir.Add:
		w.floatingModifier(false, in.Left)
		w.simpleInstruction2("add ", in.Left, in.Right)

	case *ir.Sub:
		w.floatingModifier(false, in.Left)
		w.simpleInstruction2("sub ", in.Left, in.Right)

	case *ir.Mul:
		w.floatingModifier(false, in.Left)
		w.simpleInstruction2("mul ", in.Left, in.Right)

	case *ir.Div:
		w.floatingSignedModifier('u', 's', "f", in.Kind)
		w.simpleInstruction2("div ", in.Left, in.Right)

	case *ir.Rem:
		w.floatingSignedModifier('u', 's', "f", in.Kind)
		w.simpleInstruction2("rem ", in.Left, in.Right)

	// Bitwise binary instructions

	case *ir.Shl:
		w.simpleInstruction2("shl ", in.Left, in.Right)

	case *ir.Shr:
		name := "lshr "
		if in.SignExt {
			name = "ashr "
		}
		w.simpleInstruction2(name, in.Left, in.Right)

	case *ir.And:
		w.simpleInstruction2("and ", in.Left, in.Right)

	case *ir.Or:
		w.simpleInstruction2("or ", in.Left, in.Right)

	case *ir.Xor:
		w.simpleInstruction2("xor ", in.Left, in.Right)

	// Vector instructions

	case *ir.ExtractElement:
		w.simpleInstruction2("extractelement ", in.Value, in.Index)

	case *ir.InsertElement:
		w.simpleInstruction3("insertelement ", in.Value, in.Element, in.Index)

	case *ir.ShuffleVector:
		w.simpleInstruction3("shufflevector ", in.Value1, in.Value2, in.Mask)

	// Aggregate instructions

	case *ir.ExtractValue:
		w.string("extractvalue ")
		w.typValue(in.Value)

		for _, index := range in.Indices {
			w.string(", ")
			w.uint(uint64(index), 10)
		}

	case *ir.InsertValue:
		w.string("insertvalue ")
		w.typValue(in.Value)
		w.string(", ")
		w.typValue(in.Element)

		for _, index := range in.Indices {
			w.string(", ")
			w.uint(uint64(index), 10)
		}

	// Memory access and addressing instructions

	case *ir.Alloca:
		w.string("alloca ")
		w.typ(in.Typ)
		w.string(", i32 ")
		w.uint(uint64(in.Count), 10)

	case *ir.Load:
		w.string("load ")
		w.typ(in.Typ)
		w.string(", ")
		w.typValue(in.Pointer)

	case *ir.Store:
		w.string("store ")
		w.typValue(in.Value)
		w.string(", ")
		w.typValue(in.Pointer)

	case *ir.GetElementPtrConst:
		w.string("getelementptr ")
		w.typ(in.Typ)
		w.string(", ")
		w.typValue(in.Pointer)
		for _, index := range in.Indices {
			if index != math.MaxUint32 {
				w.string(", i32 ")
				w.uint(uint64(index), 10)
			}
		}

	case *ir.GetElementPtrDyn:
		w.string("getelementptr ")
		w.typ(in.Typ)
		w.string(", ")
		w.typValue(in.Pointer)
		for _, index := range in.Indices {
			if !core.IsNil(index) {
				w.string(", ")
				w.typValue(index)
			}
		}

	// Conversion instructions

	case *ir.Trunc:
		w.floatingModifier(true, in.Value)
		w.toInstruction("trunc ", in.Value, in.Typ)

	case *ir.Ext:
		w.floatingSignedModifier('z', 's', "fp", in.Kind)
		w.toInstruction("ext ", in.Value, in.Typ)

	case *ir.FpToInt:
		name := "fptoui "
		if in.Signed {
			name = "fptosi "
		}
		w.toInstruction(name, in.Value, in.Typ)

	case *ir.IntToFp:
		name := "uitofp "
		if in.Signed {
			name = "sitofp "
		}
		w.toInstruction(name, in.Value, in.Typ)

	case *ir.PtrToInt:
		w.toInstruction("ptrtoint ", in.Value, in.Typ)

	case *ir.IntToPtr:
		w.toInstruction("inttoptr ", in.Value, ir.Pointer)

	case *ir.BitCast:
		w.toInstruction("bitcast ", in.Value, in.Typ)

	// Other instructions

	case *ir.ICmp:
		prefix := "u"
		if in.Signed {
			prefix = "s"
		}
		w.cmpInstruction("icmp ", prefix, in.Op, in.Left, in.Right)

	case *ir.FCmp:
		prefix := "u"
		if in.Ordered {
			prefix = "o"
		}
		w.cmpInstruction("fcmp ", prefix, in.Op, in.Left, in.Right)

	case *ir.Phi:
		w.string("phi ")
		w.typ(in.Pairs[0].Value.Type())
		w.rune(' ')

		for i, pair := range in.Pairs {
			if i > 0 {
				w.string(", [ ")
			} else {
				w.string("[ ")
			}

			w.value(pair.Value)
			w.string(", %")
			w.labelName(pair.Block)

			w.string(" ]")
		}

	case *ir.Select:
		w.simpleInstruction3("select ", in.Condition, in.IfTrue, in.IfFalse)

	case *ir.Call:
		w.string("call ")
		w.typ(in.Signature.Returns)

		if _, direct := in.Callee.(*ir.Function); !direct || in.Signature.VarArgs {
			w.string(" (")

			for i, param := range in.Signature.Params {
				if i > 0 {
					w.string(", ")
				}
				w.typ(param)
			}

			if in.Signature.VarArgs {
				if len(in.Signature.Params) > 0 {
					w.string(", ...")
				} else {
					w.string("...")
				}
			}

			w.rune(')')
		}

		w.rune(' ')
		w.value(in.Callee)
		w.rune('(')

		for i, arg := range in.Args {
			if i > 0 {
				w.string(", ")
			}

			w.typ(arg.Type())

			if i == 0 && !core.IsNil(in.Signature.SRet) {
				w.string(" sret(")
				w.typ(in.Signature.SRet)
				w.string(") ")
			} else {
				w.rune(' ')
			}

			w.value(arg)
		}

		w.rune(')')

	// Debug instructions

	case *ir.DbgDeclare:
		w.string("  #dbg_declare(")
		w.typValue(in.Pointer)
		w.string(", !")
		w.uint(uint64(in.VariableRef.Value()), 10)
		w.string(", !DIExpression(), !")
		w.uint(uint64(in.LocationRef.Value()), 10)
		w.rune(')')

	// Invalid

	default:
		panic("llvm.writer.instruction() - Invalid instruction")
	}

	if in.Meta().Valid() {
		w.string(", !dbg !")
		w.uint(uint64(in.Meta().Value()), 10)
	}

	w.rune('\n')
}

// Utils

func (w *writer) label(block *ir.Block) {
	w.string("label %")
	w.labelName(block)
}

func (w *writer) labelName(block *ir.Block) {
	if name, ok := w.blockNameMap[block]; ok {
		w.identifier(name)
	} else {
		w.identifier(block.Name)
	}
}

func (w *writer) floatingModifier(fp bool, value ir.Value) {
	if isFloating(value.Type()) {
		if fp {
			w.string("fp")
		} else {
			w.rune('f')
		}
	}
}

func (w *writer) floatingSignedModifier(unsigned, signed rune, floating string, kind ir.DivKind) {
	switch kind {
	case ir.Unsigned:
		w.rune(unsigned)
	case ir.Signed:
		w.rune(signed)
	case ir.Floating:
		w.string(floating)
	}
}

func (w *writer) simpleInstruction2(name string, value1, value2 ir.Value) {
	w.string(name)
	w.typValue(value1)
	w.string(", ")
	w.value(value2)
}

func (w *writer) simpleInstruction3(name string, value1, value2, value3 ir.Value) {
	w.string(name)
	w.typValue(value1)
	w.string(", ")
	w.typValue(value2)
	w.string(", ")
	w.typValue(value3)
}

func (w *writer) toInstruction(name string, value ir.Value, typ ir.Type) {
	w.string(name)
	w.typValue(value)
	w.string(" to ")
	w.typ(typ)
}

func (w *writer) cmpInstruction(name, kindPrefix string, kind ir.CmpOp, left, right ir.Value) {
	w.string(name)

	if name == "fcmp " || (kind != ir.Eq && kind != ir.Ne) {
		w.string(kindPrefix)
	}

	switch kind {
	case ir.Eq:
		w.string("eq ")
	case ir.Ne:
		w.string("ne ")
	case ir.Gt:
		w.string("gt ")
	case ir.Ge:
		w.string("ge ")
	case ir.Lt:
		w.string("lt ")
	case ir.Le:
		w.string("le ")
	}

	w.typValue(left)
	w.string(", ")
	w.value(right)
}

func isFloating(typ ir.Type) bool {
	if typ, ok := typ.(*ir.SimpleType); ok {
		return typ.Kind == ir.FloatKind || typ.Kind == ir.DoubleKind
	}

	return false
}

func isVoid(typ ir.Type) bool {
	if typ, ok := typ.(*ir.SimpleType); ok {
		return typ.Kind == ir.VoidKind
	}

	return false
}
