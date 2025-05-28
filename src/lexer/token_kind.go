package lexer

type TokenKind uint8

const (
	Error TokenKind = iota

	Identifier
	Number
	String

	Equal
	EqualEqual
	Bang
	BangEqual

	Pipe
	PipeEqual
	PipePipe
	Xor
	XorEqual
	Ampersand
	AmpersandEqual
	AmpersandAmpersand

	Less
	LessEqual
	Greater
	GreaterEqual

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

	LeftParen
	RightParen
	LeftBrace
	RightBrace
	LeftBracket
	RightBracket

	Dot
	DotDotDot
	Comma
	Colon
	Semicolon
	Hashtag

	Eof
)

func (t TokenKind) IsOperator() bool {
	return t >= Equal && t <= PercentageEqual
}

func (t TokenKind) String() string {
	switch t {
	case Error:
		return "Error"

	case Identifier:
		return "Identifier"
	case Number:
		return "Number"
	case String:
		return "String"

	case Equal:
		return "Equal"
	case EqualEqual:
		return "EqualEqual"
	case Bang:
		return "Bang"
	case BangEqual:
		return "BangEqual"

	case Pipe:
		return "Pipe"
	case PipeEqual:
		return "PipeEqual"
	case PipePipe:
		return "PipePipe"
	case Xor:
		return "Xor"
	case XorEqual:
		return "XorEqual"
	case Ampersand:
		return "Ampersand"
	case AmpersandEqual:
		return "AmpersandEqual"
	case AmpersandAmpersand:
		return "AmpersandAmpersand"

	case Less:
		return "Less"
	case LessEqual:
		return "LessEqual"
	case Greater:
		return "Greater"
	case GreaterEqual:
		return "GreaterEqual"

	case Plus:
		return "Plus"
	case PlusEqual:
		return "PlusEqual"
	case PlusPlus:
		return "PlusPlus"
	case Minus:
		return "Minus"
	case MinusEqual:
		return "MinusEqual"
	case MinusMinus:
		return "MinusMinus"
	case Star:
		return "Star"
	case StarEqual:
		return "StarEqual"
	case Slash:
		return "Slash"
	case SlashEqual:
		return "SlashEqual"
	case Percentage:
		return "Percentage"
	case PercentageEqual:
		return "PercentageEqual"

	case LeftParen:
		return "LeftParen"
	case RightParen:
		return "RightParen"
	case LeftBrace:
		return "LeftBrace"
	case RightBrace:
		return "RightBrace"
	case LeftBracket:
		return "LeftBracket"
	case RightBracket:
		return "RightBracket"

	case Dot:
		return "Dot"
	case DotDotDot:
		return "DotDotDot"
	case Comma:
		return "Comma"
	case Colon:
		return "Colon"
	case Semicolon:
		return "Semicolon"
	case Hashtag:
		return "Hashtag"

	case Eof:
		return "EOF"
	default:
		panic("lexer.TokenKind.String() - Invalid")
	}
}
