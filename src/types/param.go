package types

type Param struct {
	Name string
}

func (p *Param) Equals(other Type) bool {
	return p == other
}

func (p *Param) String() string {
	return p.Name
}
