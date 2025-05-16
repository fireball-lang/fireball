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
