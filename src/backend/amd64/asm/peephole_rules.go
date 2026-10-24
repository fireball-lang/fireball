package asm

import "math/bits"

const minImm32 = -1 << 31
const maxImm32 = 1<<31 - 1

// 1. Immediate & memory folding

// mov reg, <imm>
// <op> dst, reg   -> <op> dst, <imm> (deletes mov if reg is dead)
func ruleFoldMovImmIntoOp(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMov32 {
		return false
	}
	if !inst.Args[0].IsReg() || !inst.Args[1].IsImm() {
		return false
	}

	reg := inst.Args[0].Reg
	val := inst.Args[1].Val

	nextI := nextActiveI(instructions, i, false)
	if nextI == -1 {
		return false
	}
	next := &instructions[nextI]

	if next.NumArgs < 2 || !next.Args[1].IsReg() || next.Args[1].Reg != reg {
		return false
	}

	dst := next.Args[0]
	if dst.IsReg() && dst.Reg == reg {
		return false
	}
	if dst.IsMem() && dst.SymbolID == 0 && (dst.Reg == reg || (dst.Scale != 0 && dst.Index == reg)) {
		return false
	}

	is64 := false
	is16 := false
	is8 := false

	switch next.Op {
	case OcAdd, OcSub, OcAnd, OcOr, OcXor, OcCmp:
		is64 = true
	case OcAdd32, OcSub32, OcAnd32, OcOr32, OcXor32, OcCmp32:
		is64 = false

	case OcTest:
		if !dst.IsReg() {
			return false
		}
		is64 = true
	case OcTest32:
		if !dst.IsReg() {
			return false
		}
		is64 = false

	case OcImul:
		if !dst.IsReg() {
			return false
		}
		is64 = true
	case OcImul32:
		if !dst.IsReg() {
			return false
		}
		is64 = false

	case OcMov:
		if dst.IsMem() && (val < minImm32 || val > maxImm32) {
			return false
		}
		is64 = true
	case OcMov32:
		is64 = false

	case OcMov16:
		// Must fit in signed int16 or unsigned uint16
		if val < -32768 || val > 65535 {
			return false
		}
		is16 = true

	case OcMov8:
		if val < -128 || val > 255 {
			return false
		}
		is8 = true

	default:
		return false
	}

	if is64 && next.Op != OcMov {
		if val < minImm32 || val > maxImm32 {
			return false
		}
	}

	if !isRegDeadAfter(instructions, nextI, reg) {
		return false
	}

	// Assign the correctly sized immediate operand
	if is64 {
		next.Args[1] = Imm(val)
	} else if is16 {
		next.Args[1] = Imm16(int16(val))
	} else if is8 {
		next.Args[1] = Imm8(uint8(val))
	} else {
		next.Args[1] = Imm32(int32(val))
	}

	inst.Flags |= FlagDeleted
	return true
}

// mov reg, [mem]
// <alu> reg, src       (src is reg or imm)
// mov [mem], reg   ->  <alu> [mem], src (collapses 3 instructions into 1)
func ruleFoldReadModifyWriteMem(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMov32 {
		return false
	}
	if !inst.Args[0].IsReg() || !inst.Args[1].IsMem() {
		return false
	}

	reg := inst.Args[0].Reg
	mem := inst.Args[1]

	// Only apply to local stack frame slots (RBP relative, no index)
	if mem.SymbolID != 0 || mem.Scale != 0 || mem.Reg != RBP.Reg {
		return false
	}

	// 1. Next instruction must be an ALU op on reg
	next1I := nextActiveI(instructions, i, false)
	if next1I == -1 {
		return false
	}
	next1 := &instructions[next1I]

	if next1.NumArgs < 2 || !next1.Args[0].IsReg() || next1.Args[0].Reg != reg {
		return false
	}

	is64 := inst.Op == OcMov
	if is64 {
		switch next1.Op {
		case OcAdd, OcSub, OcAnd, OcOr, OcXor:
		default:
			return false
		}
	} else {
		switch next1.Op {
		case OcAdd32, OcSub32, OcAnd32, OcOr32, OcXor32:
		default:
			return false
		}
	}

	src := next1.Args[1]
	if src.IsReg() {
		if src.Reg == reg {
			return false // x86 cannot do add [mem], [mem]
		}
	} else if src.IsImm() {
		if is64 && (src.Val < minImm32 || src.Val > maxImm32) {
			return false
		}
	} else {
		return false // Memory source not supported when destination is memory
	}

	// 2. Third instruction must be storing reg back to the exact same stack slot
	next2I := nextActiveI(instructions, next1I, false)
	if next2I == -1 {
		return false
	}
	next2 := &instructions[next2I]

	if next2.Op != inst.Op {
		return false
	}
	if !next2.Args[0].IsMem() || next2.Args[0] != mem {
		return false
	}
	if !next2.Args[1].IsReg() || next2.Args[1].Reg != reg {
		return false
	}

	// Verify reg is not read downstream
	if !isRegDeadAfter(instructions, next2I, reg) {
		return false
	}

	// Transform: delete both movs, make the ALU op write directly to [mem]
	inst.Flags |= FlagDeleted
	next1.Args[0] = mem
	next2.Flags |= FlagDeleted
	return true
}

