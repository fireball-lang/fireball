package ast

import (
	"fireball/core"
	"fireball/lexer"
	"iter"
)

// IdentifierEntry

type IdentifierEntry struct {
	baseNode

	Name     *Leaf
	TypeArgs []Type
}

func (i *IdentifierEntry) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(i.Name) {
			return
		}
		for _, arg := range i.TypeArgs {
			if !yield(arg) {
				return
			}
		}
	}
}

// Leaf

type Leaf struct {
	baseLeafNode

	Token lexer.Token
}

func (l *Leaf) Range() core.Range {
	return l.Token.Range
}
