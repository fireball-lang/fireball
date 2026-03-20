package lexer

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
	While
	For

	// Misc

	Identifier

	Dot
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

	Ampersand
	AmpersandEqual
	AmpersandAmpersand

	Pipe
	PipeEqual
	PipePipe

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
)

type Token struct {
	Kind  TokenKind
	Text  string
	Range Range
}

type Pos struct {
	Line   uint16
	Column uint16
}

type Range struct {
	Start, End Pos
}
