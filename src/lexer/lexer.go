package lexer

import "strings"

type Lexer struct {
	runes []rune
	i     int

	line   uint32
	column uint32

	start Pos
	sb    strings.Builder
}

func NewLexer(source string) *Lexer {
	return &Lexer{
		runes: []rune(source),
		line:  1,
	}
}

func (l *Lexer) Next() Token {
	l.skipWhitespace()

	l.start = Pos{Line: l.line, Column: l.column}
	l.sb.Reset()

	if l.i >= len(l.runes) {
		return l.tokenPrimitive(Eof, "")
	}

	ch := l.advance()

	if isAlpha(ch) {
		return l.identifier()
	}
	if isNumber(ch) || (ch == '-' && isNumber(l.peek())) {
		return l.number(ch)
	}

	switch ch {
	case '\'':
		return l.character()
	case '"':
		return l.string()

	case '=':
		return l.tokenCond('=', Equal, EqualEqual)
	case '!':
		return l.tokenCond('=', Bang, BangEqual)

	case '|':
		if l.peek() == '=' {
			l.advance()
			return l.token(PipeEqual)
		}
		return l.tokenCond('|', Pipe, PipePipe)
	case '^':
		return l.tokenCond('=', Xor, XorEqual)
	case '&':
		if l.peek() == '=' {
			l.advance()
			return l.token(AmpersandEqual)
		}
		return l.tokenCond('&', Ampersand, AmpersandAmpersand)

	case '<':
		return l.tokenCond('<', Less, LessEqual)
	case '>':
		return l.tokenCond('>', Greater, GreaterEqual)

	case '+':
		if l.peek() == '=' {
			l.advance()
			return l.token(PlusEqual)
		}
		return l.tokenCond('+', Plus, PlusPlus)
	case '-':
		if l.peek() == '=' {
			l.advance()
			return l.token(MinusEqual)
		}
		return l.tokenCond('-', Minus, MinusMinus)
	case '*':
		return l.tokenCond('=', Star, StarEqual)
	case '/':
		return l.tokenCond('=', Slash, SlashEqual)
	case '%':
		return l.tokenCond('=', Percentage, PercentageEqual)

	case '(':
		return l.token(LeftParen)
	case ')':
		return l.token(RightParen)
	case '{':
		return l.token(LeftBrace)
	case '}':
		return l.token(RightBrace)
	case '[':
		return l.token(LeftBracket)
	case ']':
		return l.token(RightBracket)

	case '.':
		if l.peekN(0) == '.' && l.peekN(1) == '.' {
			l.advance()
			l.advance()

			return l.token(DotDotDot)
		}

		return l.token(Dot)
	case ',':
		return l.token(Comma)
	case ':':
		return l.token(Colon)
	case ';':
		return l.token(Semicolon)
	case '#':
		return l.token(Hashtag)

	default:
		return l.tokenPrimitive(Error, "Invalid character '"+string(ch)+"'")
	}
}

func (l *Lexer) identifier() Token {
	for isAlpha(l.peek()) || isNumber(l.peek()) {
		l.advance()
	}

	return l.token(Identifier)
}

func (l *Lexer) number(ch rune) Token {
	if ch == '0' && (l.peek() == 'x' || l.peek() == 'X') {
		l.advance()
		return l.hexadecimal()
	}

	if ch == '0' && (l.peek() == 'b' || l.peek() == 'B') {
		l.advance()
		return l.binary()
	}

	return l.integerOrFloating()
}

func (l *Lexer) hexadecimal() Token {
	for isHex(l.peek()) {
		l.advance()
	}

	return l.token(Hexadecimal)
}

func (l *Lexer) binary() Token {
	for isBinary(l.peek()) {
		l.advance()
	}

	return l.token(Binary)
}

func (l *Lexer) integerOrFloating() Token {
	for isNumber(l.peek()) {
		l.advance()
	}

	floating := false

	if l.peek() == '.' {
		l.advance()

		for isNumber(l.peek()) {
			l.advance()
		}

		floating = true
	}

	if l.peek() == 'f' || l.peek() == 'F' {
		l.advance()
		floating = true
	}

	if floating {
		return l.token(Floating)
	}

	if l.peek() == 'u' || l.peek() == 'U' {
		l.advance()
	}

	return l.token(Integer)
}

func (l *Lexer) character() Token {
	if l.isAtEnd() || l.peek() == '\'' {
		return l.tokenPrimitive(Error, "Empty character.")
	}

	if l.advance() == '\\' && !l.isAtEnd() {
		c := l.advance()

		if c != '\'' && c != '0' && c != 'n' && c != 'r' && c != 't' {
			return l.tokenPrimitive(Error, "Unexpected character.")
		}
	}

	if l.peek() != '\'' {
		return l.tokenPrimitive(Error, "Unterminated character.")
	}

	l.advance()

	return l.token(Character)
}

func (l *Lexer) string() Token {
	for {
		if l.i >= len(l.runes) {
			return l.tokenPrimitive(Error, "Unterminated string.")
		}

		ch := l.advance()

		if ch == '"' {
			break
		}
	}

	return l.token(String)
}

func (l *Lexer) tokenCond(next rune, false, true TokenKind) Token {
	if l.peek() == next {
		l.advance()
		return l.token(true)
	}

	return l.token(false)
}

func (l *Lexer) token(kind TokenKind) Token {
	return l.tokenPrimitive(kind, l.sb.String())
}

func (l *Lexer) tokenPrimitive(kind TokenKind, message string) Token {
	return Token{
		Kind: kind,
		Text: message,
		Range: Range{
			Start: l.start,
			End:   Pos{Line: l.line, Column: l.column},
		},
	}
}

func (l *Lexer) skipWhitespace() {
	for l.i < len(l.runes) {
		switch l.peek() {
		case ' ', '\t', '\r', '\n':
			l.advance_(false)

		case '/':
			if l.peekN(1) == '/' {
				l.advance_(false)
				l.advance_(false)

				for l.i < len(l.runes) {
					if l.advance_(false) == '\n' {
						break
					}
				}
			} else if l.peekN(1) == '*' {
				l.advance_(false)
				l.advance_(false)

				for l.i < len(l.runes) {
					if l.advance_(false) == '*' && l.peek() == '/' {
						l.advance_(false)
						break
					}
				}
			} else {
				return
			}

		default:
			return
		}
	}
}

func (l *Lexer) isAtEnd() bool {
	return l.i >= len(l.runes)
}

func (l *Lexer) peek() rune {
	return l.peekN(0)
}

func (l *Lexer) peekN(offset int) rune {
	if l.i+offset < len(l.runes) {
		return l.runes[l.i+offset]
	}

	return '\000'
}

func (l *Lexer) advance() rune {
	return l.advance_(true)
}

func (l *Lexer) advance_(append bool) rune {
	ch := l.runes[l.i]
	l.i++

	if ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}

	if append {
		l.sb.WriteRune(ch)
	}

	return ch
}

// Character checks

func isAlpha(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isNumber(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isHex(ch rune) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isBinary(ch rune) bool {
	return ch == '0' || ch == '1'
}
