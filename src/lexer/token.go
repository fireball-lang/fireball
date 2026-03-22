package lexer

import "fireball/core"

type TokenKind uint8

const (
	EOF TokenKind = iota
	Error

	// Literals

	BinaryInteger
	HexInteger

	UnsignedInteger
	SignedInteger

	Decimal
	Decimal32bit

	Character
	String

	// Keywords

	True
	False

	Struct
	Func

	Var
	If
	Else
	While
	For

	Return
	Break
	Continue

	// Misc

	Identifier

	Dot
	DotDotDot
	Comma
	Colon
	Semicolon
	Hashtag

	LeftParen
	RightParen

	LeftBracket
	RightBracket

	LeftBrace
	RightBrace

	// Operators

	Plus
	PlusEqual
	PlusPlus

	Minus
	MinusEqual
	MinusMinus

	Star
	StarEqual

	Slash
	SlashEqual

	Percentage
	PercentageEqual

	Pipe
	PipeEqual
	PipePipe

	Caret
	CaretEqual

	Ampersand
	AmpersandEqual
	AmpersandAmpersand

	Equal
	EqualEqual

	Bang
	BangEqual

	Less
	LessEqual
	LessLess
	LessLessEqual

	Greater
	GreaterEqual
	GreaterGreater
	GreaterGreaterEqual

	// Last

	Last
)

type Token struct {
	Kind  TokenKind
	Text  string
	Range core.Range
}

func IsInteger(kind TokenKind) bool {
	return kind >= BinaryInteger && kind <= SignedInteger
}
