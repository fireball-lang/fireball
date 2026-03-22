package ast

import (
	"fireball/core"
	"iter"
)

type Node interface {
	Parent() Node
	Children() iter.Seq[Node]

	Range() core.Range
}

// baseNode

type baseNode struct {
	Parent_ Node
	Range_  core.Range
}

func (b *baseNode) Parent() Node {
	return b.Parent_
}

func (b *baseNode) Range() core.Range {
	return b.Range_
}

// baseLeafNode

type baseLeafNode struct {
	Parent_ Node
}

func (b *baseLeafNode) Parent() Node {
	return b.Parent_
}

func (b *baseLeafNode) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}
