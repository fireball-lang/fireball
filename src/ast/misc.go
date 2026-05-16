package ast

import (
	"fireball/core"
	"fireball/lexer"
	"iter"
	"strings"
)

// IdentifierPath

type IdentifierPath struct {
	baseNode

	Entries []*Leaf
}

func (i *IdentifierPath) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, entry := range i.Entries {
			if !yield(entry) {
				return
			}
		}
	}
}

func (i *IdentifierPath) String() string {
	sb := strings.Builder{}

	for index, entry := range i.Entries {
		if index > 0 {
			sb.WriteString("::")
		}

		sb.WriteString(entry.Token.Text)
	}

	return sb.String()
}

func (i *IdentifierPath) LastName() string {
	return i.Entries[len(i.Entries)-1].Token.Text
}

// Leaf

type Leaf struct {
	baseLeafNode

	Token lexer.Token
}

func (l *Leaf) Range() core.Range {
	return l.Token.Range
}