// mov reg, [mem]
// <op> dst, reg   -> <op> dst, [mem] (deletes mov if reg is dead)
func ruleFoldMemIntoAluSrc(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMov32 {
		return false
	}
	if !inst.Args[0].IsReg() || !inst.Args[1].IsMem() {
		return false
	}

	reg := inst.Args[0].Reg
	mem := inst.Args[1]

	nextI := nextActiveI(instructions, i, false)
	if nextI == -1 {
		return false
	}
	next := &instructions[nextI]

	if next.NumArgs < 2 || !next.Args[1].IsReg() || next.Args[1].Reg != reg {
		return false
	}

	// Destination MUST be a register (x86 cannot do op [mem], [mem])
	if !next.Args[0].IsReg() || next.Args[0].Reg == reg {
		return false
	}

	// Verify opcode compatibility
	if inst.Op == OcMov {
		switch next.Op {
		case OcAdd, OcSub, OcAnd, OcOr, OcXor, OcCmp, OcImul:
		default:
			return false
		}
	} else {
		switch next.Op {
		case OcAdd32, OcSub32, OcAnd32, OcOr32, OcXor32, OcCmp32, OcImul32:
		default:
			return false
		}
	}

	// Make sure reg is not read downstream
	if !isRegDeadAfter(instructions, nextI, reg) {
		return false
	}

	next.Args[1] = mem
	inst.Flags |= FlagDeleted
	return true
}

// set<cc> al
// movzx eax, al
// test eax, eax
// jne/je .L_target  -> j<cc>/j<inv_cc> .L_target
func ruleEliminateCompareBooleanMaterialization(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if !isSetcc(inst.Op) || !inst.Args[0].IsReg() {
		return false
	}
	reg := inst.Args[0].Reg // e.g. AL (RAX)

	// 1. Next must be movzx reg, al
	next1I := nextActiveI(instructions, i, false)
	if next1I == -1 {
		return false
	}
	next1 := &instructions[next1I]
	if next1.Op != OcMovzx8 || !next1.Args[0].IsReg() || next1.Args[0].Reg != reg ||
		!next1.Args[1].IsReg() || next1.Args[1].Reg != reg {
		return false
	}

	// 2. Next must be test reg, reg
	next2I := nextActiveI(instructions, next1I, false)
	if next2I == -1 {
		return false
	}
	next2 := &instructions[next2I]
	if (next2.Op != OcTest && next2.Op != OcTest32) || !next2.Args[0].IsReg() || next2.Args[0].Reg != reg ||
		!next2.Args[1].IsReg() || next2.Args[1].Reg != reg {
		return false
	}

	// 3. Next must be jne or je
	next3I := nextActiveI(instructions, next2I, false)
	if next3I == -1 {
		return false
	}
	next3 := &instructions[next3I]
	if next3.Op != OcJe && next3.Op != OcJne {
		return false
	}

	// Verify the scratch register is dead after the branch
	if !isRegDeadAfter(instructions, next3I, reg) {
		return false
	}

	// Map set<cc> to j<cc>
	jccOp := setccToJcc(inst.Op)
	if next3.Op == OcJe {
		jccOp = invertJcc(jccOp) // je tests for 0 (false), so invert condition
	}

	// Delete intermediate instructions and convert branch
	inst.Flags |= FlagDeleted
	next1.Flags |= FlagDeleted
	next2.Flags |= FlagDeleted
	next3.Op = jccOp
	return true
}

// push rA
// pop rB  -> mov rB, rA (or DELETE both if rA == rB)
func rulePushPopToMov(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcPush || !inst.Args[0].IsReg() {
		return false
	}

	nextI := nextActiveI(instructions, i, false)
	if nextI == -1 {
		return false
	}
	next := &instructions[nextI]
	if next.Op != OcPop || !next.Args[0].IsReg() {
		return false
	}

	src := inst.Args[0]
	dst := next.Args[0]

	if src.Reg == dst.Reg {
		inst.Flags |= FlagDeleted
		next.Flags |= FlagDeleted
	} else {
		inst.Flags |= FlagDeleted
		next.Op = OcMov
		next.NumArgs = 2
		next.Args[0] = dst
		next.Args[1] = src
	}

	return true
}

