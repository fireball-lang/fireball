package types

type Pointer struct {
	Pointee Type
}

func (p *Pointer) Equals(other Type) bool {
	if other, ok := other.(*Pointer); ok {
		return p.Pointee.Equals(other.Pointee)
	}

	return false
}

func (p *Pointer) String() string {
	return "*" + p.Pointee.String()
}
