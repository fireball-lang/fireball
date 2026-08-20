package types

import "strconv"

type Integer struct {
	Negative bool
	Unsigned bool
	RawBits  uint32
}

func (i *Integer) Equals(other Type) bool {
	if o, ok := other.(*Integer); ok {
		return i.Negative == o.Negative && i.Unsigned == o.Unsigned && i.RawBits == o.RawBits
	}

	return false
}

func (i *Integer) String() string {
	prefix := "i"
	if i.Unsigned {
		prefix = "u"
	}

	return prefix + strconv.FormatUint(uint64(i.Bits()), 10)
}

func (i *Integer) Underlying() Type {
	return i.ToPrimitive()
}

func (i *Integer) Bits() uint32 {
	if !i.Unsigned {
		return 1 + i.RawBits
	}

	return i.RawBits
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
