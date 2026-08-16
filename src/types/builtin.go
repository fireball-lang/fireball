package types

type Builtins struct {
	StringView *Struct

	Zeroable *Interface

	// Reflection

	Case  *Struct
	Field *Struct

	Implementation *Struct

	TypeInfo *Struct
}
