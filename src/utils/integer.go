package utils

import (
	"cmp"
	"math"
	"strconv"
)

type Integer struct {
	negative bool
	value    uint64
}

func Signed(v int64) Integer {
	abs := v
	if abs < 0 {
		abs = -abs
	}

	return Integer{
		negative: v < 0,
		value:    uint64(abs),
	}
}

func Unsigned(negative bool, v uint64) Integer {
	return Integer{
		negative: negative,
		value:    v,
	}
}

func (i Integer) Negative() bool {
	return i.negative
}

func (i Integer) AddOne() Integer {
	if i.negative {
		if i.value == 1 {
			return Integer{
				negative: false,
				value:    0,
			}
		}

		return Integer{
			negative: true,
			value:    i.value - 1,
		}
	}

	return Integer{
		negative: false,
		value:    i.value + 1,
	}
}

func (i Integer) Min(other Integer) Integer {
	if i.negative && other.negative {
		if i.value > other.value {
			return i
		}
		return other
	}

	if i.negative {
		return i
	}
	if other.negative {
		return other
	}

	if i.value < other.value {
		return i
	}
	return other
}

func (i Integer) Max(other Integer) Integer {
	if i.negative && other.negative {
		if i.value < other.value {
			return i
		}
		return other
	}

	if i.negative {
		return other
	}
	if other.negative {
		return i
	}

	if i.value > other.value {
		return i
	}
	return other
}

func (i Integer) Compare(other Integer) int {
	if i == other {
		return 0
	}

	if i.negative && other.negative {
		return -cmp.Compare(i.value, other.value)
	}

	if i.negative {
		return -1
	}
	if other.negative {
		return 1
	}

	return cmp.Compare(i.value, other.value)
}

func (i Integer) GreaterThan(other Integer) bool {
	return i.Compare(other) == 1
}

func (i Integer) GreaterThanEqual(other Integer) bool {
	c := i.Compare(other)
	return c == 1 || c == 0
}

func (i Integer) LessThan(other Integer) bool {
	return i.Compare(other) == -1
}

func (i Integer) LessThanEqual(other Integer) bool {
	c := i.Compare(other)
	return c == -1 || c == 0
}

func (i Integer) Signed() int64 {
	if i.value > math.MaxInt64 {
		panic("utils.Integer.Signed() - Value is out of int64 range")
	}

	if i.negative {
		return -int64(i.value)
	}
	return int64(i.value)
}

func (i Integer) Unsigned() uint64 {
	if i.negative {
		panic("utils.Integer.Unsigned() - Value is negative")
	}

	return i.value
}

func (i Integer) String() string {
	if i.negative {
		return "-" + strconv.FormatUint(i.value, 10)
	}

	return strconv.FormatUint(i.value, 10)
}
