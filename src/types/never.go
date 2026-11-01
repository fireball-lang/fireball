package types

type Never struct{}

var NeverType = &Never{}

func (n *Never) Equals(other Type) bool {
	_, ok := other.(*Never)
	return ok
}

func (n *Never) String() string {
	return "never"
}
