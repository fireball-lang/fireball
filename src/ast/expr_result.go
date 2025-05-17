package ast

type ExprResultKind uint8

const (
	Invalid ExprResultKind = iota
	Value
	Address
)

type ExprResult struct {
	Kind ExprResultKind
	Type Type
}

func (e *ExprResult) SetInvalid() {
	e.Kind = Invalid
	e.Type = VoidType
}

func (e *ExprResult) Set(kind ExprResultKind, type_ Type) {
	e.Kind = kind
	e.Type = type_
}
