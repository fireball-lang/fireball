package types

import "fmt"

type Array struct {
	Size    uint32
	Element Type
}

func (a *Array) Equals(other Type) bool {
	if other, ok := other.(*Array); ok {
		return a.Size == other.Size && a.Element.Equals(other.Element)
	}

	return false
}

func (a *Array) String() string {
	return fmt.Sprintf("[%d]%s", a.Size, a.Element)
}