// lea reg, [mem]
// <op> ... [reg] ...  -> <op> ... [mem] ... (deletes lea if reg is dead)
func ruleFoldLeaAddressIntoMemory(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcLea || !inst.Args[0].IsReg() || !inst.Args[1].IsMem() {
		return false
	}

	reg := inst.Args[0].Reg
	mem := inst.Args[1]

	// Must be either:
	// A) Local stack frame slot: [rbp - X] with no index
	// B) RIP-relative constant: [rip + symbol] with no index
	isStackSlot := mem.SymbolID == 0 && mem.Scale == 0 && mem.Reg == RBP.Reg
	isRipConst := mem.SymbolID != 0 && mem.Scale == 0

	if !isStackSlot && !isRipConst {
		return false
	}

	foldedAny := false

	// Scan forward up to 8 instructions to fold all uses of [reg]
	for k := i + 1; k < len(instructions) && k <= i+8; k++ {
		next := &instructions[k]
		if next.Flags&FlagDeleted != 0 {
			continue
		}

		if next.Op == OcLabel || next.Op == OcJmp || next.Op == OcCall || next.Op == OcRet || next.Op == OcFuncEnd {
			break
		}

		// Look for operands using [reg] with no displacement or index
		for a := 0; a < int(next.NumArgs); a++ {
			arg := next.Args[a]
			if arg.IsMem() && arg.SymbolID == 0 && arg.Scale == 0 && arg.Val == 0 && arg.Reg == reg {
				targetMem := mem
				targetMem.Size = arg.Size
				next.Args[a] = targetMem
				foldedAny = true
			}
		}

		// If reg is read as a direct register (e.g. add rbx, reg), stop
		if instReadsReg(next, reg) {
			break
		}

		// If reg is overwritten, stop
		if instOverwritesReg(next, reg) {
			break
		}
	}

	// If all uses were folded and reg is dead, delete the lea
	if isRegDeadAfter(instructions, i, reg) {
		inst.Flags |= FlagDeleted
		return true
	}

	return foldedAny
}

// 2. Immediate & ALU simplifications

// cmp reg, 0   -> test reg, reg
// cmp32 reg, 0 -> test32 reg, reg
func ruleCmpZeroToTest(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if (inst.Op != OcCmp && inst.Op != OcCmp32) || !inst.Args[0].IsReg() || !inst.Args[1].IsImm(0) {
		return false
	}

	if inst.Op == OcCmp {
		inst.Op = OcTest
	} else {
		inst.Op = OcTest32
	}

	inst.Args[1] = inst.Args[0]
	return true
}

// add <reg>, 0 <- DELETE
// sub <reg>, 0 <- DELETE
func ruleIdentityArith(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcAdd && inst.Op != OcSub {
		return false
	}

	if !inst.Args[0].IsReg() || !inst.Args[1].IsImm(0) {
		return false
	}

	if !areFlagsUsedBeforeNextDef(instructions, i) {
		inst.Flags |= FlagDeleted
		return true
	}

	return false
}

// imul reg, 1      -> DELETE
// imul reg, 2^k    -> shl reg, k
func ruleImulPowerOfTwoToShl(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcImul && inst.Op != OcImul32 {
		return false
	}
	if !inst.Args[0].IsReg() || !inst.Args[1].IsImm() {
		return false
	}

	val := inst.Args[1].Val
	if val <= 0 || (val&(val-1)) != 0 {
		return false
	}

	if areFlagsUsedBeforeNextDef(instructions, i) {
		return false
	}

	shift := uint8(bits.TrailingZeros64(uint64(val)))
	if shift == 0 {
		if inst.Op == OcImul {
			inst.Flags |= FlagDeleted
			return true
		}

		return false
	}

	if inst.Op == OcImul {
		inst.Op = OcShlImm
	} else {
		inst.Op = OcShlImm32
	}

	inst.Args[1] = Imm8(shift)
	return true
}

// lea rA, [rA]  -> DELETE
// lea rA, [rB]  -> mov rA, rB
func ruleLeaToMovOrDelete(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcLea {
		return false
	}

	dst := inst.Args[0]
	mem := inst.Args[1]

	// Must be [base] with no index, no symbol, and 0 displacement
	if mem.SymbolID == 0 && mem.Scale == 0 && mem.Index == 0 && mem.Val == 0 {
		if dst.Reg == mem.Reg {
			inst.Flags |= FlagDeleted
			return true
		}

		inst.Op = OcMov
		inst.Args[1] = Operand{Kind: OpReg, Reg: mem.Reg, Size: 8}
		return true
	}

	return false
}

// mov rax, 0 -> xor eax, eax
func ruleMovZeroToXor(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMov32 {
		return false
	}

	if !inst.Args[0].IsReg() || !inst.Args[1].IsImm(0) {
		return false
	}

	if !areFlagsUsedBeforeNextDef(instructions, i) {
		inst.Op = OcXor32
		inst.Args[0].Size = 4
		inst.Args[1] = inst.Args[0]
		return true
	}

	return false
}

// xor rax, rax (3 bytes) -> xor eax, eax (2 bytes, zero-extends to 64-bit)
func rule64bitXorZeroTo32bit(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcXor {
		return false
	}

	if inst.Args[0].IsReg() && inst.Args[1].IsReg() && inst.Args[0].Reg == inst.Args[1].Reg {
		inst.Op = OcXor32
		inst.Args[0].Size = 4
		inst.Args[1].Size = 4
		return true
	}

	return false
}

