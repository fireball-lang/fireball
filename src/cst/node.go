package cst

import (
	"fireball/lexer"
	"fmt"
	"math"
)

type Node struct {
	Kind     NodeKind
	Children []Node

	Token lexer.Token
	Range lexer.Range
}

func (n *Node) Valid() bool {
	return n.Token != (lexer.Token{}) || len(n.Children) > 0
}

func (n *Node) String() string {
	if n.Kind == Leaf {
		return "Leaf - " + n.Token.String()
	}

	return fmt.Sprintf("%s - %d (%s - %s)", n.Kind, len(n.Children), n.Range.Start, n.Range.End)
}

func (n *Node) append(child Node) {
	if child.Valid() {
		n.Children = append(n.Children, child)
	}
}

func (n *Node) calculateRange() {
	if n.Kind == Leaf {
		n.Range = n.Token.Range
		return
	}

	start := lexer.Pos{
		Line:   math.MaxUint,
		Column: math.MaxUint,
	}

	end := lexer.Pos{
		Line:   0,
		Column: 0,
	}

	for _, child := range n.Children {
		start = lexer.Min(start, child.Range.Start)
		end = lexer.Max(end, child.Range.End)
	}

	n.Range = lexer.Range{Start: start, End: end}
}
