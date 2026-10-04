package amd64

import (
	"encoding/binary"
	"fireball/backend/amd64/asm"
	"fireball/backend/obj"
	"fireball/core"
	"fireball/ir"
	"math"
	"strconv"
	"strings"
)

type floatMode uint8

const (
	none floatMode = iota
	f32
	f64
)

func (b *backend) GenerateInstruction(inst ir.Instruction) {
	switch inst := inst.(type) {

	// Terminator instructions

	case *ir.Ret:
		if !core.IsNil(inst.Value) {
			size := inst.Value.Type().Info().Size

			if size == 0 {
				// Zero-sized return value: no-op
			} else if !strings.Contains(b.module.Triple, "windows") && ir.IsAggregate(inst.Value.Type()) && isPureFloat(inst.Value.Type()) {
				// Pure float aggregate: returned in XMM0 (and XMM1 if > 8 bytes)
				b.LoadAddress(inst.Value, asm.RCX)

				if size <= 4 {
					b.asm.Movss(asm.XMM0, asm.RegDisp(asm.RCX, 0))
				} else {
					b.asm.Movsd(asm.XMM0, asm.RegDisp(asm.RCX, 0))
				}

				if size > 8 {
					if size <= 12 {
						b.asm.Movss(asm.XMM1, asm.RegDisp(asm.RCX, 8))
					} else {
						b.asm.Movsd(asm.XMM1, asm.RegDisp(asm.RCX, 8))
					}
				}
			} else if size > 8 && size <= 16 {
				b.LoadAddress(inst.Value, asm.RCX)
				b.asm.Mov(asm.RAX, asm.RegDisp(asm.RCX, 0))
				b.LoadUpTo8(asm.RDX, asm.RCX, 8, size-8)
			} else if ir.IsAggregate(inst.Value.Type()) && size <= 8 {
				b.LoadAddress(inst.Value, asm.RCX)
				b.LoadUpTo8(asm.RAX, asm.RCX, 0, size)
			} else if size <= 8 {
				b.LoadOperand(inst.Value, asm.RAX, asm.XMM0)
			} else {
				panic("amd64.backend.GenerateInstruction(): ir.Ret - Invalid size")
			}
		}

		b.asm.Jmp(b.epilogue)

	case *ir.Br:
		b.ResolvePhis(inst.Block(), inst.Label)
		b.asm.Jmp(b.blockLabels[inst.Label])

	case *ir.BrCond:
		b.LoadOperand(inst.Condition, asm.RAX, 0)
		b.asm.Test32(asm.RAX, asm.RAX)

		if !b.HasPhis(inst.IfTrue) && !b.HasPhis(inst.IfFalse) {
			b.asm.Jne(b.blockLabels[inst.IfTrue])
			b.asm.Jmp(b.blockLabels[inst.IfFalse])
		} else {
			falseLabel := b.asm.Label(nil)
			b.asm.Je(falseLabel)

			// True edge
			b.ResolvePhis(inst.Block(), inst.IfTrue)
			b.asm.Jmp(b.blockLabels[inst.IfTrue])

			// False edge
			b.asm.Bind(falseLabel)
			b.ResolvePhis(inst.Block(), inst.IfFalse)
			b.asm.Jmp(b.blockLabels[inst.IfFalse])
		}

	// Unary instructions

	case *ir.FNeg:
		mode := b.LoadOperand(inst.Value, 0, asm.XMM0)

		if mode == f32 {
			b.asm.Mov(asm.RAX, int64(0x80000000))
			b.asm.MovdToXmm(asm.XMM1, asm.RAX)
			b.asm.Xorps(asm.XMM0, asm.XMM1)
		} else {
			b.asm.Mov(asm.RAX, int64(math.MinInt64))
			b.asm.MovqToXmm(asm.XMM1, asm.RAX)
			b.asm.Xorpd(asm.XMM0, asm.XMM1)
		}

		b.StoreResult(inst, 0, asm.XMM0)

	// Binary instructions

	case *ir.IAdd:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		switch inst.Type().Info().Size {
		case 1:
			b.asm.Add32(asm.RAX, asm.RCX)
			b.asm.And32(asm.RAX, int32(0xFF))
		case 2:
			b.asm.Add32(asm.RAX, asm.RCX)
			b.asm.And32(asm.RAX, int32(0xFFFF))
		case 4:
			b.asm.Add32(asm.RAX, asm.RCX)
		case 8:
			b.asm.Add(asm.RAX, asm.RCX)
		default:
			panic("amd64.backend.GenerateInstruction(): ir.IAdd - Invalid size")
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.FAdd:
		mode := b.LoadOperand(inst.Left, 0, asm.XMM0)
		b.LoadOperand(inst.Right, 0, asm.XMM1)

		if mode == f32 {
			b.asm.Addss(asm.XMM0, asm.XMM1)
		} else {
			b.asm.Addsd(asm.XMM0, asm.XMM1)
		}

		b.StoreResult(inst, 0, asm.XMM0)

	case *ir.ISub:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		switch inst.Type().Info().Size {
		case 1:
			b.asm.Sub32(asm.RAX, asm.RCX)
			b.asm.And32(asm.RAX, int32(0xFF))
		case 2:
			b.asm.Sub32(asm.RAX, asm.RCX)
			b.asm.And32(asm.RAX, int32(0xFFFF))
		case 4:
			b.asm.Sub32(asm.RAX, asm.RCX)
		case 8:
			b.asm.Sub(asm.RAX, asm.RCX)
		default:
			panic("amd64.backend.GenerateInstruction(): ir.ISub - Invalid size")
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.FSub:
		mode := b.LoadOperand(inst.Left, 0, asm.XMM0)
		b.LoadOperand(inst.Right, 0, asm.XMM1)

		if mode == f32 {
			b.asm.Subss(asm.XMM0, asm.XMM1)
		} else {
			b.asm.Subsd(asm.XMM0, asm.XMM1)
		}

		b.StoreResult(inst, 0, asm.XMM0)

	case *ir.IMul:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		switch inst.Type().Info().Size {
		case 1, 2, 4:
			b.asm.Imul32(asm.RAX, asm.RCX)
		case 8:
			b.asm.Imul(asm.RAX, asm.RCX)
		default:
			panic("amd64.backend.GenerateInstruction(): ir.IMul - Invalid size")
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.FMul:
		mode := b.LoadOperand(inst.Left, 0, asm.XMM0)
		b.LoadOperand(inst.Right, 0, asm.XMM1)

		if mode == f32 {
			b.asm.Mulss(asm.XMM0, asm.XMM1)
		} else {
			b.asm.Mulsd(asm.XMM0, asm.XMM1)
		}

		b.StoreResult(inst, 0, asm.XMM0)

	case *ir.IDiv:
		b.PrepareIDiv(inst.Left, inst.Right, inst.Type(), inst.Kind)

		// Quotient is in RAX
		b.StoreResult(inst, asm.RAX, 0)

	case *ir.FDiv:
		mode := b.LoadOperand(inst.Left, 0, asm.XMM0)
		b.LoadOperand(inst.Right, 0, asm.XMM1)

		if mode == f32 {
			b.asm.Divss(asm.XMM0, asm.XMM1)
		} else {
			b.asm.Divsd(asm.XMM0, asm.XMM1)
		}

		b.StoreResult(inst, 0, asm.XMM0)

	case *ir.IRem:
		b.PrepareIDiv(inst.Left, inst.Right, inst.Type(), inst.Kind)

		// Remainder is in RDX
		b.StoreResult(inst, asm.RDX, 0)

	case *ir.FRem:
		mode := b.LoadOperand(inst.Left, 0, asm.XMM0)
		b.LoadOperand(inst.Right, 0, asm.XMM1)

		name := "fmod"
		if mode == f32 {
			name = "fmodf"
		}

		b.asm.Call(b.GetFuncSymbol(name))
		b.StoreResult(inst, 0, asm.XMM0)

	// Bitwise binary instructions

	case *ir.Shl:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		switch inst.Type().Info().Size {
		case 1, 2, 4:
			b.asm.ShlCl32(asm.RAX)
		case 8:
			b.asm.ShlCl(asm.RAX)
		default:
			panic("amd64.backend.GenerateInstruction(): ir.Shl - Invalid size")
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.Shr:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		size := inst.Type().Info().Size

		if inst.SignExt {
			switch size {
			case 1:
				b.asm.Movsx8(asm.RAX, asm.RAX)
			case 2:
				b.asm.Movsx16(asm.RAX, asm.RAX)
			}

			switch size {
			case 1, 2, 4:
				b.asm.SarCl32(asm.RAX)
			case 8:
				b.asm.SarCl(asm.RAX)
			default:
				panic("amd64.backend.GenerateInstruction(): ir.Shr - Invalid size (sign-ext)")
			}
		} else {
			switch size {
			case 1, 2, 4:
				b.asm.ShrCl32(asm.RAX)
			case 8:
				b.asm.ShrCl(asm.RAX)
			default:
				panic("amd64.backend.GenerateInstruction(): ir.Shr - Invalid size (zero-ext)")
			}
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.And:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		switch inst.Type().Info().Size {
		case 1:
			b.asm.And32(asm.RAX, asm.RCX)
		case 2:
			b.asm.And32(asm.RAX, asm.RCX)
		case 4:
			b.asm.And32(asm.RAX, asm.RCX)
		case 8:
			b.asm.And(asm.RAX, asm.RCX)
		default:
			panic("amd64.backend.GenerateInstruction(): ir.IAnd - Invalid size")
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.Or:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		switch inst.Type().Info().Size {
		case 1:
			b.asm.Or32(asm.RAX, asm.RCX)
			b.asm.And32(asm.RAX, int32(0xFF))
		case 2:
			b.asm.Or32(asm.RAX, asm.RCX)
			b.asm.And32(asm.RAX, int32(0xFFFF))
		case 4:
			b.asm.Or32(asm.RAX, asm.RCX)
		case 8:
			b.asm.Or(asm.RAX, asm.RCX)
		default:
			panic("amd64.backend.GenerateInstruction(): ir.IOr - Invalid size")
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.Xor:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		switch inst.Type().Info().Size {
		case 1:
			b.asm.Xor32(asm.RAX, asm.RCX)
			b.asm.And32(asm.RAX, int32(0xFF))
		case 2:
			b.asm.Xor32(asm.RAX, asm.RCX)
			b.asm.And32(asm.RAX, int32(0xFFFF))
		case 4:
			b.asm.Xor32(asm.RAX, asm.RCX)
		case 8:
			b.asm.Xor(asm.RAX, asm.RCX)
		default:
			panic("amd64.backend.GenerateInstruction(): ir.IXor - Invalid size")
		}

		b.StoreResult(inst, asm.RAX, 0)

	// Vector instructions

	case *ir.ExtractElement:
		panic("amd64.backend.GenerateInstruction(): ir.ExtractElement - Not implemented")

	case *ir.InsertElement:
		panic("amd64.backend.GenerateInstruction(): ir.InsertElement - Not implemented")

	case *ir.ShuffleVector:
		panic("amd64.backend.GenerateInstruction(): ir.ShuffleVector - Not implemented")

	// Aggregate instructions

	case *ir.ExtractValue:
		// Load the base address of the source aggregate into RAX
		b.LoadAddress(inst.Value, asm.RAX)

		// Compute the offset and type of the extracted element
		fieldOffset, elemType := b.CalculateAggregateOffset(inst.Value.Type(), inst.Indices)
		fieldSize := elemType.Info().Size

		// Extract the element
		if mode := getFloatMode(elemType); mode != none {
			if mode == f32 {
				b.asm.Movss(asm.XMM0, asm.RegDisp(asm.RAX, int32(fieldOffset)))
			} else {
				b.asm.Movsd(asm.XMM0, asm.RegDisp(asm.RAX, int32(fieldOffset)))
			}

			b.StoreResult(inst, 0, asm.XMM0)
		} else if fieldSize == 0 {
			// Zero-sized element (e.g. empty struct): no-op
		} else if fieldSize <= 8 {
			b.LoadUpTo8(asm.RCX, asm.RAX, int32(fieldOffset), fieldSize)
			b.StoreResult(inst, asm.RCX, 0)
		} else {
			dstOff := b.layout.offsets[inst]
			b.asm.Lea(asm.RDX, asm.RegDisp(asm.RBP, dstOff))
			b.asm.Lea(asm.RCX, asm.RegDisp(asm.RAX, int32(fieldOffset)))
			b.CopyMemory(asm.RDX, asm.RCX, fieldSize)
		}

	case *ir.InsertValue:
		dstOff := b.layout.offsets[inst]
		aggSize := inst.Type().Info().Size

		// Get destination pointer in RAX (points to this instruction's stack slot)
		b.asm.Lea(asm.RAX, asm.RegDisp(asm.RBP, dstOff))

		// Clone the existing aggregate into the destination slot
		b.LoadAddress(inst.Value, asm.RCX)
		b.CopyMemory(asm.RAX, asm.RCX, aggSize)

		// Compute offset of the field to overwrite
		fieldOffset, elemType := b.CalculateAggregateOffset(inst.Type(), inst.Indices)
		fieldSize := elemType.Info().Size
		fieldDest := asm.RegDisp(asm.RAX, int32(fieldOffset))

		// Overwrite the field with the new value
		if mode := getFloatMode(elemType); mode != none {
			b.LoadOperand(inst.Element, 0, asm.XMM0)

			if mode == f32 {
				b.asm.Movss(fieldDest, asm.XMM0)
			} else {
				b.asm.Movsd(fieldDest, asm.XMM0)
			}
		} else if fieldSize <= 8 {
			if fieldSize > 0 {
				b.LoadOperand(inst.Element, asm.RCX, 0)
				b.StoreUpTo8(asm.RAX, int32(fieldOffset), asm.RCX, fieldSize)
			}
		} else {
			b.LoadAddress(inst.Element, asm.RDX)
			b.asm.Lea(asm.RCX, fieldDest)
			b.CopyMemory(asm.RCX, asm.RDX, fieldSize)
		}

	// Memory access and addressing instructions

	case *ir.Alloca:
		// nop, alloca just reserves space in the layout calculation pass

	case *ir.Load:
		// Load pointer into RCX
		b.LoadOperand(inst.Pointer, asm.RCX, 0)

		// Floating point type
		if mode := getFloatMode(inst.Type()); mode != none {
			if mode == f32 {
				b.asm.Movss(asm.XMM0, asm.Ptr(asm.RCX))
			} else {
				b.asm.Movsd(asm.XMM0, asm.Ptr(asm.RCX))
			}

			b.StoreResult(inst, 0, asm.XMM0)
			break
		}

		// Integer type
		size := inst.Type().Info().Size
		isInt := false

		switch t := inst.Type().(type) {
		case *ir.SimpleType:
			isInt = t.Kind == ir.PointerKind
		case *ir.IntegerType:
			isInt = true
		}

		if isInt {
			switch size {
			case 1:
				b.asm.Movzx8(asm.RAX, asm.Ptr(asm.RCX))
				b.StoreResult(inst, asm.RAX, 0)
			case 2:
				b.asm.Movzx16(asm.RAX, asm.Ptr(asm.RCX))
				b.StoreResult(inst, asm.RAX, 0)
			case 4:
				b.asm.Mov32(asm.RAX, asm.Ptr(asm.RCX))
				b.StoreResult(inst, asm.RAX, 0)
			case 8:
				b.asm.Mov(asm.RAX, asm.Ptr(asm.RCX))
				b.StoreResult(inst, asm.RAX, 0)

			default:
				panic("amd64.backend.GenerateInstruction(): ir.Load - Invalid integer size")
			}

			break
		}

		// Aggregate type
		off := b.layout.offsets[inst]
		b.asm.Lea(asm.RAX, asm.RegDisp(asm.RBP, off))
		b.CopyMemory(asm.RAX, asm.RCX, size)

	case *ir.Store:
		// Load pointer into RCX
		b.LoadOperand(inst.Pointer, asm.RCX, 0)

		// Zero initializer / Null
		switch inst.Value.(type) {
		case *ir.ZeroInitializer, *ir.Null:
			b.ZeroMemory(asm.RCX, inst.Value.Type().Info().Size)
			return
		}

		// Floating point type
		if mode := getFloatMode(inst.Value.Type()); mode != none {
			b.LoadOperand(inst.Value, 0, asm.XMM0)

			if mode == f32 {
				b.asm.Movss(asm.Ptr(asm.RCX), asm.XMM0)
			} else {
				b.asm.Movsd(asm.Ptr(asm.RCX), asm.XMM0)
			}

			break
		}

		// Integer / Pointer type
		size := inst.Value.Type().Info().Size
		isInt := false

		switch t := inst.Value.Type().(type) {
		case *ir.SimpleType:
			isInt = t.Kind == ir.PointerKind
		case *ir.IntegerType:
			isInt = true
		}

		if isInt {
			switch size {
			case 1:
				b.LoadOperand(inst.Value, asm.RAX, 0)
				b.asm.Mov8(asm.Ptr(asm.RCX), asm.RAX)
			case 2:
				b.LoadOperand(inst.Value, asm.RAX, 0)
				b.asm.Mov16(asm.Ptr(asm.RCX), asm.RAX)
			case 4:
				b.LoadOperand(inst.Value, asm.RAX, 0)
				b.asm.Mov32(asm.Ptr(asm.RCX), asm.RAX)
			case 8:
				b.LoadOperand(inst.Value, asm.RAX, 0)
				b.asm.Mov(asm.Ptr(asm.RCX), asm.RAX)

			default:
				panic("amd64.backend.GenerateInstruction(): ir.Store - Invalid integer size")
			}

			break
		}

		// Aggregate type
		b.LoadAddress(inst.Value, asm.RAX)
		b.CopyMemory(asm.RCX, asm.RAX, size)

	case *ir.GetElementPtrConst:
		b.LoadOperand(inst.Pointer, asm.RAX, 0)

		offset := b.CalculateGepConstOffset(inst.Typ, inst.Indices)
		if offset != 0 {
			b.asm.Add(asm.RAX, int32(offset))
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.GetElementPtrDyn:
		b.LoadOperand(inst.Pointer, asm.RAX, 0)

		current := inst.Typ

		for i, indexVal := range inst.Indices {
			if core.IsNil(indexVal) {
				break
			}

			var nextType ir.Type
			var stride uint32 = 0
			var constFieldOffset uint32 = 0

			if i == 0 {
				// Index 0: pointer arithmetic
				stride = inst.Typ.Info().Size
				nextType = inst.Typ
			} else {
				switch t := current.(type) {
				case *ir.ArrayType:
					stride = t.Element.Info().Size
					nextType = t.Element

				case *ir.VectorType:
					stride = t.Element.Info().Size
					nextType = t.Element

				case ir.StructLikeType:
					fieldIdx := uint32(indexVal.(*ir.Integer).Value.TwosComplement())

					for k, range_ := range ir.GetStructFieldRanges(t) {
						if uint32(k) == fieldIdx {
							constFieldOffset = uint32(range_.Offset)
							break
						}
					}

					switch t := t.(type) {
					case *ir.StructType:
						nextType = t.Fields[fieldIdx].Type
					case *ir.RefStructType:
						nextType = t.Struct.Fields[fieldIdx].Type
					}
				}
			}

			// Apply the offset for this level
			if constInt, ok := indexVal.(*ir.Integer); ok {
				idx := uint32(constInt.Value.TwosComplement())
				var totalOffset int32

				if stride > 0 {
					totalOffset = int32(idx * stride)
				} else {
					totalOffset = int32(constFieldOffset)
				}

				if totalOffset != 0 {
					b.asm.Add(asm.RAX, totalOffset)
				}
			} else {
				// Dynamic runtime value (LoadOperand preserves RAX)
				b.LoadOperand(indexVal, asm.RCX, 0)

				switch indexVal.Type().Info().Size {
				case 1:
					b.asm.Movsx8(asm.RCX, asm.RCX)
				case 2:
					b.asm.Movsx16(asm.RCX, asm.RCX)
				case 4:
					b.asm.Movsxd(asm.RCX, asm.RCX)
				}

				// Scale index by stride: RCX = RCX * stride
				switch stride {
				case 1:
					// nop
				case 2:
					b.asm.Add(asm.RCX, asm.RCX)
				default:
					b.asm.Mov(asm.RDX, int64(stride))
					b.asm.Imul(asm.RCX, asm.RDX)
				}

				// Accumulate: RAX = RAX + RCX
				b.asm.Add(asm.RAX, asm.RCX)
			}

			current = nextType
		}

		b.StoreResult(inst, asm.RAX, 0)

	// Conversion instructions

	case *ir.ITrunc:
		b.LoadOperand(inst.Value, asm.RAX, 0)

		// b.Store() only stores the exact sub-region of the register,
		// so basically truncating it

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.FTrunc:
		b.LoadOperand(inst.Value, 0, asm.XMM0)
		b.asm.Cvtsd2ss(asm.XMM0, asm.XMM0)
		b.StoreResult(inst, 0, asm.XMM0)

	case *ir.IExt:
		b.LoadOperand(inst.Value, asm.RAX, 0)

		if inst.Kind == ir.Signed {
			switch inst.Value.Type().Info().Size {
			case 1:
				b.asm.Movsx8(asm.RAX, asm.RAX)
			case 2:
				b.asm.Movsx16(asm.RAX, asm.RAX)
			case 4:
				b.asm.Movsxd(asm.RAX, asm.RAX)
			default:
				panic("amd64.backend.GenerateInstruction(): ir.IExt - Invalid size")
			}
		}

		// Unsigned is intentionally a nop, b.Load() guarantees the register
		// is already zero-extended via movzx, mov32, or masked constants

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.FExt:
		b.LoadOperand(inst.Value, 0, asm.XMM0)
		b.asm.Cvtss2sd(asm.XMM0, asm.XMM0)
		b.StoreResult(inst, 0, asm.XMM0)

	case *ir.FpToInt:
		mode := b.LoadOperand(inst.Value, 0, asm.XMM0)
		size := inst.Type().Info().Size

		if size != 1 && size != 2 && size != 4 && size != 8 {
			panic("amd64.backend.GenerateInstruction(): ir.FpToInt - Invalid integer size")
		}

		if !inst.Signed && size == 8 {
			largeLabel := b.asm.Label(nil)
			doneLabel := b.asm.Label(nil)

			if mode == f32 {
				b.asm.Mov(asm.RAX, int64(0x5f000000))
				b.asm.MovdToXmm(asm.XMM1, asm.RAX)
				b.asm.Ucomiss(asm.XMM0, asm.XMM1)
			} else {
				b.asm.Mov(asm.RAX, int64(0x43e0000000000000))
				b.asm.MovqToXmm(asm.XMM1, asm.RAX)
				b.asm.Ucomisd(asm.XMM0, asm.XMM1)
			}

			b.asm.Jae(largeLabel)

			// Fast path: 0 <= XMM0 < 2^63
			if mode == f32 {
				b.asm.Cvttss2si(true, asm.RAX, asm.XMM0)
			} else {
				b.asm.Cvttsd2si(true, asm.RAX, asm.XMM0)
			}

			b.asm.Jmp(doneLabel)

			// Slow path: XMM0 >= 2^63
			b.asm.Bind(largeLabel)

			if mode == f32 {
				b.asm.Subss(asm.XMM0, asm.XMM1)
				b.asm.Cvttss2si(true, asm.RAX, asm.XMM0)
			} else {
				b.asm.Subsd(asm.XMM0, asm.XMM1)
				b.asm.Cvttsd2si(true, asm.RAX, asm.XMM0)
			}

			b.asm.Mov(asm.RCX, int64(math.MinInt64))
			b.asm.Xor(asm.RAX, asm.RCX)

			b.asm.Bind(doneLabel)
		} else {
			is64 := size == 8 || (!inst.Signed && size == 4)

			if mode == f32 {
				b.asm.Cvttss2si(is64, asm.RAX, asm.XMM0)
			} else {
				b.asm.Cvttsd2si(is64, asm.RAX, asm.XMM0)
			}
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.IntToFp:
		b.LoadOperand(inst.Value, asm.RAX, 0)
		size := inst.Value.Type().Info().Size

		if size != 1 && size != 2 && size != 4 && size != 8 {
			panic("amd64.backend.GenerateInstruction(): ir.IntToFp - Invalid size")
		}

		if inst.Signed {
			switch size {
			case 1:
				b.asm.Movsx8(asm.RAX, asm.RAX)
			case 2:
				b.asm.Movsx16(asm.RAX, asm.RAX)
			}
		}

		isF32 := getFloatMode(inst.Type()) == f32

		// Special handling for unsigned 64-bit integers
		if !inst.Signed && size == 8 {
			largeLabel := b.asm.Label(nil)
			doneLabel := b.asm.Label(nil)

			// Check if MSB (bit 63) is set:
			b.asm.Test(asm.RAX, asm.RAX)
			b.asm.Js(largeLabel)

			// --- Fast Path: 0 <= RAX < 2^63 ---
			if isF32 {
				b.asm.Cvtsi2ss(true, asm.XMM0, asm.RAX)
			} else {
				b.asm.Cvtsi2sd(true, asm.XMM0, asm.RAX)
			}

			b.asm.Jmp(doneLabel)

			// --- Slow Path: RAX >= 2^63 ---
			b.asm.Bind(largeLabel)

			// RCX = RAX >> 1
			b.asm.Mov(asm.RCX, asm.RAX)
			b.asm.ShrImm(asm.RCX, 1)

			// RAX = RAX & 1 (sticky bit)
			b.asm.And32(asm.RAX, int32(1))

			// RCX = (RAX >> 1) | (RAX & 1)
			b.asm.Or(asm.RCX, asm.RAX)

			// Convert the halved value and multiply by 2 (add to itself)
			if isF32 {
				b.asm.Cvtsi2ss(true, asm.XMM0, asm.RCX)
				b.asm.Addss(asm.XMM0, asm.XMM0)
			} else {
				b.asm.Cvtsi2sd(true, asm.XMM0, asm.RCX)
				b.asm.Addsd(asm.XMM0, asm.XMM0)
			}

			b.asm.Bind(doneLabel)
		} else {
			// Standard path for signed integers and unsigned <= 32-bit
			is64 := size == 8 || (!inst.Signed && size == 4)

			if isF32 {
				b.asm.Cvtsi2ss(is64, asm.XMM0, asm.RAX)
			} else {
				b.asm.Cvtsi2sd(is64, asm.XMM0, asm.RAX)
			}
		}

		b.StoreResult(inst, 0, asm.XMM0)

	case *ir.PtrToInt:
		b.LoadOperand(inst.Value, asm.RAX, 0)
		b.StoreResult(inst, asm.RAX, 0)

	case *ir.IntToPtr:
		b.LoadOperand(inst.Value, asm.RAX, 0)
		b.StoreResult(inst, asm.RAX, 0)

	case *ir.BitCast:
		srcMode := getFloatMode(inst.Value.Type())
		dstMode := getFloatMode(inst.Type())

		if srcMode == none && dstMode == none {
			// Int/Ptr <-> Int/Ptr
			b.LoadOperand(inst.Value, asm.RAX, 0)
			b.StoreResult(inst, asm.RAX, 0)
		} else if srcMode != none && dstMode != none {
			// Float <-> Float
			b.LoadOperand(inst.Value, 0, asm.XMM0)
			b.StoreResult(inst, 0, asm.XMM0)
		} else if srcMode == none && dstMode != none {
			// Int -> Float
			b.LoadOperand(inst.Value, asm.RAX, 0)

			if dstMode == f32 {
				b.asm.MovdToXmm(asm.XMM0, asm.RAX)
			} else {
				b.asm.MovqToXmm(asm.XMM0, asm.RAX)
			}

			b.StoreResult(inst, 0, asm.XMM0)
		} else {
			// Float -> Int
			b.LoadOperand(inst.Value, 0, asm.XMM0)

			if srcMode == f32 {
				b.asm.MovdToReg(asm.RAX, asm.XMM0)
			} else {
				b.asm.MovqToReg(asm.RAX, asm.XMM0)
			}

			b.StoreResult(inst, asm.RAX, 0)
		}

	// Other instructions

	case *ir.ICmp:
		b.LoadOperand(inst.Left, asm.RAX, 0)
		b.LoadOperand(inst.Right, asm.RCX, 0)

		size := inst.Left.Type().Info().Size

		if size == 8 {
			b.asm.Cmp(asm.RAX, asm.RCX)
		} else {
			if inst.Signed {
				switch size {
				case 1:
					b.asm.Movsx8(asm.RAX, asm.RAX)
					b.asm.Movsx8(asm.RCX, asm.RCX)
				case 2:
					b.asm.Movsx16(asm.RAX, asm.RAX)
					b.asm.Movsx16(asm.RCX, asm.RCX)
				}
			}

			b.asm.Cmp32(asm.RAX, asm.RCX)
		}

		switch inst.Op {
		case ir.Eq:
			b.asm.Sete(asm.RAX)
		case ir.Ne:
			b.asm.Setne(asm.RAX)
		case ir.Lt:
			if inst.Signed {
				b.asm.Setl(asm.RAX)
			} else {
				b.asm.Setb(asm.RAX)
			}
		case ir.Le:
			if inst.Signed {
				b.asm.Setle(asm.RAX)
			} else {
				b.asm.Setbe(asm.RAX)
			}
		case ir.Gt:
			if inst.Signed {
				b.asm.Setg(asm.RAX)
			} else {
				b.asm.Seta(asm.RAX)
			}
		case ir.Ge:
			if inst.Signed {
				b.asm.Setge(asm.RAX)
			} else {
				b.asm.Setae(asm.RAX)
			}
		}

		b.asm.Movzx8(asm.RAX, asm.RAX)
		b.StoreResult(inst, asm.RAX, 0)

	case *ir.FCmp:
		mode := b.LoadOperand(inst.Left, 0, asm.XMM0)
		b.LoadOperand(inst.Right, 0, asm.XMM1)

		b.asm.Xor(asm.RAX, asm.RAX)
		b.asm.Xor(asm.RCX, asm.RCX)

		if mode == f32 {
			b.asm.Ucomiss(asm.XMM0, asm.XMM1)
		} else {
			b.asm.Ucomisd(asm.XMM0, asm.XMM1)
		}

		switch inst.Op {
		case ir.Eq:
			b.asm.Sete(asm.RAX)
			if inst.Ordered {
				b.asm.Setnp(asm.RCX)
				b.asm.And32(asm.RAX, asm.RCX)
			}

		case ir.Ne:
			b.asm.Setne(asm.RAX)
			if !inst.Ordered {
				b.asm.Setp(asm.RCX)
				b.asm.Or32(asm.RAX, asm.RCX)
			}

		case ir.Lt:
			b.asm.Setb(asm.RAX)
			if inst.Ordered {
				b.asm.Setnp(asm.RCX)
				b.asm.And32(asm.RAX, asm.RCX)
			}

		case ir.Le:
			b.asm.Setbe(asm.RAX)
			if inst.Ordered {
				b.asm.Setnp(asm.RCX)
				b.asm.And32(asm.RAX, asm.RCX)
			}

		case ir.Gt:
			b.asm.Seta(asm.RAX)
			if !inst.Ordered {
				b.asm.Setp(asm.RCX)
				b.asm.Or32(asm.RAX, asm.RCX)
			}

		case ir.Ge:
			b.asm.Setae(asm.RAX)
			if !inst.Ordered {
				b.asm.Setp(asm.RCX)
				b.asm.Or32(asm.RAX, asm.RCX)
			}
		}

		b.StoreResult(inst, asm.RAX, 0)

	case *ir.Phi:
		// nop, values are assigned when doing a jump

	case *ir.Select:
		size := inst.Type().Info().Size

		// Evaluate condition
		b.LoadOperand(inst.Condition, asm.RAX, 0)
		b.asm.Test32(asm.RAX, asm.RAX)

		trueLabel := b.asm.Label(nil)
		endLabel := b.asm.Label(nil)

		b.asm.Jne(trueLabel)

		// False branch
		if size <= 8 {
			b.LoadOperand(inst.IfFalse, asm.RAX, asm.XMM0)
		} else {
			dstOff := b.layout.offsets[inst]
			b.asm.Lea(asm.RAX, asm.RegDisp(asm.RBP, dstOff))
			b.LoadAddress(inst.IfFalse, asm.RCX)
			b.CopyMemory(asm.RAX, asm.RCX, size)
		}

		b.asm.Jmp(endLabel)

		// True branch
		b.asm.Bind(trueLabel)

		if size <= 8 {
			b.LoadOperand(inst.IfTrue, asm.RAX, asm.XMM0)
		} else {
			dstOff := b.layout.offsets[inst]
			b.asm.Lea(asm.RAX, asm.RegDisp(asm.RBP, dstOff))
			b.LoadAddress(inst.IfTrue, asm.RCX)
			b.CopyMemory(asm.RAX, asm.RCX, size)
		}

		// End
		b.asm.Bind(endLabel)

		if size <= 8 {
			b.StoreResult(inst, asm.RAX, asm.XMM0)
		}

	case *ir.Call:
		if as, ok := inst.Callee.(*ir.Assembly); ok {
			b.GenerateInlineAssembly(inst, as)
			break
		}

		intI := 0
		fpI := 0
		mergedIndices := strings.Contains(b.module.Triple, "windows")

		type stackArg struct {
			val    ir.Value
			offset int32
			size   uint32
		}

		var stackArgs []stackArg
		var stackBytes int32 = 0

		if mergedIndices {
			stackBytes = 32 // Offset past the shadow space
		}

		type regArg struct {
			val      ir.Value
			isFP     bool
			isTwoReg bool
			isPureFP bool
			isTwoFP  bool
			size     uint32
			intR1    asm.Reg
			intR2    asm.Reg
			fpR      asm.XmmReg
			fpR2     asm.XmmReg
		}

		var regArgs []regArg

		// Classify arguments
		for i, arg := range inst.Args {
			if fun, ok := inst.Callee.(*ir.Function); ok && strings.HasPrefix(fun.Name, "llvm.") && fun.Params[i].Name == "volatile" {
				break
			}

			info := arg.Type().Info()
			size := info.Size
			isFP := false

			if s, ok := arg.Type().(*ir.SimpleType); ok && (s.Kind == ir.FloatKind || s.Kind == ir.DoubleKind) {
				isFP = true
			}

			if isFP && fpI < len(b.fpParamRegs) {
				regArgs = append(regArgs, regArg{val: arg, isFP: true, fpR: b.fpParamRegs[fpI]})
				fpI++

				if mergedIndices {
					intI++
				}
			} else if !mergedIndices && ir.IsAggregate(arg.Type()) && isPureFloat(arg.Type()) && size <= 16 {
				if size <= 8 && fpI < len(b.fpParamRegs) {
					regArgs = append(regArgs, regArg{
						val:      arg,
						isPureFP: true,
						size:     size,
						fpR:      b.fpParamRegs[fpI],
					})

					fpI++
				} else if size > 8 && fpI+1 < len(b.fpParamRegs) {
					regArgs = append(regArgs, regArg{
						val:      arg,
						isPureFP: true,
						isTwoFP:  true,
						size:     size,
						fpR:      b.fpParamRegs[fpI],
						fpR2:     b.fpParamRegs[fpI+1],
					})

					fpI += 2
				} else {
					stackArgs = append(stackArgs, stackArg{val: arg, offset: stackBytes, size: size})
					stackBytes += int32(obj.AlignUp(uint64(size), 8))
				}
			} else if !isFP && size > 8 && size <= 16 && !mergedIndices && intI+1 < len(b.intParamRegs) {
				regArgs = append(regArgs, regArg{
					val:      arg,
					isTwoReg: true,
					size:     size,
					intR1:    b.intParamRegs[intI],
					intR2:    b.intParamRegs[intI+1],
				})

				intI += 2
			} else if !isFP && size <= 8 && intI < len(b.intParamRegs) {
				regArgs = append(regArgs, regArg{val: arg, size: size, intR1: b.intParamRegs[intI]})
				intI++

				if mergedIndices {
					fpI++
				}
			} else {
				stackArgs = append(stackArgs, stackArg{val: arg, offset: stackBytes, size: size})
				stackBytes += int32(obj.AlignUp(uint64(size), 8))
			}
		}

		// Copy stack arguments directly to [RSP + offset] (space pre-allocated in prologue)
		for _, sa := range stackArgs {
			if sa.size <= 8 {
				mode := b.LoadOperand(sa.val, asm.RAX, asm.XMM0)

				if mode == f32 {
					b.asm.Movss(asm.RegDisp(asm.RSP, sa.offset), asm.XMM0)
				} else if mode == f64 {
					b.asm.Movsd(asm.RegDisp(asm.RSP, sa.offset), asm.XMM0)
				} else {
					b.asm.Mov(asm.RegDisp(asm.RSP, sa.offset), asm.RAX)
				}
			} else {
				b.LoadAddress(sa.val, asm.R10)
				b.asm.Lea(asm.RAX, asm.RegDisp(asm.RSP, sa.offset))
				b.CopyMemory(asm.RAX, asm.R10, sa.size)
			}
		}

		// Load register arguments
		for _, ra := range regArgs {
			if ra.isPureFP {
				b.LoadAddress(ra.val, asm.R10)

				if ra.size <= 4 {
					b.asm.Movss(ra.fpR, asm.RegDisp(asm.R10, 0))
				} else {
					b.asm.Movsd(ra.fpR, asm.RegDisp(asm.R10, 0))
				}

				if ra.isTwoFP {
					if ra.size <= 12 {
						b.asm.Movss(ra.fpR2, asm.RegDisp(asm.R10, 8))
					} else {
						b.asm.Movsd(ra.fpR2, asm.RegDisp(asm.R10, 8))
					}
				}
			} else if ra.isFP {
				b.LoadOperand(ra.val, 0, ra.fpR)
			} else if ra.isTwoReg {
				b.LoadAddress(ra.val, asm.R10)
				b.asm.Mov(ra.intR1, asm.RegDisp(asm.R10, 0))
				b.LoadUpTo8(ra.intR2, asm.R10, 8, ra.size-8)
			} else {
				b.LoadOperand(ra.val, ra.intR1, 0)
			}
		}

		// Perform call
		if !strings.Contains(b.module.Triple, "windows") {
			if fpI > 0 {
				b.asm.Mov(asm.RAX, int64(fpI))
			} else {
				b.asm.Xor(asm.RAX, asm.RAX)
			}
		}

		switch callee := inst.Callee.(type) {
		case *ir.Function:
			b.asm.Call(b.funSymbols[callee])

		default:
			off, ok := b.layout.offsets[callee]
			if !ok {
				panic("amd64.backend.GenerateInstruction(): ir.Call - Value has no stack offset")
			}

			b.asm.Call(asm.RegDisp(asm.RBP, off))
		}

		// Store return result
		retSize := inst.Type().Info().Size

		if retSize == 0 {
			// Zero-sized return: no-op
		} else if !mergedIndices && ir.IsAggregate(inst.Type()) && isPureFloat(inst.Type()) && retSize <= 16 {
			dstOff := b.layout.offsets[inst]

			if retSize <= 4 {
				b.asm.Movss(asm.RegDisp(asm.RBP, dstOff), asm.XMM0)
			} else {
				b.asm.Movsd(asm.RegDisp(asm.RBP, dstOff), asm.XMM0)
			}

			if retSize > 8 {
				if retSize <= 12 {
					b.asm.Movss(asm.RegDisp(asm.RBP, dstOff+8), asm.XMM1)
				} else {
					b.asm.Movsd(asm.RegDisp(asm.RBP, dstOff+8), asm.XMM1)
				}
			}
		} else if retSize > 8 && retSize <= 16 {
			dstOff := b.layout.offsets[inst]
			b.asm.Mov(asm.RegDisp(asm.RBP, dstOff), asm.RAX)
			b.StoreUpTo8(asm.RBP, dstOff+8, asm.RDX, retSize-8)
		} else {
			b.StoreResult(inst, asm.RAX, asm.XMM0)
		}

	// Debug instructions

	case *ir.DbgDeclare:
		// nop

	default:
		panic("amd64.backend.GenerateInstruction() - Invalid instruction")
	}
}

// Utils

var asmRegNames = map[string]asm.Reg{
	"rax": asm.RAX, "rcx": asm.RCX, "rdx": asm.RDX, "rbx": asm.RBX,
	"rsi": asm.RSI, "rdi": asm.RDI, "rbp": asm.RBP, "rsp": asm.RSP,
	"r8": asm.R8, "r9": asm.R9, "r10": asm.R10, "r11": asm.R11,
	"r12": asm.R12, "r13": asm.R13, "r14": asm.R14, "r15": asm.R15,
}

func parseRegConstraint(c string) (asm.Reg, bool) {
	c = strings.TrimPrefix(c, "=")
	c = strings.TrimPrefix(c, "+") // in-out operand
	c = strings.TrimPrefix(c, "&") // early-clobber

	// Handles "{rax}", "{r10}", etc.
	if strings.HasPrefix(c, "{") && strings.HasSuffix(c, "}") {
		reg, ok := asmRegNames[strings.ToLower(c[1:len(c)-1])]
		return reg, ok
	}

	// Standard GCC/LLVM single-letter constraints fallback
	switch c {
	case "a", "r": // General register or accumulator
		return asm.RAX, true
	case "b":
		return asm.RBX, true
	case "c":
		return asm.RCX, true
	case "d":
		return asm.RDX, true
	case "S":
		return asm.RSI, true
	case "D":
		return asm.RDI, true
	}

	return 0, false
}

func (b *backend) GenerateInlineAssembly(inst *ir.Call, as *ir.Assembly) {
	// Verify single-word template (no operands)
	template := strings.TrimSpace(as.Template)

	if strings.ContainsAny(template, " \t\n") {
		panic("amd64.backend.GenerateInlineAssembly() - Inline asm with operands is not supported: " + template)
	}

	// Parse constraints
	outReg := asm.RAX
	hasOutput := false
	argIdx := 0

	type inputBinding struct {
		reg asm.Reg
		val ir.Value
	}

	var inputs []inputBinding

	for _, c := range as.Constraints {
		c = strings.TrimSpace(c)
		if c == "" || strings.HasPrefix(c, "~") {
			// Clobbers (e.g. ~{rcx}, ~{r11}, ~{memory}) are no-ops because
			// all variables live on the stack
			continue
		}

		if strings.HasPrefix(c, "=") || strings.HasPrefix(c, "+") {
			// Output constraint
			if reg, ok := parseRegConstraint(c); ok {
				outReg = reg
				hasOutput = true
			} else {
				panic("amd64.backend.GenerateInlineAssembly() - Unsupported output constraint: " + c)
			}
		} else {
			// Input constraint (maps to inst.Args in order)
			if argIdx >= len(inst.Args) {
				panic("amd64.backend.GenerateInlineAssembly() - More input constraints than arguments in inline asm")
			}

			if reg, ok := parseRegConstraint(c); ok {
				inputs = append(inputs, inputBinding{reg: reg, val: inst.Args[argIdx]})
			} else {
				panic("amd64.backend.GenerateInlineAssembly() - Unsupported input constraint: " + c)
			}

			argIdx++
		}
	}

	// Load all inputs into their specified registers
	for _, in := range inputs {
		b.LoadOperand(in.val, in.reg, 0)
	}

	// Emit the assembly instruction
	switch strings.ToLower(template) {
	case "syscall":
		b.asm.Syscall()
	case "nop":
		b.asm.Nop()

	default:
		panic("amd64.backend.GenerateInlineAssembly() - Unsupported inline asm instruction: " + template)
	}

	// 5. Save the output register if non-void
	if hasOutput {
		if s, ok := inst.Type().(*ir.SimpleType); !ok || s.Kind != ir.VoidKind {
			b.StoreResult(inst, outReg, 0)
		}
	}
}

func (b *backend) CalculateAggregateOffset(baseTyp ir.Type, indices []uint32) (uint32, ir.Type) {
	var totalOffset uint32 = 0
	current := baseTyp

	for _, idx := range indices {
		if idx == math.MaxUint32 {
			break
		}

		switch t := current.(type) {
		case *ir.ArrayType:
			totalOffset += idx * t.Element.Info().Size
			current = t.Element

		case *ir.VectorType:
			totalOffset += idx * t.Element.Info().Size
			current = t.Element

		case ir.StructLikeType:
			for fieldIdx, range_ := range ir.GetStructFieldRanges(t) {
				if uint32(fieldIdx) == idx {
					totalOffset += uint32(range_.Offset)
					break
				}
			}

			switch t := t.(type) {
			case *ir.StructType:
				current = t.Fields[idx].Type
			case *ir.RefStructType:
				current = t.Struct.Fields[idx].Type
			}

		default:
			panic("amd64.backend.CalculateAggregateOffset() - Unsupported type for indexing")
		}
	}

	return totalOffset, current
}

func (b *backend) CalculateGepConstOffset(baseTyp ir.Type, indices [4]uint32) uint32 {
	var totalOffset uint32 = 0
	current := baseTyp

	for i, idx := range indices {
		if idx == math.MaxUint32 {
			break
		}

		if i == 0 {
			totalOffset += idx * baseTyp.Info().Size
			continue
		}

		switch t := current.(type) {
		case *ir.ArrayType:
			totalOffset += idx * t.Element.Info().Size
			current = t.Element

		case *ir.VectorType:
			totalOffset += idx * t.Element.Info().Size
			current = t.Element

		case ir.StructLikeType:
			for fieldIdx, range_ := range ir.GetStructFieldRanges(t) {
				if uint32(fieldIdx) == idx {
					totalOffset += uint32(range_.Offset)
					break
				}
			}

			switch t := t.(type) {
			case *ir.StructType:
				current = t.Fields[idx].Type
			case *ir.RefStructType:
				current = t.Struct.Fields[idx].Type
			}

		default:
			panic("amd64.backend.CalculateGepConstOffset() - Unsupported type for indexing")
		}
	}

	return totalOffset
}

func (b *backend) HasPhis(block *ir.Block) bool {
	for inst := range block.Instructions() {
		if _, ok := inst.(*ir.Phi); ok {
			return true
		}
	}

	return false
}

type phiResolution struct {
	phi *ir.Phi
	val ir.Value
}

func (b *backend) ResolvePhis(from *ir.Block, to *ir.Block) {
	var items []phiResolution

	// 1. Collect all Phi assignments on this edge
	for inst := range to.Instructions() {
		phi, ok := inst.(*ir.Phi)
		if !ok {
			continue
		}

		for _, pair := range phi.Pairs {
			if pair.Block == from {
				if pair.Value != phi { // Skip self-assignment (no-op)
					items = append(items, phiResolution{phi: phi, val: pair.Value})
				}

				break
			}
		}
	}

	if len(items) == 0 {
		return
	}

	// 2. Identify Phis whose current values are clobbered by this transition.
	// A Phi must be saved if it is modified and another Phi reads its incoming value.
	tempOffsets := make(map[*ir.Phi]int32)
	var tempSize uint32 = 0

	for _, item := range items {
		needed := false

		for _, other := range items {
			if other.phi != item.phi && other.val == item.phi {
				needed = true
				break
			}
		}

		if needed {
			info := item.phi.Type().Info()
			tempSize = uint32(obj.AlignUp(uint64(tempSize), uint64(info.Align)))
			tempOffsets[item.phi] = int32(tempSize)
			tempSize += info.Size
		}
	}

	// 3. Allocate 16-byte aligned temporary stack space if backups are needed
	totalAlloc := uint32(obj.AlignUp(uint64(tempSize), 16))

	if totalAlloc > 0 {
		b.asm.Sub(asm.RSP, int32(totalAlloc))

		// Snapshot the current values of conflicting Phis to [RSP + tempOff]
		for phi, tempOff := range tempOffsets {
			srcOff := b.layout.offsets[phi]
			size := phi.Type().Info().Size

			if mode := getFloatMode(phi.Type()); mode != none {
				if mode == f32 {
					b.asm.Movss(asm.XMM0, asm.RegDisp(asm.RBP, srcOff))
					b.asm.Movss(asm.RegDisp(asm.RSP, tempOff), asm.XMM0)
				} else {
					b.asm.Movsd(asm.XMM0, asm.RegDisp(asm.RBP, srcOff))
					b.asm.Movsd(asm.RegDisp(asm.RSP, tempOff), asm.XMM0)
				}
			} else if size <= 8 {
				if size > 0 {
					b.LoadUpTo8(asm.RAX, asm.RBP, srcOff, size)
					b.StoreUpTo8(asm.RSP, tempOff, asm.RAX, size)
				}
			} else {
				b.asm.Lea(asm.RAX, asm.RegDisp(asm.RSP, tempOff))
				b.asm.Lea(asm.RCX, asm.RegDisp(asm.RBP, srcOff))
				b.CopyMemory(asm.RAX, asm.RCX, size)
			}
		}
	}

	// 4. Perform the Phi assignments
	for _, item := range items {
		phiVal, isPhi := item.val.(*ir.Phi)
		savedTempOff, hasSaved := tempOffsets[phiVal]
		size := item.phi.Type().Info().Size

		if mode := getFloatMode(item.phi.Type()); mode != none {
			if isPhi && hasSaved {
				if mode == f32 {
					b.asm.Movss(asm.XMM0, asm.RegDisp(asm.RSP, savedTempOff))
				} else {
					b.asm.Movsd(asm.XMM0, asm.RegDisp(asm.RSP, savedTempOff))
				}
			} else {
				b.LoadOperand(item.val, 0, asm.XMM0)
			}

			b.StoreResult(item.phi, 0, asm.XMM0)
		} else if size <= 8 {
			if isPhi && hasSaved {
				if size > 0 {
					b.LoadUpTo8(asm.RAX, asm.RSP, savedTempOff, size)
				}
			} else {
				b.LoadOperand(item.val, asm.RAX, 0)
			}

			b.StoreResult(item.phi, asm.RAX, 0)
		} else {
			dstOff := b.layout.offsets[item.phi]
			b.asm.Lea(asm.RAX, asm.RegDisp(asm.RBP, dstOff))

			if isPhi && hasSaved {
				b.asm.Lea(asm.RCX, asm.RegDisp(asm.RSP, savedTempOff))
			} else {
				b.LoadAddress(item.val, asm.RCX)
			}

			b.CopyMemory(asm.RAX, asm.RCX, size)
		}
	}

	// 5. Restore stack pointer before jumping
	if totalAlloc > 0 {
		b.asm.Add(asm.RSP, int32(totalAlloc))
	}
}

func (b *backend) PrepareIDiv(left, right ir.Value, typ ir.Type, kind ir.DivKind) {
	b.LoadOperand(left, asm.RAX, 0)
	b.LoadOperand(right, asm.RCX, 0)

	size := typ.Info().Size

	if kind == ir.Signed {
		if size == 8 {
			b.asm.Cqo()
			b.asm.Idiv(asm.RCX)
		} else {
			switch size {
			case 1:
				b.asm.Movsx8(asm.RAX, asm.RAX)
				b.asm.Movsx8(asm.RCX, asm.RCX)
			case 2:
				b.asm.Movsx16(asm.RAX, asm.RAX)
				b.asm.Movsx16(asm.RCX, asm.RCX)
			case 4:
				// nop
			default:
				panic("amd64.backend.PrepareIDiv() - Invalid size (signed)")
			}

			b.asm.Cdq()
			b.asm.Idiv32(asm.RCX)
		}
	} else {
		b.asm.Xor(asm.RDX, asm.RDX)

		switch size {
		case 1, 2, 4:
			b.asm.Div32(asm.RCX)
		case 8:
			b.asm.Div(asm.RCX)

		default:
			panic("amd64.backend.PrepareIDiv() - Invalid size (unsigned)")
		}
	}
}

func (b *backend) LoadOperand(val ir.Value, intReg asm.Reg, fpReg asm.XmmReg) floatMode {
	switch val := val.(type) {
	case *ir.ZeroInitializer:
		// Floating point zero
		if mode := getFloatMode(val.Typ); mode != none {
			if mode == f32 {
				b.asm.Xorps(fpReg, fpReg)
				return f32
			}

			b.asm.Xorpd(fpReg, fpReg)
			return f64
		}

		// Any scalar, pointer, or small aggregate fitting in a 64-bit register
		if val.Type().Info().Size <= 8 {
			b.asm.Xor(intReg, intReg)
			return none
		}

		panic("amd64.backend.LoadOperand(): ir.ZeroInitializer - Cannot load aggregate > 8 bytes into a register")

	case *ir.Null:
		b.asm.Xor(intReg, intReg)
		return none

	case *ir.Integer:
		v := val.Value.TwosComplement()

		switch val.Type().Info().Size {
		case 1:
			v &= 0xFF
		case 2:
			v &= 0xFFFF
		case 4:
			v &= 0xFFFFFFFF
		}

		if v == 0 {
			b.asm.Xor(intReg, intReg)
		} else {
			b.asm.Mov(intReg, int64(v))
		}

		return none

	case *ir.FloatV:
		bits := math.Float32bits(val.Value)

		if bits == 0 {
			b.asm.Xorps(fpReg, fpReg)
			return f32
		}

		start := b.PrepareSectionData(b.rodata, val.Type().Info())
		binary.LittleEndian.PutUint32(b.rodata.Data[start:start+4], bits)

		symbol := b.file.AddSymbol(&obj.Symbol{
			Name:    "f32-" + strconv.FormatUint(start, 16),
			Kind:    obj.SymData,
			Scope:   obj.ScopeLocal,
			Section: b.rodata,
			Value:   start,
			Size:    4,
		})

		b.asm.Movss(fpReg, asm.Rip(symbol))
		return f32

	case *ir.DoubleV:
		bits := math.Float64bits(val.Value)

		if bits == 0 {
			b.asm.Xorpd(fpReg, fpReg)
			return f64
		}

		start := b.PrepareSectionData(b.rodata, val.Type().Info())
		binary.LittleEndian.PutUint64(b.rodata.Data[start:start+8], bits)

		symbol := b.file.AddSymbol(&obj.Symbol{
			Name:    "f64-" + strconv.FormatUint(start, 16),
			Kind:    obj.SymData,
			Scope:   obj.ScopeLocal,
			Section: b.rodata,
			Value:   start,
			Size:    8,
		})

		b.asm.Movsd(fpReg, asm.Rip(symbol))
		return f64

	case *ir.Struct, *ir.Array, *ir.Vector:
		size := val.Type().Info().Size

		if size == 0 {
			b.asm.Xor(intReg, intReg)
			return none
		}

		if size <= 8 {
			b.LoadAddress(val, intReg)
			b.LoadUpTo8(intReg, intReg, 0, size)
			return none
		}

		panic("amd64.backend.LoadOperand(): Aggregate > 8 bytes cannot be loaded into a register")

	case *ir.GlobalVar:
		b.asm.Lea(intReg, asm.Rip(b.gVarSymbols[val]))
		return none

	case *ir.Function:
		b.asm.Lea(intReg, asm.Rip(b.funSymbols[val]))
		return none

	case *ir.Alloca:
		off := b.layout.offsets[val]
		b.asm.Lea(intReg, asm.RegDisp(asm.RBP, off))
		return none

	default:
		// Instruction result or spilled parameter on the stack
		off, ok := b.layout.offsets[val]
		if !ok {
			panic("amd64.backend.Load() - Value has no stack offset")
		}

		if mode := getFloatMode(val.Type()); mode != none {
			if mode == f32 {
				b.asm.Movss(fpReg, asm.RegDisp(asm.RBP, off))
			} else {
				b.asm.Movsd(fpReg, asm.RegDisp(asm.RBP, off))
			}

			return mode
		}

		size := val.Type().Info().Size
		if size == 0 {
			b.asm.Xor(intReg, intReg)
			return none
		}

		// FIX 3: Load exact bytes and zero-extend cleanly for all sizes <= 8
		if size <= 8 {
			b.LoadUpTo8(intReg, asm.RBP, off, size)
			return none
		}

		panic("amd64.backend.Load() - Invalid size: " + strconv.FormatUint(uint64(size), 10))
	}
}

func (b *backend) StoreResult(inst ir.Instruction, intReg asm.Reg, fpReg asm.XmmReg) {
	offset, ok := b.layout.offsets[inst]
	if !ok {
		return // void instruction
	}

	if mode := getFloatMode(inst.Type()); mode != none {
		if mode == f32 {
			b.asm.Movss(asm.RegDisp(asm.RBP, offset), fpReg)
		} else {
			b.asm.Movsd(asm.RegDisp(asm.RBP, offset), fpReg)
		}

		return
	}

	size := inst.Type().Info().Size
	if size == 0 {
		return
	}

	if size <= 8 {
		b.StoreUpTo8(asm.RBP, offset, intReg, size)
		return
	}

	panic("amd64.backend.Store() - Invalid size: " + strconv.FormatUint(uint64(size), 10))
}

func (b *backend) LoadAddress(val ir.Value, reg asm.Reg) {
	switch v := val.(type) {
	case *ir.GlobalVar:
		b.asm.Lea(reg, asm.Rip(b.gVarSymbols[v]))

	case *ir.Function:
		b.asm.Lea(reg, asm.Rip(b.funSymbols[v]))

	case *ir.ZeroInitializer:
		info := v.Type().Info()
		start := b.PrepareSectionData(b.rodata, info)

		symbol := b.file.AddSymbol(&obj.Symbol{
			Name:    "const-" + strconv.FormatUint(start, 16),
			Kind:    obj.SymData,
			Scope:   obj.ScopeLocal,
			Section: b.rodata,
			Value:   start,
			Size:    uint64(info.Size),
		})

		b.asm.Lea(reg, asm.Rip(symbol))

	case *ir.Null:
		start := b.PrepareSectionData(b.rodata, ir.TypeInfo{Size: 8, Align: 8})

		symbol := b.file.AddSymbol(&obj.Symbol{
			Name:    "null-" + strconv.FormatUint(start, 16),
			Kind:    obj.SymData,
			Scope:   obj.ScopeLocal,
			Section: b.rodata,
			Value:   start,
			Size:    8,
		})

		b.asm.Lea(reg, asm.Rip(symbol))

	case *ir.Struct, *ir.Vector, *ir.Array:
		info := v.Type().Info()
		start := b.PrepareSectionData(b.rodata, info)

		data := b.rodata.Data[start : start+uint64(info.Size)]
		b.WriteIrValue(v, b.rodata, start, data, 0)

		symbol := b.file.AddSymbol(&obj.Symbol{
			Name:    "const-" + strconv.FormatUint(start, 16),
			Kind:    obj.SymData,
			Scope:   obj.ScopeLocal,
			Section: b.rodata,
			Value:   start,
			Size:    uint64(info.Size),
		})

		b.asm.Lea(reg, asm.Rip(symbol))

	default:
		off, ok := b.layout.offsets[val]
		if !ok {
			panic("amd64.backend.LoadAddress() - Value has no stack offset")
		}

		b.asm.Lea(reg, asm.RegDisp(asm.RBP, off))
	}
}

// LoadUpTo8 loads 'size' bytes (1 <= size <= 8) from [base + off] into reg, zero-extending it.
func (b *backend) LoadUpTo8(reg asm.Reg, base asm.Reg, off int32, size uint32) {
	if reg == base && (size == 3 || size == 5 || size == 6 || size == 7) {
		b.asm.Mov(asm.R10, base)
		base = asm.R10
	}

	switch size {
	case 1:
		b.asm.Movzx8(reg, asm.RegDisp(base, off))
	case 2:
		b.asm.Movzx16(reg, asm.RegDisp(base, off))
	case 4:
		b.asm.Mov32(reg, asm.RegDisp(base, off))
	case 8:
		b.asm.Mov(reg, asm.RegDisp(base, off))

	case 3, 5, 6, 7:
		b.asm.Xor(reg, reg)

		var curOff int32 = 0
		var shift uint8 = 0

		if size >= 4 {
			b.asm.Mov32(reg, asm.RegDisp(base, off+curOff))
			curOff += 4
			size -= 4
			shift += 32
		}

		if size >= 2 {
			b.asm.Movzx16(asm.R11, asm.RegDisp(base, off+curOff))
			if shift > 0 {
				b.asm.ShlImm(asm.R11, shift)
				b.asm.Or(reg, asm.R11)
			} else {
				b.asm.Mov(reg, asm.R11)
			}

			curOff += 2
			size -= 2
			shift += 16
		}

		if size == 1 {
			b.asm.Movzx8(asm.R11, asm.RegDisp(base, off+curOff))
			if shift > 0 {
				b.asm.ShlImm(asm.R11, shift)
				b.asm.Or(reg, asm.R11)
			} else {
				b.asm.Mov(reg, asm.R11)
			}
		}

	default:
		panic("amd64.backend.LoadUpTo8() - Invalid size: " + strconv.FormatUint(uint64(size), 10))
	}
}

// StoreUpTo8 writes 'size' bytes (1 <= size <= 8) from reg into [base + off].
func (b *backend) StoreUpTo8(base asm.Reg, off int32, reg asm.Reg, size uint32) {
	switch size {
	case 1:
		b.asm.Mov8(asm.RegDisp(base, off), reg)
	case 2:
		b.asm.Mov16(asm.RegDisp(base, off), reg)
	case 4:
		b.asm.Mov32(asm.RegDisp(base, off), reg)
	case 8:
		b.asm.Mov(asm.RegDisp(base, off), reg)

	case 3, 5, 6, 7:
		var curOff int32 = 0

		if size >= 4 {
			b.asm.Mov32(asm.RegDisp(base, off+curOff), reg)
			curOff += 4
			size -= 4
			b.asm.Mov(asm.R11, reg)
			b.asm.ShrImm(asm.R11, 32)
			reg = asm.R11
		}

		if size >= 2 {
			b.asm.Mov16(asm.RegDisp(base, off+curOff), reg)
			curOff += 2
			size -= 2
			b.asm.Mov(asm.R11, reg)
			b.asm.ShrImm(asm.R11, 16)
			reg = asm.R11
		}

		if size == 1 {
			b.asm.Mov8(asm.RegDisp(base, off+curOff), reg)
		}

	default:
		panic("amd64.backend.StoreUpTo8() - Invalid size: " + strconv.FormatUint(uint64(size), 10))
	}
}

func (b *backend) CopyMemory(dstReg, srcReg asm.Reg, size uint32) {
	var off int32 = 0

	for size >= 8 {
		b.asm.Mov(asm.R11, asm.RegDisp(srcReg, off))
		b.asm.Mov(asm.RegDisp(dstReg, off), asm.R11)

		off += 8
		size -= 8
	}

	if size >= 4 {
		b.asm.Mov32(asm.R11, asm.RegDisp(srcReg, off))
		b.asm.Mov32(asm.RegDisp(dstReg, off), asm.R11)

		off += 4
		size -= 4
	}

	if size >= 2 {
		b.asm.Movzx16(asm.R11, asm.RegDisp(srcReg, off))
		b.asm.Mov16(asm.RegDisp(dstReg, off), asm.R11)

		off += 2
		size -= 2
	}

	if size == 1 {
		b.asm.Movzx8(asm.R11, asm.RegDisp(srcReg, off))
		b.asm.Mov8(asm.RegDisp(dstReg, off), asm.R11)
	}
}

func (b *backend) ZeroMemory(base asm.Reg, size uint32) {
	var off int32 = 0

	b.asm.Xor(asm.R11, asm.R11)

	for size >= 8 {
		b.asm.Mov(asm.RegDisp(base, off), asm.R11)

		off += 8
		size -= 8
	}

	if size >= 4 {
		b.asm.Mov32(asm.RegDisp(base, off), asm.R11)

		off += 4
		size -= 4
	}

	if size >= 2 {
		b.asm.Mov16(asm.RegDisp(base, off), asm.R11)

		off += 2
		size -= 2
	}

	if size == 1 {
		b.asm.Mov8(asm.RegDisp(base, off), asm.R11)
	}
}

func isPureFloat(typ ir.Type) bool {
	if mode := getFloatMode(typ); mode != none {
		return true
	}

	switch t := typ.(type) {
	case *ir.VectorType:
		return isPureFloat(t.Element)

	case *ir.ArrayType:
		return isPureFloat(t.Element)

	case ir.StructLikeType:
		for _, field := range t.AllFields() {
			if !isPureFloat(field.Type) {
				return false
			}
		}

		return true
	}

	return false
}

func getFloatMode(typ ir.Type) floatMode {
	if s, ok := typ.(*ir.SimpleType); ok {
		if s.Kind == ir.FloatKind {
			return f32
		}
		if s.Kind == ir.DoubleKind {
			return f64
		}
	}

	return none
}
