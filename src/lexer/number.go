package lexer

import (
	"fireball/core"
	"strconv"
)

func ParseInteger(token Token) core.Integer {
	switch token.Kind {
	case BinaryInteger:
		val, err := parseInteger(token.Text[2:], 2)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse binary integer")
		}
		return val

	case HexInteger:
		val, err := parseInteger(token.Text[2:], 16)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse hexadecimal integer")
		}
		return val

	case UnsignedInteger:
		val, err := parseInteger(token.Text[:len(token.Text)-1], 10)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse unsigned integer")
		}
		return val

	case SignedInteger:
		val, err := parseInteger(token.Text, 10)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse signed integer")
		}
		return val

	default:
		panic("lexer.ParseInteger() - Invalid token kind")
	}
}

func ParseDecimal(token Token) (float64, error) {
	var buffer [64]uint8
	i := 0

	for _, ch := range token.Text {
		if ch != '_' {
			buffer[i] = uint8(ch)
			i++
		}
	}

	switch token.Kind {
	case Decimal32bit:
		return strconv.ParseFloat(string(buffer[0:i-1]), 32)
	case Decimal:
		return strconv.ParseFloat(string(buffer[0:i]), 64)

	default:
		panic("lexer.ParseDecimal() - Invalid token kind")
	}
}

func parseInteger(str string, base int) (core.Integer, error) {
	if str == "" {
		return core.Integer{}, strconv.ErrSyntax
	}

	// Negative
	negative := false

	if str[0] == '-' {
		negative = true
		str = str[1:]
	}

	// Value
	var buffer [64]uint8
	i := 0

	for _, ch := range str {
		if ch != '_' {
			buffer[i] = uint8(ch)
			i++
		}
	}

	value, err := strconv.ParseUint(string(buffer[0:i]), base, 64)
	return core.Unsigned(negative, value), err
}
