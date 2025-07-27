package codegen

import "fireball/ir"

type Value struct {
	Ir          ir.Value
	Typ         ir.Type
	Addressable bool
}

func toValue(ir ir.Value) Value {
	return Value{
		Ir:          ir,
		Typ:         ir.Type(),
		Addressable: false,
	}
}

func (v Value) SetName(name string) {
	v.Ir.(ir.Instruction).SetName(name)
}

func (v Value) IsPointer() bool {
	if v.Addressable {
		return true
	}

	if typ, ok := v.Typ.(*ir.SimpleType); ok && typ.Kind == ir.PointerKind {
		return true
	}

	return false
}

func (v Value) Read(c *codegen) Value {
	if !v.Addressable {
		return v
	}

	return Value{
		Ir:          c.emitter.Ir.Load(v.Typ, v.Ir),
		Typ:         v.Typ,
		Addressable: false,
	}
}

func (v Value) Write(c *codegen, value Value) {
	if !v.Addressable {
		panic("codegen.Value.Write() - Value is not addressable")
	}

	c.emitter.Ir.Store(value.Ir, v.Ir)
}

func (v Value) IntoPointer() Value {
	if !v.Addressable {
		panic("codegen.Value.Write() - Value is not addressable")
	}

	return Value{
		Ir:          v.Ir,
		Typ:         ir.Pointer,
		Addressable: false,
	}
}

func (v Value) IntoAddressable(typ ir.Type) Value {
	if v.Addressable {
		panic("codegen.Value.Write() - Value is addressable")
	}

	return Value{
		Ir:          v.Ir,
		Typ:         typ,
		Addressable: true,
	}
}