// xorpd xmmA, xmmA -> xorps xmmA, xmmA
func ruleXorpdToXorps(instructions []Instruction, i int) bool {
	inst := &instructions[i]

	if inst.Op == OcXorpd && inst.Args[0].IsXmm() && inst.Args[1].IsXmm() && inst.Args[0].Reg == inst.Args[1].Reg {
		inst.Op = OcXorps
		return true
	}

	return false
}

// and/or/xor dst, src
// test dst, dst       <-- DELETE test
func ruleRedundantTestAfterAlu(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	is64 := inst.Op == OcAnd || inst.Op == OcOr || inst.Op == OcXor
	is32 := inst.Op == OcAnd32 || inst.Op == OcOr32 || inst.Op == OcXor32
	if !is64 && !is32 {
		return false
	}

	if !inst.Args[0].IsReg() {
		return false
	}

	next := nextActive(instructions, i, false)
	if next == nil {
		return false
	}

	reg := inst.Args[0]

	if (is64 && next.Op == OcTest) || (is32 && next.Op == OcTest32) {
		if next.Args[0].IsReg() && next.Args[1].IsReg() &&
			next.Args[0].Reg == reg.Reg && next.Args[1].Reg == reg.Reg {
			next.Flags |= FlagDeleted
			return true
		}
	}

	return false
}

// not reg; not reg  -> DELETE BOTH
func ruleDoubleNot(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcNot && inst.Op != OcNot32 {
		return false
	}
	if !inst.Args[0].IsReg() {
		return false
	}

	next := nextActive(instructions, i, false)
	if next == nil || next.Op != inst.Op {
		return false
	}

	if next.Args[0] == inst.Args[0] {
		inst.Flags |= FlagDeleted
		next.Flags |= FlagDeleted
		return true
	}

	return false
}

// imul reg, 0 -> xor reg32, reg32
func ruleImulZeroToXor(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcImul && inst.Op != OcImul32 {
		return false
	}
	if !inst.Args[0].IsReg() || !inst.Args[1].IsImm(0) {
		return false
	}

	inst.Op = OcXor32
	inst.Args[0].Size = 4
	inst.Args[1] = inst.Args[0]
	return true
}

// cmp/test A, B
// cmp/test A, B  <-- DELETE second comparison
func ruleRedundantComparison(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcCmp && inst.Op != OcCmp32 && inst.Op != OcTest && inst.Op != OcTest32 {
		return false
	}

	next := nextActive(instructions, i, false)
	if next == nil || next.Op != inst.Op {
		return false
	}

	if inst.Args[0] == next.Args[0] && inst.Args[1] == next.Args[1] {
		next.Flags |= FlagDeleted
		return true
	}

	return false
}

// 3. Memory & move redundancies

// mov [rbp - X], rA  <-- DELETE
// mov [rbp - X], rB
func ruleDeadStoreAfterStore(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMov32 {
		return false
	}
	if !inst.Args[0].IsMem() || !inst.Args[1].IsReg() {
		return false
	}

	mem := inst.Args[0]
	// Only apply to local stack slots (RBP relative, no index, no symbol)
	if mem.SymbolID != 0 || mem.Reg != RBP.Reg || mem.Scale != 0 {
		return false
	}

	next := nextActive(instructions, i, false)
	if next == nil || inst.Op != next.Op {
		return false
	}

	if next.Args[0].IsMem() && next.Args[0] == mem && next.Args[1].IsReg() {
		inst.Flags |= FlagDeleted
		return true
	}

	return false
}

// mov rA, [mem]
// mov rB, [mem]  -> mov rB, rA (or DELETE if rA == rB)
func ruleRedundantLoadAfterLoad(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMov32 {
		return false
	}
	if !inst.Args[0].IsReg() || !inst.Args[1].IsMem() {
		return false
	}

	dst := inst.Args[0]
	mem := inst.Args[1]

	// Ensure destination doesn't clobber the address base or index
	if mem.SymbolID == 0 && (dst.Reg == mem.Reg || (mem.Scale != 0 && dst.Reg == mem.Index)) {
		return false
	}

	next := nextActive(instructions, i, false)
	if next == nil || next.Op != inst.Op {
		return false
	}

	if next.Args[0].IsReg() && next.Args[1].IsMem() && next.Args[1] == mem {
		if next.Args[0] == dst {
			next.Flags |= FlagDeleted
		} else {
			next.Args[1] = dst
		}
		return true
	}

	return false
}

