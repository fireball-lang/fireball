package asm

import (
	"fireball/core"
)

type ruleFunc func(instructions []Instruction, i int) bool

var rules = []ruleFunc{
	// 1. Immediate & memory folding
	ruleFoldMovImmIntoOp,
	ruleFoldReadModifyWriteMem,
	ruleFoldMemIntoAluSrc,
	ruleEliminateCompareBooleanMaterialization,
	rulePushPopToMov,
	ruleFoldLeaAddressIntoMemory,

	// 2. Immediate & ALU simplifications
	ruleCmpZeroToTest,
	ruleIdentityArith,
	ruleImulPowerOfTwoToShl,
	ruleLeaToMovOrDelete,
	ruleMovZeroToXor,
	rule64bitXorZeroTo32bit,
	ruleXorpdToXorps,
	ruleRedundantTestAfterAlu,
	ruleDoubleNot,
	ruleImulZeroToXor,
	ruleRedundantComparison,

	// 3. Memory & move redundancies
	ruleDeadStoreAfterStore,
	ruleRedundantLoadAfterLoad,
	ruleRedundantStoreLoad,
	ruleStoreLoadForwardNarrow,
	ruleStoreLoadDifferentReg,
	ruleRedundantMoveChain,
	ruleRedundantSelfMove,
	ruleRedundantRepeatedMovzx,

	// 4. Control flow cleanups
	ruleInvertBranchOverJmp,
	ruleRedundantBranchToNextLabel,
	ruleBranchToSameTargetAsJmp,
	ruleUnreachableAfterTransfer,
}

func PeepholeOptimize(instructions []Instruction, passes uint8) {
	if passes == 0 {
		return
	}

	defer core.Scope()()
	i := 0

	for range passes {
		changed := false

		for i := 0; i < len(instructions); i++ {
			if instructions[i].Flags&FlagDeleted != 0 {
				continue
			}

			for _, rule := range rules {
				if rule(instructions, i) {
					changed = true

					if prev := prevActiveI(instructions, i); prev != -1 {
						i = prev - 1
					}

					break
				}
			}
		}

		i++

		if !changed {
			break
		}
	}
}

// Utils

func nextActiveI(instructions []Instruction, from int, allowCrossLabel bool) int {
	for i := from + 1; i < len(instructions); i++ {
		inst := &instructions[i]

		if inst.Flags&FlagDeleted != 0 {
			continue
		}

		if !allowCrossLabel && inst.Op == OcLabel {
			return -1
		}

		return i
	}

	return -1
}

func nextActive(instructions []Instruction, from int, allowCrossLabel bool) *Instruction {
	if i := nextActiveI(instructions, from, allowCrossLabel); i != -1 {
		return &instructions[i]
	}

	return nil
}

func prevActiveI(instructions []Instruction, from int) int {
	for i := from - 1; i >= 0; i-- {
		inst := &instructions[i]

		if inst.Flags&FlagDeleted != 0 {
			continue
		}

		if inst.Op == OcLabel {
			return -1
		}

		return i
	}

	return -1
}

func prevActive(instructions []Instruction, from int) *Instruction {
	if i := prevActiveI(instructions, from); i != -1 {
		return &instructions[i]
	}

	return nil
}
