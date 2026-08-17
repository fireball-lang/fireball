package types

type Reference struct {
	Mutable bool
	Pointee Type
}

func (r *Reference) Equals(other Type) bool {
	if other, ok := other.(*Reference); ok {
		return r.Mutable == other.Mutable && r.Pointee.Equals(other.Pointee)
	}

	return false
}

func (r *Reference) String() string {
	if r.Mutable {
		return "mut &" + r.Pointee.String()
	}

	return "&" + r.Pointee.String()
}
