package amd64

import (
	"fireball/backend/amd64/asm"
	"fireball/backend/obj"
	"fireball/ir"
	"strings"
)

func (b *backend) CreateFunction(fun *ir.Function) {
	name := fun.Name

	switch name {
	case "llvm.memcpy.p0.p0.i64":
		name = "memcpy"
	case "llvm.memmove.p0.p0.i64":
		name = "memmove"
	case "llvm.memset.p0.i64":
		name = "memset"
	}

	// External symbol
	if fun.Flags&ir.Declare != 0 {
		b.funSymbols[fun] = b.file.AddSymbol(&obj.Symbol{
			Name:  name,
			Kind:  obj.SymFunction,
			Scope: obj.ScopeGlobal,
		})

		return
	}

	// Get target section
	section := b.text

	if fun.Flags&ir.LinkOnceODR != 0 {
		section = b.file.AddSection(&obj.Section{
			Name:        ".text",
			Kind:        obj.SecText,
			Align:       16,
			Deduplicate: true,
		})
	}

	// Create symbol
	b.funSymbols[fun] = b.file.AddSymbol(&obj.Symbol{
		Name:    name,
		Kind:    obj.SymFunction,
		Scope:   obj.ScopeGlobal,
		Section: section,
	})
}

func (b *backend) GenerateFunction(fun *ir.Function) {
	// Skip external
	if fun.Flags&ir.Declare != 0 {
		return
	}

	// Setup assembler
	symbol := b.funSymbols[fun]
	prevAsm := b.asm

	if symbol.Section.Deduplicate {
		b.asm = asm.Assembler{}
	} else {
		b.asm.Align(16)
	}

	// Start function
	b.asm.Bind(b.asm.Label(symbol))
	startOffset := b.asm.Len()

	b.layout = b.CalculateFrameLayout(fun)
	b.epilogue = b.asm.Label(nil)
	b.CreateBlockLabels(fun)

	frameSize := obj.AlignUp(uint64(b.layout.size), 16)

	// Prologue
	b.asm.Push(asm.RBP)
	b.asm.Mov(asm.RBP, asm.RSP)

	if frameSize > 0 {
		b.asm.Sub(asm.RSP, int32(frameSize))
	}

	// Instructions
	b.SpillParameters(fun)

	for _, block := range fun.Blocks {
		b.asm.Bind(b.blockLabels[block])

		for inst := range block.Instructions() {
			b.GenerateInstruction(inst)
		}
	}

	// Epilogue
	b.asm.Bind(b.epilogue)
	b.asm.Mov(asm.RSP, asm.RBP)
	b.asm.Pop(asm.RBP)
	b.asm.Ret()

	// Finish assembler and symbol
	if symbol.Section.Deduplicate {
		symbol.Section.Data, symbol.Section.Relocations = b.asm.Assemble()
		symbol.Size = uint64(len(symbol.Section.Data))

		b.asm = prevAsm
	} else {
		symbol.Size = b.asm.Len() - startOffset
	}
}

func (b *backend) CreateBlockLabels(fun *ir.Function) {
	b.blockLabels = make(map[*ir.Block]asm.Label)

	for _, block := range fun.Blocks {
		b.blockLabels[block] = b.asm.Label(nil)
	}
}

func (b *backend) SpillParameters(fun *ir.Function) {
	intI := 0
	fpI := 0

	mergedIndices := strings.Contains(b.module.Triple, "windows")

	for i, value := range fun.ParamValues {
		off := b.layout.offsets[value]
		if off > 0 {
			continue
		}

		mem := asm.RegDisp(asm.RBP, off)
		paramType := fun.Signature.Params[i]
		size := paramType.Info().Size

		if s, ok := paramType.(*ir.SimpleType); ok && (s.Kind == ir.FloatKind || s.Kind == ir.DoubleKind) {
			reg := b.fpParamRegs[fpI]
			fpI++

			if s.Kind == ir.FloatKind {
				b.asm.Movss(mem, reg)
			} else {
				b.asm.Movsd(mem, reg)
			}
		} else if !mergedIndices && ir.IsAggregate(paramType) && isPureFloat(paramType) && size <= 16 {
			regLo := b.fpParamRegs[fpI]
			fpI++

			if size <= 4 {
				b.asm.Movss(asm.RegDisp(asm.RBP, off), regLo)
			} else {
				b.asm.Movsd(asm.RegDisp(asm.RBP, off), regLo)
			}

			if size > 8 {
				regHi := b.fpParamRegs[fpI]
				fpI++

				if size <= 12 {
					b.asm.Movss(asm.RegDisp(asm.RBP, off+8), regHi)
				} else {
					b.asm.Movsd(asm.RegDisp(asm.RBP, off+8), regHi)
				}
			}
		} else if size > 8 && size <= 16 && !mergedIndices {
			regLo := b.intParamRegs[intI]
			intI++
			regHi := b.intParamRegs[intI]
			intI++

			// Store lower 8 bytes
			b.asm.Mov(asm.RegDisp(asm.RBP, off), regLo)
			// Store upper (size - 8) bytes (e.g. 4 bytes for { i64, i32 })
			b.StoreUpTo8(asm.RBP, off+8, regHi, size-8)
		} else if size <= 8 {
			reg := b.intParamRegs[intI]
			intI++

			b.asm.Mov(asm.RegDisp(asm.RBP, off), reg)
		} else {
			panic("amd64.backend.SpillParameters() - Invalid size")
		}

		if mergedIndices {
			intI = max(intI, fpI)
			fpI = max(intI, fpI)
		}
	}
}