// mov [rbp - X], <reg:0>
// mov <reg:0>, [rbp - X] <-- DELETE
func ruleRedundantStoreLoad(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMov32 && inst.Op != OcMovss && inst.Op != OcMovsd {
		return false
	}

	if !inst.Args[0].IsMem() || (!inst.Args[1].IsReg() && !inst.Args[1].IsXmm()) {
		return false
	}

	nextI := nextActiveI(instructions, i, false)
	if nextI == -1 {
		return false
	}
	next := &instructions[nextI]

	// Must be identical opcode (same width and type)
	if next.Op != inst.Op {
		return false
	}

	if (next.Args[0].IsReg() || next.Args[0].IsXmm()) && next.Args[1].IsMem() {
		// Register stored must match register loaded
		if inst.Args[1] != next.Args[0] {
			return false
		}

		// Memory stored to must match memory loaded from
		if isSameMem(inst.Args[0], next.Args[1]) {
			next.Flags |= FlagDeleted
			return true
		}
	}

	return false
}

// mov8  [mem], reg8
// movzx regA, [mem]   ->  movzx regA, reg8   (eliminates stack load, preserves zero-extension)
//
// mov16 [mem], reg16
// movzx regA, [mem]   ->  movzx regA, reg16
func ruleStoreLoadForwardNarrow(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov8 && inst.Op != OcMov16 {
		return false
	}
	if !inst.Args[0].IsMem() || !inst.Args[1].IsReg() {
		return false
	}

	mem := inst.Args[0]
	srcReg := inst.Args[1]

	nextI := nextActiveI(instructions, i, false)
	if nextI == -1 {
		return false
	}
	next := &instructions[nextI]

	// Match 8-bit or 16-bit stores followed by zero/sign extensions
	isMatch := false
	if inst.Op == OcMov8 {
		isMatch = next.Op == OcMovzx8 || next.Op == OcMovsx8 || next.Op == OcMovsx8to32
	} else if inst.Op == OcMov16 {
		isMatch = next.Op == OcMovzx16 || next.Op == OcMovsx16 || next.Op == OcMovsx16to32
	}

	if !isMatch || !next.Args[1].IsMem() || !next.Args[0].IsReg() {
		return false
	}

	// Must be the exact same stack slot (base RBP, identical offset, no index)
	nextMem := next.Args[1]
	if nextMem.Reg != mem.Reg || nextMem.Val != mem.Val ||
		nextMem.Scale != 0 || mem.Scale != 0 ||
		nextMem.SymbolID != 0 || mem.SymbolID != 0 {
		return false
	}

	// Forward the register: turns movzx regA, [mem] into movzx regA, regB
	next.Args[1] = srcReg
	return true
}

// mov [rbp - 8], <reg:0>
// mov <reg:1>, [rbp - 8]
//
//	->
//
// mov [rbp - 8], <reg:0>
// mov <reg:1>, <reg:0>
func ruleStoreLoadDifferentReg(instructions []Instruction, i int) bool {
	inst := &instructions[i]

	if inst.Op != OcMov && inst.Op != OcMov32 && inst.Op != OcMovss && inst.Op != OcMovsd {
		return false
	}
	if !inst.Args[0].IsMem() || (!inst.Args[1].IsReg() && !inst.Args[1].IsXmm()) {
		return false
	}

	next := nextActive(instructions, i, false)
	if next == nil || next.Op != inst.Op {
		return false
	}

	// Match: mov regB, [mem] where [mem] matches inst.Args[0]
	if (next.Args[0].IsReg() || next.Args[0].IsXmm()) && next.Args[1].IsMem() {
		if isSameMem(inst.Args[0], next.Args[1]) && inst.Args[1] != next.Args[0] {
			next.Args[1] = inst.Args[1]
			return true
		}
	}

	return false
}

// mov rA, rB
// mov rB, rA  <-- DELETE
func ruleRedundantMoveChain(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMovss && inst.Op != OcMovsd {
		return false
	}

	if (!inst.Args[0].IsReg() && !inst.Args[0].IsXmm()) || (!inst.Args[1].IsReg() && !inst.Args[1].IsXmm()) {
		return false
	}

	next := nextActive(instructions, i, false)
	if next == nil || next.Op != inst.Op {
		return false
	}

	if inst.Args[0] == next.Args[1] && inst.Args[1] == next.Args[0] {
		next.Flags |= FlagDeleted
		return true
	}

	return false
}

// mov   <reg:0>, <reg:0> <- DELETE
// movss <reg:0>, <reg:0> <- DELETE
// movsd <reg:0>, <reg:0> <- DELETE
func ruleRedundantSelfMove(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMov && inst.Op != OcMovss && inst.Op != OcMovsd {
		return false
	}

	// Note: We do NOT delete mov eax, eax because 32-bit mov explicitly zeroes upper 32-bits!
	if inst.Args[0].IsReg() && inst.Args[1].IsReg() && inst.Args[0].Reg == inst.Args[1].Reg {
		inst.Flags |= FlagDeleted
		return true
	}

	if inst.Args[0].IsXmm() && inst.Args[1].IsXmm() && inst.Args[0].Reg == inst.Args[1].Reg {
		inst.Flags |= FlagDeleted
		return true
	}

	return false
}

