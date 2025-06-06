package abi

func alignTo(num, align uint32) uint32 {
	if num%align != 0 {
		num += align - (num % align)
	}

	return num
}
