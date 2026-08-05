package types

import "fireball/core"

type Enum struct {
	Name       string
	ModulePath []string

	CaseType Type
	Cases    []Case
}

type Case struct {
	Name  string
	Value core.Integer
}

func (e *Enum) Equals(other Type) bool {
	return e == other
}

func (e *Enum) String() string {
	return e.Name
}

func (e *Enum) Underlying() Type {
	return e.CaseType
}

func (e *Enum) Case(name string) (core.Integer, bool) {
	for _, c := range e.Cases {
		if c.Name == name {
			return c.Value, true
		}
	}

	return core.Integer{}, false
}
