package utils

func Ternary[T any](condition bool, ifTrue T, ifFalse T) T {
	if condition {
		return ifTrue
	}

	return ifFalse
}