// movzx rax, al
// ... (instructions that don't touch rax)
// movzx rax, al  <-- DELETE (rax is already zero-extended from al)
func ruleRedundantRepeatedMovzx(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcMovzx8 && inst.Op != OcMovzx16 {
		return false
	}
	if !inst.Args[0].IsReg() {
		return false
	}

	dst := inst.Args[0].Reg

	for k := i + 1; k < len(instructions) && k <= i+4; k++ {
		next := &instructions[k]
		if next.Flags&FlagDeleted != 0 {
			continue
		}

		if next.Op == OcLabel || next.Op == OcJmp || next.Op == OcCall || next.Op == OcRet || next.Op == OcFuncEnd {
			break
		}

		// If next is movzx dst, dst (e.g. movzx rax, ax or movzx rax, al)
		// and dst was already zero-extended by inst and not modified:
		if next.Op == inst.Op && next.Args[0].IsReg() && next.Args[0].Reg == dst &&
			next.Args[1].IsReg() && next.Args[1].Reg == dst {
			next.Flags |= FlagDeleted
			return true
		}

		if instModifiesReg(next, dst) {
			break
		}
	}

	return false
}

// 4. Control flow cleanups

// jne .L_skip
// jmp .L_target
// .L_skip:
//
//	->
//
// je .L_target
// .L_skip:
func ruleInvertBranchOverJmp(instructions []Instruction, i int) bool {
	inst := &instructions[i]

	invOp := invertJcc(inst.Op)
	if invOp == OcNop {
		return false
	}

	// 1. Find the unconditional jmp
	jmpI := nextActiveI(instructions, i, false)
	if jmpI == -1 {
		return false
	}

	jmp := &instructions[jmpI]
	if jmp.Op != OcJmp || !jmp.Args[0].IsLabel() {
		return false
	}

	// 2. Find the label immediately following the jmp
	label := nextActive(instructions, jmpI, true)
	if label == nil || label.Op != OcLabel {
		return false
	}

	labelSkipID := label.Args[0].Val
	branchTargetID := inst.Args[0].Val

	// If the conditional branch jumps over the jmp straight to the label:
	if branchTargetID == labelSkipID {
		inst.Op = invOp
		inst.Args[0] = jmp.Args[0]
		jmp.Flags |= FlagDeleted
		return true
	}

	return false
}

// jmp .L1  <-- DELETE jmp (and delete .L1 if it has no other incoming branches)
// .L1:
func ruleRedundantBranchToNextLabel(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if (inst.Op != OcJmp && invertJcc(inst.Op) == OcNop) || !inst.Args[0].IsLabel() {
		return false
	}

	next := nextActive(instructions, i, true)
	if next == nil {
		return false
	}

	labelID := inst.Args[0].Val

	if next.Op == OcLabel && labelID == next.Args[0].Val {
		inst.Flags |= FlagDeleted

		// NEVER delete a label if it has an attached symbol (e.g. function entry)!
		// Only delete purely internal, unreferenced branch labels.
		if !isLabelReferenced(instructions, labelID) && next.Args[0].SymbolID == 0 {
			next.Flags |= FlagDeleted
		}

		return true
	}

	return false
}

// jcc .L1
// jmp .L1   -> DELETE jcc
func ruleBranchToSameTargetAsJmp(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if invertJcc(inst.Op) == OcNop || !inst.Args[0].IsLabel() {
		return false
	}

	next := nextActive(instructions, i, false)
	if next == nil || next.Op != OcJmp || !next.Args[0].IsLabel() {
		return false
	}

	if inst.Args[0].Val == next.Args[0].Val {
		inst.Flags |= FlagDeleted
		return true
	}

	return false
}

// jmp .L_target  (or ret)
// <any>          <-- DELETE ALL until next label/align/funcend
func ruleUnreachableAfterTransfer(instructions []Instruction, i int) bool {
	inst := &instructions[i]
	if inst.Op != OcJmp && inst.Op != OcRet {
		return false
	}

	deleted := false
	for {
		next := nextActive(instructions, i, true)
		if next == nil {
			break
		}

		// Stop when reaching code targets or alignment directives
		if next.Op == OcLabel || next.Op == OcFuncEnd || next.Op == OcAlign {
			break
		}

		next.Flags |= FlagDeleted
		deleted = true
	}

	return deleted
}

// Utils

func setccToJcc(op Opcode) Opcode {
	switch op {
	case OcSete:
		return OcJe
	case OcSetne:
		return OcJne
	case OcSetl:
		return OcJl
	case OcSetle:
		return OcJle
	case OcSetg:
		return OcJg
	case OcSetge:
		return OcJge
	case OcSetb:
		return OcJb
	case OcSetbe:
		return OcJbe
	case OcSeta:
		return OcJa
	case OcSetae:
		return OcJae
	case OcSetp:
		return OcJp
	case OcSetnp:
		return OcJnp
	default:
		return OcNop
	}
}

