package types

type Pointer struct {
	Mutable bool
	Pointee Type
}

func (p *Pointer) Equals(other Type) bool {
	if other, ok := other.(*Pointer); ok {
		return p.Mutable == other.Mutable && p.Pointee.Equals(other.Pointee)
	}

	return false
}

func (p *Pointer) String() string {
	if p.Mutable {
		return "mut *" + p.Pointee.String()
	}

	return "*" + p.Pointee.String()
}
