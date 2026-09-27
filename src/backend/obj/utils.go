package obj

func AlignUp(val, align uint64) uint64 {
	if align <= 1 {
		return val
	}

	rem := val % align
	if rem == 0 {
		return val
	}

	return val + (align - rem)
}
