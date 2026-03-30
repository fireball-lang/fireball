package abi

import "fireball/types"

type Class uint8

const (
	Integer Class = iota + 1
	Float
	Memory
)

type Info struct {
	Size    uint32
	Align   uint32
	Offsets []uint32
}

type Arch interface {
	Info(typ types.Type) Info
}

type CallConv interface {
	Classify(arch Arch, typ types.Type) ([]Class, Info)
}
