package eval

import (
	"encoding/binary"
	"fireball/core"
	"fireball/ir"
	"iter"
	"math"
)

func (e *Evaluator) evalInst(inst ir.Instruction) (Register, bool) {
	switch inst := inst.(type) {

	// Terminator instructions

	case *ir.Br:
		e.next = inst.Label.First()
		return Register{}, false

	case *ir.BrCond:
		reg := e.getRegister(inst.Condition)

		if reg.Scalar == 1 {
			e.next = inst.IfTrue.First()
		} else {
			e.next = inst.IfFalse.First()
		}

		return Register{}, false

	// Unary instructions

	case *ir.FNeg:
		reg := e.getRegister(inst.Value)

		if inst.Value.Type().(*ir.SimpleType).Kind == ir.FloatKind {
			return Float32Reg(-reg.Float32()), true
		}

		return Float64Reg(-reg.Float64()), true

	// Binary instructions

	case *ir.IAdd:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar

		return Register{Scalar: l + r}, true

	case *ir.FAdd:
		l := e.getRegister(inst.Left)
		r := e.getRegister(inst.Right)

		if inst.Type().(*ir.SimpleType).Kind == ir.DoubleKind {
			return Float64Reg(l.Float64() + r.Float64()), true
		}

		return Float32Reg(l.Float32() + r.Float32()), true

	case *ir.ISub:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar

		return Register{Scalar: l - r}, true

	case *ir.FSub:
		l := e.getRegister(inst.Left)
		r := e.getRegister(inst.Right)

		if inst.Type().(*ir.SimpleType).Kind == ir.DoubleKind {
			return Float64Reg(l.Float64() - r.Float64()), true
		}

		return Float32Reg(l.Float32() - r.Float32()), true

	case *ir.IMul:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar

		return Register{Scalar: l * r}, true

	case *ir.FMul:
		l := e.getRegister(inst.Left)
		r := e.getRegister(inst.Right)

		if inst.Type().(*ir.SimpleType).Kind == ir.DoubleKind {
			return Float64Reg(l.Float64() * r.Float64()), true
		}

		return Float32Reg(l.Float32() * r.Float32()), true

	case *ir.IDiv:
		left := e.getRegister(inst.Left)
		right := e.getRegister(inst.Right)

		if right.Scalar == 0 {
			e.panic("tried to divide by zero")
		}

		if inst.Kind == ir.Signed {
			return Register{Scalar: uint64(int64(left.Scalar) / int64(right.Scalar))}, true
		}

		return Register{Scalar: left.Scalar / right.Scalar}, true

	case *ir.FDiv:
		l := e.getRegister(inst.Left)
		r := e.getRegister(inst.Right)

		if inst.Type().(*ir.SimpleType).Kind == ir.DoubleKind {
			right := r.Float64()
			if right == 0 {
				e.panic("tried to divide by zero")
			}

			return Float64Reg(l.Float64() / right), true
		}

		right := r.Float32()
		if right == 0 {
			e.panic("tried to divide by zero")
		}

		return Float32Reg(l.Float32() / right), true

	case *ir.IRem:
		left := e.getRegister(inst.Left)
		right := e.getRegister(inst.Right)

		if inst.Kind == ir.Signed {
			return Register{Scalar: uint64(int64(left.Scalar) % int64(right.Scalar))}, true
		}

		return Register{Scalar: left.Scalar % right.Scalar}, true

	case *ir.FRem:
		left := e.getRegister(inst.Left)
		right := e.getRegister(inst.Right)

		if inst.Type().(*ir.SimpleType).Kind == ir.DoubleKind {
			return Float64Reg(math.Mod(left.Float64(), right.Float64())), true
		}

		return Float32Reg(float32(math.Mod(float64(left.Float32()), float64(right.Float32())))), true

	// Bitwise binary instructions

	case *ir.Shl:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar

		return Register{Scalar: l << r}, true

	case *ir.Shr:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar

		if inst.SignExt {
			return Register{Scalar: uint64(int64(l) >> r)}, true
		}

		return Register{Scalar: l >> r}, true

	case *ir.And:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar

		return Register{Scalar: l & r}, true

	case *ir.Or:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar

		return Register{Scalar: l | r}, true

	case *ir.Xor:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar

		return Register{Scalar: l ^ r}, true

	// Vector instructions

	case *ir.ExtractElement:
		val := e.getRegister(inst.Value)
		idx := e.getRegister(inst.Index)

		return val.Aggregate[idx.Scalar], true

	case *ir.InsertElement:
		val := e.getRegister(inst.Value)
		idx := e.getRegister(inst.Index)
		ele := e.getRegister(inst.Element)

		val.Aggregate[idx.Scalar] = ele
		return Register{}, false

	case *ir.ShuffleVector:
		val1 := e.getRegister(inst.Value1).Aggregate
		val2 := e.getRegister(inst.Value2).Aggregate
		mask := e.getRegister(inst.Mask).Aggregate

		aggregate := make([]Register, inst.Mask.Type().(*ir.VectorType).Length)

		for i := range len(aggregate) {
			idx := mask[i].Scalar

			if idx < uint64(len(val1)) {
				aggregate[i] = val1[idx]
			} else {
				aggregate[i] = val2[idx-uint64(len(val1))]
			}
		}

		return Register{Aggregate: aggregate}, true

	// Aggregate instructions

	case *ir.ExtractValue:
		val := e.getRegister(inst.Value)

		for _, idx := range inst.Indices {
			val = val.Aggregate[idx]
		}

		return val, true

	case *ir.InsertValue:
		val := e.getRegister(inst.Value)
		ele := e.getRegister(inst.Element)

		root := val.ShallowClone()
		current := &root

		for _, idx := range inst.Indices[:len(inst.Indices)-1] {
			current.Aggregate[idx] = current.Aggregate[idx].ShallowClone()
			current = &current.Aggregate[idx]
		}

		current.Aggregate[inst.Indices[len(inst.Indices)-1]] = ele

		return root, true

	// Memory access and addressing instructions

	case *ir.Alloca:
		addr := e.push(uint64(inst.Typ.Info().Size * inst.Count))
		return Register{Scalar: addr}, true

	case *ir.Load:
		ptr := e.getRegister(inst.Pointer)

		if ptr.Scalar == 0 {
			e.panic("tried to read a null pointer")
		}

		return e.readRegister(ptr.Scalar, inst.Typ), true

	case *ir.Store:
		ptr := e.getRegister(inst.Pointer)
		val := e.getRegister(inst.Value)

		if ptr.Scalar == 0 {
			e.panic("tried to write to a null pointer")
		}

		e.writeRegister(ptr.Scalar, inst.Value.Type(), val)
		return Register{}, false

	case *ir.GetElementPtrConst:
		addr := e.getRegister(inst.Pointer).Scalar

		var indices [4]int64
		count := 0

		for i, idx := range inst.Indices {
			if idx == math.MaxUint32 {
				break
			}

			indices[i] = int64(idx)
			count++
		}

		return calcGep(addr, inst.Typ, indices[:count]), true

	case *ir.GetElementPtrDyn:
		addr := e.getRegister(inst.Pointer).Scalar

		var indices [4]int64
		count := 0

		for i, idx := range inst.Indices {
			if core.IsNil(idx) {
				break
			}

			indices[i] = int64(e.getRegister(idx).Scalar)
			count++
		}

		return calcGep(addr, inst.Typ, indices[:count]), true

	// Conversion instructions

	case *ir.ITrunc:
		val := e.getRegister(inst.Value).Scalar

		mask := (uint64(1) << inst.Typ.(*ir.IntegerType).Bits) - 1
		return Register{Scalar: val & mask}, true

	case *ir.FTrunc:
		val := e.getRegister(inst.Value).Float64()

		return Float32Reg(float32(val)), true

	case *ir.IExt:
		val := e.getRegister(inst.Value)

		// Integer
		if inst.Kind == ir.Unsigned {
			return val, true
		}

		srcBits := inst.Value.Type().(*ir.IntegerType).Bits
		leftShifted := val.Scalar << (64 - srcBits)
		arithmeticRightShifted := int64(leftShifted) >> (64 - srcBits)

		return Register{Scalar: uint64(arithmeticRightShifted)}, true

	case *ir.FExt:
		val := e.getRegister(inst.Value).Float32()

		return Float64Reg(float64(val)), true

	case *ir.FpToInt:
		val := e.getRegister(inst.Value)
		mask := (uint64(1) << inst.Typ.(*ir.IntegerType).Bits) - 1

		if inst.Signed {
			return Register{Scalar: uint64(int64(regGetFloat(val, inst.Value.Type()))) & mask}, true
		}

		return Register{Scalar: uint64(regGetFloat(val, inst.Value.Type())) & mask}, true

	case *ir.IntToFp:
		val := e.getRegister(inst.Value)

		var floatVal float64

		if inst.Signed {
			floatVal = float64(int64(val.Scalar))
		} else {
			floatVal = float64(val.Scalar)
		}

		if inst.Typ.(*ir.SimpleType).Kind == ir.DoubleKind {
			return Float64Reg(floatVal), true
		}

		return Float32Reg(float32(floatVal)), true

	case *ir.PtrToInt:
		return e.getRegister(inst.Value), true

	case *ir.IntToPtr:
		return e.getRegister(inst.Value), true

	case *ir.BitCast:
		return e.getRegister(inst.Value), true

	// Other instructions

	case *ir.ICmp:
		l := e.getRegister(inst.Left).Scalar
		r := e.getRegister(inst.Right).Scalar
		var res bool

		switch inst.Op {
		case ir.Eq:
			res = l == r
		case ir.Ne:
			res = l != r
		case ir.Gt:
			if inst.Signed {
				res = int64(l) > int64(r)
			} else {
				res = l > r
			}
		case ir.Ge:
			if inst.Signed {
				res = int64(l) >= int64(r)
			} else {
				res = l >= r
			}
		case ir.Lt:
			if inst.Signed {
				res = int64(l) < int64(r)
			} else {
				res = l < r
			}
		case ir.Le:
			if inst.Signed {
				res = int64(l) <= int64(r)
			} else {
				res = l <= r
			}
		default:
			panic("ir.eval.Evaluator.evalInst() - ICmp: invalid operation")
		}

		if res {
			return Register{Scalar: 1}, true
		}

		return Register{Scalar: 0}, true

	case *ir.FCmp:
		l := regGetFloat(e.getRegister(inst.Left), inst.Left.Type())
		r := regGetFloat(e.getRegister(inst.Right), inst.Right.Type())
		var res bool

		switch inst.Op {
		case ir.Eq:
			res = l == r
		case ir.Ne:
			res = l != r
		case ir.Gt:
			res = l > r
		case ir.Ge:
			res = l >= r
		case ir.Lt:
			res = l < r
		case ir.Le:
			res = l <= r
		default:
			panic("ir.eval.Evaluator.evalInst() - FCmp: invalid operation")
		}

		if !inst.Ordered {
			if math.IsNaN(l) || math.IsNaN(r) {
				res = true
			}
		}

		if res {
			return Register{Scalar: 1}, true
		}

		return Register{Scalar: 0}, true

	case *ir.Phi:
		previousBlock := e.prev.Block()

		for _, pair := range inst.Pairs {
			if pair.Block == previousBlock {
				return e.getRegister(pair.Value), true
			}
		}

		panic("ir.eval.Evaluator.evalInst() - Phi: node didn't match incoming block!")

	case *ir.Select:
		val := e.getRegister(inst.Condition)

		if val.Scalar == 1 {
			return e.getRegister(inst.IfTrue), true
		}

		return e.getRegister(inst.IfFalse), true

	case *ir.Call:
		callee := e.getRegister(inst.Callee)

		f, ok := e.funcByAddr[callee.Scalar]
		if !ok {
			panic("ir.eval.Evaluator.evalInst() - Call: invalid function pointer")
		}

		frm := e.call(f)

		for i, param := range f.ParamValues {
			frm.regs[param] = e.getRegister(inst.Args[i])
		}

		return Register{}, false

	// Debug instructions

	case *ir.DbgDeclare:
		return Register{}, false

	default:
		panic("ir.eval.Evaluator.evalInst() - Invalid instruction")
	}
}