func isRegDeadAfter(instructions []Instruction, from int, reg Register) bool {
	for i := from + 1; i < len(instructions); i++ {
		inst := &instructions[i]
		if inst.Flags&FlagDeleted != 0 {
			continue
		}

		// 1. If an instruction reads reg -> reg is ALIVE!
		if instReadsReg(inst, reg) {
			return false
		}

		// 2. If an instruction purely overwrites reg -> reg was DEAD!
		if instOverwritesReg(inst, reg) {
			return true
		}

		// 3. ABI Return boundary:
		// Under any x86-64 register allocator, caller-saved registers
		// (except return registers RAX and RDX) are NEVER live out of a function!
		if inst.Op == OcRet {
			return isCallerSaved(reg) && reg != RAX.Reg && reg != RDX.Reg
		}
		if inst.Op == OcFuncEnd {
			return true
		}

		// 4. Handle redundant jumps to the immediately following label (fallthrough no-op)
		if inst.Op == OcJmp && inst.Args[0].IsLabel() {
			nextActive := nextActive(instructions, i, true)
			if nextActive != nil && nextActive.Op == OcLabel && inst.Args[0].Val == nextActive.Args[0].Val {
				continue // Skip the dummy jump and keep scanning down the fallthrough path!
			}
			return false // Real jump away to an unknown block
		}

		// 5. Fallthrough labels:
		// A label does not diverge control flow when execution falls into it.
		// Keep scanning to verify whether instructions along this path read reg!
		if inst.Op == OcLabel {
			continue
		}

		// 6. Real control flow divergence:
		// Calls might read argument registers, and conditional branches diverge control flow.
		if inst.Op == OcCall || invertJcc(inst.Op) != OcNop {
			return false
		}
	}

	return false
}
func isCallerSaved(reg Register) bool {
	switch reg {
	case RAX.Reg, RCX.Reg, RDX.Reg, RSI.Reg, RDI.Reg,
		R8.Reg, R9.Reg, R10.Reg, R11.Reg:
		return true

	default:
		return false // RBX, RBP, RSP, R12-R15 are callee-saved (non-volatile)
	}
}

func instReadsReg(inst *Instruction, reg Register) bool {
	// xor reg, reg zeroes the register; it does NOT read the prior value of reg
	if (inst.Op == OcXor || inst.Op == OcXor32) &&
		inst.Args[0].IsReg() && inst.Args[1].IsReg() &&
		inst.Args[0].Reg == reg && inst.Args[1].Reg == reg {
		return false
	}

	// Check memory operands (base or index)
	for _, arg := range inst.Args[:inst.NumArgs] {
		if arg.IsMem() && arg.SymbolID == 0 {
			if arg.Reg == reg || (arg.Scale != 0 && arg.Index == reg) {
				return true
			}
		}
	}

	// Check explicit source operands (Args[1], Args[2])
	if inst.NumArgs >= 2 && inst.Args[1].IsReg() && inst.Args[1].Reg == reg {
		return true
	}
	if inst.NumArgs >= 3 && inst.Args[2].IsReg() && inst.Args[2].Reg == reg {
		return true
	}

	// Check destination operand (Args[0]):
	// Most instructions read Args[0] (e.g. add, sub, cmp, test, push)
	if inst.NumArgs >= 1 && inst.Args[0].IsReg() && inst.Args[0].Reg == reg {
		if !isPureDefOp(inst.Op) {
			return true
		}
	}

	// Implicit register reads
	switch inst.Op {
	case OcMul, OcMul32, OcDiv, OcDiv32, OcIdiv, OcIdiv32, OcCdq, OcCqo:
		if reg == RAX.Reg || reg == RDX.Reg {
			return true
		}
	case OcShlCl, OcShrCl, OcSarCl, OcShlCl32, OcShrCl32, OcSarCl32:
		if reg == RCX.Reg {
			return true
		}
	case OcSyscall:
		if reg == RAX.Reg || reg == RDI.Reg || reg == RSI.Reg || reg == RDX.Reg || reg == R10.Reg || reg == R8.Reg || reg == R9.Reg {
			return true
		}
	default:
	}

	return false
}

func instOverwritesReg(inst *Instruction, reg Register) bool {
	// xor reg, reg zeroes reg without reading prior value
	if (inst.Op == OcXor || inst.Op == OcXor32) && inst.Args[0].IsReg() && inst.Args[1].IsReg() {
		if inst.Args[0].Reg == reg && inst.Args[1].Reg == reg {
			return true
		}
	}

	// Pure overwrite instructions where Args[0] is reg
	if isPureDefOp(inst.Op) && inst.Args[0].IsReg() && inst.Args[0].Reg == reg {
		// As long as the source didn't also read reg (e.g. mov rax, rax)
		if inst.NumArgs < 2 || !inst.Args[1].IsReg() || inst.Args[1].Reg != reg {
			return true
		}
	}

	return false
}

