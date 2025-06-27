package lexer

import (
	"fmt"
)

type Pos struct {
	Line   uint32
	Column uint32
}

func (p Pos) LessThan(o Pos) bool {
	if p.Line < o.Line {
		return true
	}

	if p.Line == o.Line {
		return p.Column < o.Column
	}

	return false
}

func (p Pos) GreaterThanEqual(o Pos) bool {
	if p.Line > o.Line {
		return true
	}

	if p.Line == o.Line {
		return p.Column >= o.Column
	}

	return false
}

func (p Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

func Min(p1, p2 Pos) Pos {
	return Pos{
		Line:   min(p1.Line, p2.Line),
		Column: min(p1.Column, p2.Column),
	}
}

func Max(p1, p2 Pos) Pos {
	return Pos{
		Line:   max(p1.Line, p2.Line),
		Column: max(p1.Column, p2.Column),
	}
}

type Range struct {
	Start Pos
	End   Pos
}

func (r Range) IsZero() bool {
	var zero Range
	return r == zero
}

func (r Range) Contains(pos Pos) bool {
	return pos.GreaterThanEqual(r.Start) && pos.LessThan(r.End)
}

type Token struct {
	Kind  TokenKind
	Text  string
	Range Range
}

func (t Token) String() string {
	return fmt.Sprintf("%s '%s' (%s - %s)", t.Kind, t.Text, t.Range.Start, t.Range.End)
}
