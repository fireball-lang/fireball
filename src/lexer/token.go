package lexer

import (
	"fmt"
)

type Pos struct {
	Line   uint
	Column uint
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

type Token struct {
	Kind  TokenKind
	Text  string
	Range Range
}

func (t Token) String() string {
	return fmt.Sprintf("%s '%s' (%s - %s)", t.Kind, t.Text, t.Range.Start, t.Range.End)
}
