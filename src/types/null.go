package types

type Null struct{}

func (n *Null) Equals(other Type) bool {
	_, ok := other.(*Null)
	return ok
}

func (n *Null) String() string {
	return "null"
}

var nullUnderlying = &Pointer{Pointee: PrimitiveVoid}

func (n *Null) Underlying() Type {
	return nullUnderlying
}
