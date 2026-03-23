package types

type Struct struct {
	Fields []Type
}

func (s *Struct) Equals(other Type) bool {
	return s == other
}
