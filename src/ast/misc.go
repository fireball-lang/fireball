package ast

import (
	"fireball/lexer"
	"iter"
)

// NameType

type NameType struct {
	baseNode

	Name *Leaf
	Type Type
}

func (f *NameType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(f.Name) {
			return
		}
		yield(f.Type)
	}
}

// Leaf

type Leaf struct {
	baseLeafNode

	Token lexer.Token
}

func (l *Leaf) Range() lexer.Range {
	return l.Token.Range
}