func instModifiesReg(inst *Instruction, reg Register) bool {
	// 1. Pure overwrites (mov, lea, pop, xor reg, reg)
	if instOverwritesReg(inst, reg) {
		return true
	}

	// 2. Instructions that write to their destination register (Args[0])
	// (Check if Args[0] is reg, excluding read-only ops like cmp, test, push, jumps)
	if inst.NumArgs >= 1 && inst.Args[0].IsReg() && inst.Args[0].Reg == reg {
		switch inst.Op {
		// Read-only operations where Args[0] is NOT written to:
		case OcCmp, OcCmp32, OcTest, OcTest32,
			OcPush, OcJmp, OcCall,
			OcUcomiss, OcUcomisd:
			// These only read Args[0] or set flags; they do NOT modify reg.
			return false

		default:
			// ALU (add, sub, and, or, xor, not, neg, imul), shifts, setcc, etc.
			// all modify Args[0]!
			return true
		}
	}

	// 3. Implicit register writers in x86
	switch inst.Op {
	case OcMul, OcMul32, OcDiv, OcDiv32, OcIdiv, OcIdiv32:
		// These implicitly write to RAX and RDX
		if reg == RAX.Reg || reg == RDX.Reg {
			return true
		}

	case OcCdq, OcCqo:
		// Implicitly writes to RDX
		if reg == RDX.Reg {
			return true
		}

	case OcSyscall:
		// Syscall clobbers RAX, RCX, R11
		if reg == RAX.Reg || reg == RCX.Reg || reg == R11.Reg {
			return true
		}

	default:
	}

	return false
}

func isPureDefOp(op Opcode) bool {
	switch op {
	case OcMov, OcMov32,
		OcMovzx8, OcMovzx16,
		OcMovsx8, OcMovsx8to32, OcMovsx16, OcMovsx16to32,
		OcMovsxd, OcLea, OcPop:
		return true

	default:
		return false
	}
}

func isLabelReferenced(instructions []Instruction, labelID int64) bool {
	for i := range instructions {
		inst := &instructions[i]
		if inst.Flags&FlagDeleted != 0 {
			continue
		}

		if (inst.Op == OcJmp || inst.Op == OcCall || invertJcc(inst.Op) != OcNop) && inst.Args[0].IsLabel() {
			if inst.Args[0].Val == labelID {
				return true
			}
		}
	}

	return false
}

func invertJcc(op Opcode) Opcode {
	switch op {
	case OcJe:
		return OcJne
	case OcJne:
		return OcJe
	case OcJl:
		return OcJge
	case OcJge:
		return OcJl
	case OcJle:
		return OcJg
	case OcJg:
		return OcJle
	case OcJb:
		return OcJae
	case OcJae:
		return OcJb
	case OcJbe:
		return OcJa
	case OcJa:
		return OcJbe
	case OcJs:
		return OcJns
	case OcJns:
		return OcJs
	case OcJp:
		return OcJnp
	case OcJnp:
		return OcJp
	default:
		return OcNop
	}
}

func areFlagsUsedBeforeNextDef(instructions []Instruction, from int) bool {
	for i := from + 1; i < len(instructions); i++ {
		inst := &instructions[i]
		if inst.Flags&FlagDeleted != 0 {
			continue
		}

		// 1. If instruction reads EFLAGS, flags ARE used!
		if invertJcc(inst.Op) != OcNop || isSetcc(inst.Op) {
			return true
		}

		// 2. If instruction overwrites EFLAGS, old flags are DEAD!
		if clobbersFlags(inst.Op) {
			return false
		}

		// 3. Stop at control-flow boundaries
		if inst.Op == OcLabel || inst.Op == OcCall || inst.Op == OcJmp {
			return true // conservatively assume live across labels / calls
		}
		if inst.Op == OcRet || inst.Op == OcFuncEnd {
			return false // flags are never live after return
		}
	}

	return false
}

func isSetcc(op Opcode) bool {
	return op >= OcSetb && op <= OcSetg
}

func clobbersFlags(op Opcode) bool {
	switch op {
	case OcAdd, OcAdd32, OcSub, OcSub32,
		OcMul, OcMul32, OcImul, OcImul32,
		OcDiv, OcDiv32, OcIdiv, OcIdiv32,
		OcNeg, OcNeg32,
		OcAnd, OcAnd32, OcOr, OcOr32, OcXor, OcXor32,
		OcTest, OcTest32, OcCmp, OcCmp32,
		OcUcomiss, OcUcomisd:
		return true

	default:
		return false
	}
}

func isSameMem(a, b Operand) bool {
	return a.Kind == OpMem && b.Kind == OpMem &&
		a.Reg == b.Reg &&
		a.Val == b.Val &&
		a.Index == b.Index &&
		a.Scale == b.Scale &&
		a.SymbolID == b.SymbolID
}
