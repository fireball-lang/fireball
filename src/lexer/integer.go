package lexer

import (
	"fireball/core"
	"strconv"
)

func ParseInteger(token Token) core.Integer {
	switch token.Kind {
	case BinaryInteger:
		val, err := strconv.ParseUint(token.Text[2:], 2, 64)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse binary integer")
		}
		return core.Unsigned(false, val)

	case HexInteger:
		val, err := strconv.ParseUint(token.Text[2:], 16, 64)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse hexadecimal integer")
		}
		return core.Unsigned(false, val)

	case UnsignedInteger:
		val, err := strconv.ParseUint(token.Text[:len(token.Text)-1], 10, 64)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse unsigned integer")
		}
		return core.Unsigned(false, val)

	case SignedInteger:
		val, err := strconv.ParseInt(token.Text, 10, 64)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse signed integer")
		}
		return core.Signed(val)

	default:
		panic("lexer.ParseInteger() - Invalid token kind")
	}
}
