package types

type invalid struct{}

func (i *invalid) Equals(_ Type) bool {
	return false
}

func (i *invalid) String() string {
	return "<invalid>"
}

var Invalid = &invalid{}
