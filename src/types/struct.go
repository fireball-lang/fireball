package types

type Field struct {
	Name string
	Type Type
}

type Struct struct {
	Name   string
	Packed bool
	Fields []Field
}

func (s *Struct) Field(name string) (Field, int) {
	for i, field := range s.Fields {
		if field.Name == name {
			return field, i
		}
	}

	return Field{}, -1
}

func (s *Struct) Equals(other Type) bool {
	return s == other
}

func (s *Struct) String() string {
	return s.Name
}
