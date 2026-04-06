package ast

import (
	"fireball/core"
	"iter"
)

type Node interface {
	Parent() Node
	SetParent(parent Node)

	Children() iter.Seq[Node]

	Range() core.Range
}

// baseNode

type baseNode struct {
	parent Node
	Range_ core.Range
}

func (b *baseNode) Parent() Node {
	return b.parent
}

func (b *baseNode) SetParent(parent Node) {
	b.parent = parent
}

func (b *baseNode) Range() core.Range {
	return b.Range_
}

// baseLeafNode

type baseLeafNode struct {
	parent Node
}

func (b *baseLeafNode) Parent() Node {
	return b.parent
}

func (b *baseLeafNode) SetParent(parent Node) {
	b.parent = parent
}

func (b *baseLeafNode) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}
