package types

import (
	"math/bits"
	"strconv"
)

type Integer struct {
	Negative bool
	Unsigned bool
	Value    uint64
}

func (i *Integer) Equals(other Type) bool {
	if o, ok := other.(*Integer); ok {
		return i.Negative == o.Negative && i.Unsigned == o.Unsigned && i.Value == o.Value
	}

	return false
}

func (i *Integer) String() string {
	if i.Negative {
		return "-" + strconv.FormatUint(i.Value, 10)
	}

	return strconv.FormatUint(i.Value, 10)
}

func (i *Integer) Underlying() Type {
	return i.ToPrimitive()
}

func (i *Integer) RawBits() uint32 {
	return uint32(bits.Len64(i.Value))
}

func (i *Integer) Bits() uint32 {
	if !i.Unsigned {
		return 1 + i.RawBits()
	}

	return i.RawBits()
}

func (i *Integer) ToPrimitive() *Primitive {
	bits := i.Bits()

	// Unsigned
	if i.Unsigned {
		if bits <= 8 {
			return PrimitiveU8
		}
		if bits <= 16 {
			return PrimitiveU16
		}
		if bits <= 32 {
			return PrimitiveU32
		}
		return PrimitiveU64
	}

	// Signed
	if bits <= 8 {
		return PrimitiveI8
	}
	if bits <= 16 {
		return PrimitiveI16
	}
	if bits <= 32 {
		return PrimitiveI32
	}
	return PrimitiveI64
}