// Utils

func calcGep(addr uint64, typ ir.Type, indices []int64) Register {
	for i, idx := range indices {
		if i == 0 {
			addr += uint64(idx * int64(typ.Info().Size))
			continue
		}

		switch t := typ.(type) {
		case *ir.ArrayType:
			elemSize := int64(t.Element.Info().Size)
			addr += uint64(idx * elemSize)
			typ = t.Element

		case *ir.StructType:
			ranges := getStructFieldRanges(t)
			addr += seqAt(ranges, int(idx)).offset

			typ = t.Fields[idx].Type

		case *ir.RefStructType:
			ranges := getStructFieldRanges(&t.Struct)
			addr += seqAt(ranges, int(idx)).offset

			typ = t.Struct.Fields[idx].Type

		default:
			panic("ir.eval.Evaluator.calcGep() - Invalid type")
		}
	}

	return Register{Scalar: addr}
}

func seqAt[T any](seq iter.Seq2[int, T], idx int) T {
	for i, value := range seq {
		if i == idx {
			return value
		}
	}

	panic("ir.eval.seqAt() - Sequence was too small")
}

func (e *Evaluator) readRegister(addr uint64, typ ir.Type) Register {
	switch typ := typ.(type) {
	case *ir.SimpleType, *ir.IntegerType:
		end := addr + uint64(typ.Info().Size)

		if end >= uint64(len(e.memory)) {
			e.panic("tried to read from an invalid memory location")
		}

		src := e.memory[addr:end]

		switch typ.Info().Size {
		case 1:
			return Register{Scalar: uint64(src[0])}
		case 2:
			return Register{Scalar: uint64(binary.NativeEndian.Uint16(src))}
		case 4:
			return Register{Scalar: uint64(binary.NativeEndian.Uint32(src))}
		case 8:
			return Register{Scalar: binary.NativeEndian.Uint64(src)}

		default:
			panic("ir.eval.Evaluator.readRegister() - SimpleType, IntegerType: invalid size")
		}

	case *ir.VectorType:
		elemSize := uint64(typ.Element.Info().Size)
		aggregate := make([]Register, typ.Length)

		for i := range typ.Length {
			aggregate[i] = e.readRegister(addr+uint64(i)*elemSize, typ.Element)
		}

		return Register{Aggregate: aggregate}

	case *ir.ArrayType:
		elemSize := uint64(typ.Element.Info().Size)
		aggregate := make([]Register, typ.Length)

		for i := range typ.Length {
			aggregate[i] = e.readRegister(addr+uint64(i)*elemSize, typ.Element)
		}

		return Register{Aggregate: aggregate}

	case *ir.StructType:
		aggregate := make([]Register, len(typ.Fields))

		for i, r := range getStructFieldRanges(typ) {
			aggregate[i] = e.readRegister(addr+r.offset, typ.Fields[i].Type)
		}

		return Register{Aggregate: aggregate}

	case *ir.RefStructType:
		return e.readRegister(addr, &typ.Struct)

	default:
		panic("ir.eval.Evaluator.readRegister() - Invalid type")
	}
}

