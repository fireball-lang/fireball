package ast

type ExprResultFlags uint8

var None ExprResultFlags = 0

const (
	Addressable ExprResultFlags = 1 << iota
	Assignable
	Invalid
)

func (e ExprResultFlags) IsAddressable() bool {
	return e&Addressable != 0
}

func (e ExprResultFlags) IsAssignable() bool {
	return e&Assignable != 0
}

func (e ExprResultFlags) IsInvalid() bool {
	return e&Invalid != 0
}

type ExprResult struct {
	Flags ExprResultFlags
	Type  Type
}

func (e *ExprResult) SetInvalid() {
	e.Flags = Invalid
	e.Type = VoidType
}

func (e *ExprResult) Set(flags ExprResultFlags, type_ Type) {
	e.Flags = flags
	e.Type = type_
}
