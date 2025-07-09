package analyzer

import (
	"fireball/lexer"
	"fireball/utils"
	"strconv"
	"strings"
)

type StringBuilder interface {
	WriteRune(r rune)
	WriteEscapeSequence(esc uint8)

	Error(r lexer.Range, msg string)
}

type stringParser[T StringBuilder] struct {
	builder T

	runes []rune
	i     int

	startPos lexer.Pos
	pos      lexer.Pos
}

func ParseString[T StringBuilder](str string, builder T) {
	s := stringParser[T]{
		builder: builder,
		runes:   []rune(str),
	}

	s.parse()
}

func (s *stringParser[T]) parse() {
	for !s.isAtEnd() {
		if s.peek() == '\\' {
			s.parseEscapeSequence()
			continue
		}

		ch := s.advance()
		switch ch {
		case '\a':
			s.builder.WriteEscapeSequence('\a')
		case '\b':
			s.builder.WriteEscapeSequence('\n')
		case '\f':
			s.builder.WriteEscapeSequence('\f')
		case '\n':
			s.builder.WriteEscapeSequence('\n')
		case '\r':
			s.builder.WriteEscapeSequence('\r')
		case '\t':
			s.builder.WriteEscapeSequence('\t')
		case '\v':
			s.builder.WriteEscapeSequence('\v')
		default:
			s.builder.WriteRune(ch)
		}
	}
}

func (s *stringParser[T]) parseEscapeSequence() {
	s.startPos = s.pos
	s.advance()

	if isOctal(s.peek()) {
		s.parseOctalEscapeSequence()
		return
	}

	if s.peek() == 'x' {
		s.advance()
		s.parseHexEscapeSequence()
		return
	}

	switch s.advance() {
	case 'a':
		s.builder.WriteEscapeSequence('\a')
	case 'b':
		s.builder.WriteEscapeSequence('\n')
	case 'f':
		s.builder.WriteEscapeSequence('\f')
	case 'n':
		s.builder.WriteEscapeSequence('\n')
	case 'r':
		s.builder.WriteEscapeSequence('\r')
	case 't':
		s.builder.WriteEscapeSequence('\t')
	case 'v':
		s.builder.WriteEscapeSequence('\v')
	case '\\':
		s.builder.WriteRune('\\')
	case '"':
		s.builder.WriteEscapeSequence('"')

	default:
		s.error("Invalid escape sequence")
	}
}

func (s *stringParser[T]) parseOctalEscapeSequence() {
	var sb strings.Builder

	for i := 0; i < 3; i++ {
		if !isOctal(s.peek()) {
			s.error("Octal escape sequence needs 3 numbers.")
			return
		}

		sb.WriteRune(s.advance())
	}

	num, err := strconv.ParseUint(sb.String(), 8, 8)
	if err != nil {
		s.error("Invalid octal escape sequence.")
		return
	}

	s.builder.WriteEscapeSequence(uint8(num))
}

func (s *stringParser[T]) parseHexEscapeSequence() {
	var sb strings.Builder

	for i := 0; i < 2; i++ {
		if !isHex(s.peek()) {
			s.error("Hex escape sequence needs 3 numbers.")
			return
		}

		sb.WriteRune(s.advance())
	}

	num, err := strconv.ParseUint(sb.String(), 16, 8)
	if err != nil {
		s.error("Invalid hex escape sequence.")
		return
	}

	s.builder.WriteEscapeSequence(uint8(num))
}

func (s *stringParser[T]) isAtEnd() bool {
	return s.i >= len(s.runes)
}

func (s *stringParser[T]) peek() rune {
	return s.peekN(0)
}

func (s *stringParser[T]) peekN(offset int) rune {
	if s.i+offset < len(s.runes) {
		return s.runes[s.i+offset]
	}

	return '\000'
}

func (s *stringParser[T]) error(msg string) {
	s.builder.Error(lexer.Range{
		Start: s.startPos,
		End:   s.pos,
	}, msg)
}

func (s *stringParser[T]) advance() rune {
	r := s.runes[s.i]

	s.i++

	if r == '\n' {
		s.pos.Line++
		s.pos.Column = 0
	} else {
		s.pos.Column++
	}

	return r
}

func isOctal(r rune) bool {
	return r >= '0' && r <= '7'
}

func isHex(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// Analyzer impl

type stringBuilder struct {
	startPos lexer.Pos
	errors   []utils.Diagnostic
}

func (s *stringBuilder) WriteRune(r rune) {}

func (s *stringBuilder) WriteEscapeSequence(esc uint8) {}

func (s *stringBuilder) Error(r lexer.Range, msg string) {
	if r.Start.Line == 0 {
		r.Start.Column += s.startPos.Column
	}
	if r.End.Line == 0 {
		r.End.Column += s.startPos.Column
	}

	r.Start.Line += s.startPos.Line
	r.End.Line += s.startPos.Line

	s.errors = append(s.errors, utils.Diagnostic{
		Kind:    utils.Error,
		Message: msg,
		Range:   r,
	})
}
