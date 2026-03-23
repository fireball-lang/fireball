package types

type Type interface {
	Equals(other Type) bool
}

type Composed interface {
	Type

	Underlying() Type
}
