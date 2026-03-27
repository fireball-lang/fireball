package lexer

import "strconv"

func ParseInteger(token Token) uint64 {
	switch token.Kind {
	case BinaryInteger:
		val, err := strconv.ParseUint(token.Text[2:], 2, 64)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse binary integer")
		}
		return val

	case HexInteger:
		val, err := strconv.ParseUint(token.Text[2:], 16, 64)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse hexadecimal integer")
		}
		return val

	case UnsignedInteger, SignedInteger:
		val, err := strconv.ParseUint(token.Text, 10, 64)
		if err != nil {
			panic("lexer.ParseInteger() - Failed to parse integer")
		}
		return val

	default:
		panic("lexer.ParseInteger() - Invalid token kind")
	}
}
