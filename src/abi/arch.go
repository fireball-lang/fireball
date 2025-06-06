package abi

type Arch struct {
	WordSize uint32
}

var AMD64 = Arch{
	WordSize: 8,
}