func (e *Evaluator) writeRegister(addr uint64, typ ir.Type, reg Register) {
	switch typ := typ.(type) {
	case *ir.SimpleType, *ir.IntegerType:
		end := addr + uint64(typ.Info().Size)

		if end >= uint64(len(e.memory)) {
			e.panic("tried to write to an invalid memory location")
		}

		dst := e.memory[addr:end]

		switch typ.Info().Size {
		case 1:
			dst[0] = uint8(reg.Scalar)
		case 2:
			binary.NativeEndian.PutUint16(dst, uint16(reg.Scalar))
		case 4:
			binary.NativeEndian.PutUint32(dst, uint32(reg.Scalar))
		case 8:
			binary.NativeEndian.PutUint64(dst, reg.Scalar)

		default:
			panic("ir.eval.Evaluator.writeRegister() - SimpleType, IntegerType: invalid size")
		}

	case *ir.VectorType:
		elemSize := uint64(typ.Element.Info().Size)

		for i, elem := range reg.Aggregate {
			e.writeRegister(addr+uint64(i)*elemSize, typ.Element, elem)
		}

	case *ir.ArrayType:
		elemSize := uint64(typ.Element.Info().Size)

		for i, elem := range reg.Aggregate {
			e.writeRegister(addr+uint64(i)*elemSize, typ.Element, elem)
		}

	case *ir.StructType:
		for i, r := range getStructFieldRanges(typ) {
			e.writeRegister(addr+r.offset, typ.Fields[i].Type, reg.Aggregate[i])
		}

	case *ir.RefStructType:
		e.writeRegister(addr, &typ.Struct, reg)

	default:
		panic("ir.eval.Evaluator.writeRegister() - Invalid type")
	}
}

func regGetFloat(reg Register, typ ir.Type) float64 {
	if typ.(*ir.SimpleType).Kind == ir.DoubleKind {
		return reg.Float64()
	}

	return float64(reg.Float32())
}