func (b *backend) CalculateFrameLayout(fun *ir.Function) frameLayout {
	layout := frameLayout{
		offsets: make(map[ir.Value]int32),
	}

	var currentLocalOffset uint64 = 0
	var currentStackArgOffset int32 = 16

	if strings.Contains(b.module.Triple, "windows") {
		currentStackArgOffset = 48
	}

	intI := 0
	fpI := 0
	mergedIndices := strings.Contains(b.module.Triple, "windows")

	// Assign slots for parameters
	for i, param := range fun.ParamValues {
		paramType := fun.Signature.Params[i]
		info := paramType.Info()
		size := info.Size
		isFP := false

		if s, ok := paramType.(*ir.SimpleType); ok && (s.Kind == ir.FloatKind || s.Kind == ir.DoubleKind) {
			isFP = true
		}

		isPureFP := !mergedIndices && ir.IsAggregate(paramType) && isPureFloat(paramType) && size <= 16

		inRegs := false
		if isFP {
			if fpI < len(b.fpParamRegs) {
				inRegs = true
				fpI++
				if mergedIndices {
					intI++
				}
			}
		} else if isPureFP {
			if size <= 8 && fpI < len(b.fpParamRegs) {
				inRegs = true
				fpI++
			} else if size > 8 && fpI+1 < len(b.fpParamRegs) {
				inRegs = true
				fpI += 2
			}
		} else if size > 8 && size <= 16 && !mergedIndices {
			if intI+1 < len(b.intParamRegs) {
				inRegs = true
				intI += 2
			}
		} else if size <= 8 {
			if intI < len(b.intParamRegs) {
				inRegs = true
				intI++
				if mergedIndices {
					fpI++
				}
			}
		}

		if inRegs {
			slotSize := max(info.Size, 8)
			align := max(info.Align, 8)

			currentLocalOffset = obj.AlignUp(currentLocalOffset+uint64(slotSize), uint64(align))
			layout.offsets[param] = -int32(currentLocalOffset)
		} else {
			layout.offsets[param] = currentStackArgOffset
			argBytes := int32(obj.AlignUp(uint64(info.Size), 8))
			currentStackArgOffset += argBytes
		}
	}

	// Assign slots for instructions
	for _, block := range fun.Blocks {
		for inst := range block.Instructions() {
			if s, ok := inst.Type().(*ir.SimpleType); ok && s.Kind == ir.VoidKind {
				continue
			}

			var info ir.TypeInfo
			if alloca, ok := inst.(*ir.Alloca); ok {
				info = alloca.Typ.Info()
			} else {
				info = inst.Type().Info()
			}

			align := max(info.Align, 1)

			currentLocalOffset = obj.AlignUp(currentLocalOffset+uint64(info.Size), uint64(align))
			layout.offsets[inst] = -int32(currentLocalOffset)
		}
	}

	// Outgoing call arguments & Windows shadow space
	if b.ContainsCall(fun) {
		var maxCallStackBytes uint32 = 0
		if strings.Contains(b.module.Triple, "windows") {
			maxCallStackBytes = 32 // Windows minimum shadow space
		}

		for _, block := range fun.Blocks {
			for inst := range block.Instructions() {
				switch call := inst.(type) {
				case *ir.Call:
					if _, isAsm := call.Callee.(*ir.Assembly); isAsm {
						continue
					}
					bytes := b.CalculateCallStackBytes(call)
					if bytes > maxCallStackBytes {
						maxCallStackBytes = bytes
					}

				case *ir.FRem:
					if strings.Contains(b.module.Triple, "windows") && maxCallStackBytes < 32 {
						maxCallStackBytes = 32
					}
				}
			}
		}

		currentLocalOffset += uint64(maxCallStackBytes)
	}

	// Align total frame size to 16 bytes
	layout.size = uint32(obj.AlignUp(currentLocalOffset, 16))

	return layout
}

func (b *backend) CalculateCallStackBytes(call *ir.Call) uint32 {
	intI := 0
	fpI := 0
	mergedIndices := strings.Contains(b.module.Triple, "windows")

	var stackBytes uint32 = 0
	if mergedIndices {
		stackBytes = 32 // Windows 32-byte shadow space
	}

	for i, arg := range call.Args {
		if fun, ok := call.Callee.(*ir.Function); ok && strings.HasPrefix(fun.Name, "llvm.") && fun.Params[i].Name == "volatile" {
			break
		}

		info := arg.Type().Info()
		size := info.Size
		isFP := false

		if s, ok := arg.Type().(*ir.SimpleType); ok && (s.Kind == ir.FloatKind || s.Kind == ir.DoubleKind) {
			isFP = true
		}

		if isFP && fpI < len(b.fpParamRegs) {
			fpI++
			if mergedIndices {
				intI++
			}
		} else if !isFP && size > 8 && size <= 16 && !mergedIndices && intI+1 < len(b.intParamRegs) {
			intI += 2
		} else if !isFP && size <= 8 && intI < len(b.intParamRegs) {
			intI++
			if mergedIndices {
				fpI++
			}
		} else {
			stackBytes += uint32(obj.AlignUp(uint64(size), 8))
		}
	}

	return uint32(obj.AlignUp(uint64(stackBytes), 16))
}

func (b *backend) ContainsCall(fun *ir.Function) bool {
	for _, block := range fun.Blocks {
		for inst := range block.Instructions() {
			switch inst := inst.(type) {
			case *ir.Call:
				if _, ok := inst.Callee.(*ir.Assembly); !ok {
					return true
				}

			case *ir.FRem:
				return true
			}
		}
	}

	return false
}
