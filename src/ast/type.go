package ast

import (
	"fireball/lexer"
	"iter"
)

type Type interface {
	Node
}

// DeclType

type DeclType struct {
	baseNode

	Name *Leaf
}

func (d *DeclType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if d.Name != nil && !yield(d.Name) {
			return
		}
	}
}

func (d *DeclType) Range() lexer.Range {
	return d.Name.Range()
}
