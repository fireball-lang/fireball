package types

import (
	"fmt"
	"strings"
)

type Tuple struct {
	Types []Type

	underlying *Struct
}

func (t *Tuple) Equals(other Type) bool {
	if other, ok := other.(*Tuple); ok {
		return typeSliceEquals(t.Types, other.Types)
	}

	return false
}

func (t *Tuple) String() string {
	var sb strings.Builder
	sb.WriteRune('(')

	for i, typ := range t.Types {
		if i > 0 {
			sb.WriteString(", ")
		}

		sb.WriteString(typ.String())
	}

	sb.WriteRune(')')
	return sb.String()
}

func (t *Tuple) Underlying() Type {
	if t.underlying != nil {
		return t.underlying
	}

	fields := make([]Field, len(t.Types))

	for i, typ := range t.Types {
		fields[i] = Field{
			Name:     fmt.Sprintf("_%d", i),
			Type:     typ,
			Public:   true,
			Required: true,
		}
	}

	t.underlying = &Struct{
		Layout: Fireball,
		Fields: fields,
	}

	t.underlying.Optimize()

	return t.underlying
}
