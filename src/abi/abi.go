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

type ABI interface {
	Info(typ types.Type) Info

	Classify(typ types.Type) ([]Class, Info)
}
