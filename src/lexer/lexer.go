package lexer

import (
	"bufio"
	"fireball/core"
	"io"
)

type Lexer struct {
	reader *bufio.Reader
	buffer core.Ring[rune]

	pos core.Pos

	start     core.Pos
	tokenText []rune
}

func New(reader io.Reader) *Lexer {
	return &Lexer{
		reader: bufio.NewReader(reader),
		buffer: core.NewRing[rune](4),
		pos:    core.Pos{Line: 1, Column: 1},
	}
}

func (l *Lexer) Next() Token {
	l.start = l.pos

	if msg := l.skipWhitespace(); msg != "" {
		return l.makeError(msg)
	}

	l.start = l.pos
	l.tokenText = l.tokenText[:0]

	ch := l.advance()

	if ch == '\000' {
		return l.make(EOF)
	}

	// Literals

	if ch == '0' && (l.match('b') || l.match('B')) {
		return l.binaryInteger()
	}
	if ch == '0' && (l.match('x') || l.match('X')) {
		return l.hexInteger()
	}
	if isDigit(ch) {
		return l.number()
	}

	if ch == '\'' {
		return l.character()
	}
	if ch == '"' {
		return l.string()
	}

	// Keywords / Identifiers

	if isAlpha(ch) {
		return l.keywordIdentifier()
	}

	switch ch {
	// Misc

	case '.':
		if l.peek(0) == '.' && l.peek(1) == '.' {
			l.advance()
			l.advance()
			return l.make(DotDotDot)
		}
		return l.make(Dot)
	case ',':
		return l.make(Comma)
	case ':':
		return l.makeMatch(':', Colon, ColonColon)
	case ';':
		return l.make(Semicolon)
	case '#':
		return l.make(Hashtag)

	case '(':
		return l.make(LeftParen)
	case ')':
		return l.make(RightParen)

	case '{':
		return l.make(LeftBrace)
	case '}':
		return l.make(RightBrace)

	case '[':
		return l.make(LeftBracket)
	case ']':
		return l.make(RightBracket)

	// Operators

	case '+':
		if l.match('=') {
			return l.make(PlusEqual)
		}
		return l.makeMatch('+', Plus, PlusPlus)

	case '-':
		if l.match('=') {
			return l.make(MinusEqual)
		}
		return l.makeMatch('-', Minus, MinusMinus)

	case '*':
		return l.makeMatch('=', Star, StarEqual)

	case '/':
		return l.makeMatch('=', Slash, SlashEqual)

	case '%':
		return l.makeMatch('=', Percentage, PercentageEqual)

	case '|':
		if l.match('=') {
			return l.make(PipeEqual)
		}
		return l.makeMatch('|', Pipe, PipePipe)

	case '^':
		return l.makeMatch('=', Caret, CaretEqual)

	case '&':
		if l.match('=') {
			return l.make(AmpersandEqual)
		}
		return l.makeMatch('&', Ampersand, AmpersandAmpersand)

	case '=':
		return l.makeMatch('=', Equal, EqualEqual)

	case '!':
		return l.makeMatch('=', Bang, BangEqual)

	case '<':
		if l.match('=') {
			return l.make(LessEqual)
		}
		if l.match('<') {
			return l.makeMatch('=', LessLess, LessLessEqual)
		}
		return l.make(Less)

	case '>':
		if l.match('=') {
			return l.make(GreaterEqual)
		}
		if l.match('>') {
			if l.match('>') {
				return l.makeMatch('=', GreaterGreaterGreater, GreaterGreaterGreaterEqual)
			}
			return l.makeMatch('=', GreaterGreater, GreaterGreaterEqual)
		}
		return l.make(Greater)
	}

	return l.make(Error)
}

func (l *Lexer) binaryInteger() Token {
	digits := false

	for {
		ch := l.peek(0)

		if ch == '0' || ch == '1' {
			l.advance()
			digits = true
		} else {
			break
		}
	}

	if !digits {
		return l.makeError("binary number has no digits")
	}

	if isIdentifier(l.peek(0)) {
		return l.makeError("invalid binary digit")
	}

	return l.make(BinaryInteger)
}

func (l *Lexer) hexInteger() Token {
	digits := false

	for {
		ch := l.peek(0)

		if isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			l.advance()
			digits = true
		} else {
			break
		}
	}

	if !digits {
		return l.makeError("hexadecimal number has no digits")
	}

	if isIdentifier(l.peek(0)) {
		return l.makeError("invalid hexadecimal digit")
	}

	return l.make(HexInteger)
}

func (l *Lexer) number() Token {
	for isDigit(l.peek(0)) {
		l.advance()
	}

	// Decimal
	if l.peek(0) == '.' && isDigit(l.peek(1)) {
		l.advance()

		for isDigit(l.peek(0)) {
			l.advance()
		}

		kind := Decimal

		if l.match('f') || l.match('F') {
			kind = Decimal32bit
		} else {
			if isIdentifier(l.peek(0)) {
				return l.makeError("invalid number suffix")
			}
		}

		return l.make(kind)
	}

	// Integer
	kind := SignedInteger

	if l.match('u') || l.match('U') {
		kind = UnsignedInteger
	} else {
		if isIdentifier(l.peek(0)) {
			return l.makeError("invalid number suffix")
		}
	}

	return l.make(kind)
}

