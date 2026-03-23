package types

type Array struct {
	Size    uint64
	Element Type
}

func (a *Array) Equals(other Type) bool {
	if other, ok := other.(*Array); ok {
		return a.Size == other.Size && a.Element.Equals(other.Element)
	}

	return false
}
