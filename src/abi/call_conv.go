package abi

import "fireball/ast"

type Class uint8

const (
	None Class = iota
	Integer
	SSE
	Memory
)

type Reg struct {
	Class Class
	Size  uint32
}

type CallConv interface {
	Classify(type_ ast.Type) []Reg
}