func (l *Lexer) character() Token {
	if l.peek(0) == '\'' {
		return l.makeError("empty character")
	}

	if l.match('\\') {
		ch := l.advance()

		if ch == 'x' || ch == 'X' {
			l.advance()
			l.advance()
		}
	} else {
		l.advance()
	}

	if !l.match('\'') {
		return l.makeError("unterminated character")
	}

	return l.make(Character)
}

func (l *Lexer) string() Token {
	for {
		ch := l.peek(0)

		switch ch {
		case '"':
			l.advance()
			return l.make(String)

		case '\000':
			return l.makeError("unterminated string")

		case '\\':
			l.advance()
			l.advance()

		default:
			l.advance()
		}
	}
}

func (l *Lexer) keywordIdentifier() Token {
	for isIdentifier(l.peek(0)) {
		l.advance()
	}

	token := l.make(Identifier)

	switch token.Text {
	case "true":
		token.Kind = True
	case "false":
		token.Kind = False

	case "null":
		token.Kind = Null

	case "pub":
		token.Kind = Pub
	case "mut":
		token.Kind = Mut

	case "mod":
		token.Kind = Mod
	case "import":
		token.Kind = Import

	case "struct":
		token.Kind = Struct
	case "interface":
		token.Kind = Interface
	case "impl":
		token.Kind = Impl
	case "func":
		token.Kind = Func

	case "var":
		token.Kind = Var
	case "if":
		token.Kind = If
	case "else":
		token.Kind = Else
	case "while":
		token.Kind = While
	case "for":
		token.Kind = For

	case "return":
		token.Kind = Return
	case "break":
		token.Kind = Break
	case "continue":
		token.Kind = Continue

	case "as":
		token.Kind = As

	case "sizeof":
		token.Kind = Sizeof
	case "alignof":
		token.Kind = Alignof
	case "offsetof":
		token.Kind = Offsetof
	}

	return token
}

func (l *Lexer) skipWhitespace() string {
	for {
		l.tokenText = l.tokenText[:0]

		switch l.peek(0) {
		case ' ', '\t', '\r', '\n':
			l.advance()

		case '/':
			switch l.peek(1) {
			case '/':
				l.advance()
				l.advance()

				for {
					l.tokenText = l.tokenText[:0]

					ch := l.peek(0)
					if ch == '\n' || ch == '\000' {
						break
					}

					l.advance()
				}

			case '*':
				l.advance()
				l.advance()

				depth := 1

				for {
					l.tokenText = l.tokenText[:0]
					ch := l.peek(0)

					if ch == '/' && l.peek(1) == '*' {
						depth++

						l.advance()
						l.advance()

						continue
					} else if ch == '*' && l.peek(1) == '/' {
						depth--

						l.advance()
						l.advance()

						if depth == 0 {
							break
						}

						continue
					} else if ch == '\000' {
						return "unterminated comment"
					}

					l.advance()
				}

			default:
				return ""
			}

		default:
			return ""
		}
	}
}

// Utils

func (l *Lexer) makeMatch(expected rune, kindFalse, kindTrue TokenKind) Token {
	if l.match(expected) {
		return l.make(kindTrue)
	}

	return l.make(kindFalse)
}

func (l *Lexer) make(kind TokenKind) Token {
	return Token{
		Kind:  kind,
		Text:  string(l.tokenText),
		Range: core.Range{Start: l.start, End: l.pos},
	}
}

func (l *Lexer) makeError(msg string) Token {
	return Token{
		Kind:  Error,
		Text:  msg,
		Range: core.Range{Start: l.start, End: l.pos},
	}
}

func (l *Lexer) match(expected rune) bool {
	if l.peek(0) == expected {
		l.advance()
		return true
	}

	return false
}

func (l *Lexer) peek(offset int) rune {
	for l.buffer.Size() <= offset {
		ch, _, err := l.reader.ReadRune()
		if err != nil {
			ch = '\000'
		}

		l.buffer.Add(ch)
	}

	return l.buffer.Peek(offset)
}

func (l *Lexer) advance() rune {
	var ch rune

	if l.buffer.Size() > 0 {
		ch, _ = l.buffer.TryGet()
		if ch == '\000' {
			return '\000'
		}
	} else {
		var err error
		ch, _, err = l.reader.ReadRune()

		if err != nil {
			return '\000'
		}
	}

	if ch == '\n' {
		l.pos.Line++
		l.pos.Column = 1
	} else {
		l.pos.Column++
	}

	l.tokenText = append(l.tokenText, ch)

	return ch
}

// Character groups

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isAlpha(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentifier(ch rune) bool {
	return isAlpha(ch) || isDigit(ch)
}
