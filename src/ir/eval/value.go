package eval

import (
	"fireball/ir"
	"math"
)

type Value interface {
	Equals(other Value) bool

	_isValue()
}

type IntValue struct {
	Value uint64
}

func (i *IntValue) Equals(other Value) bool {
	if other, ok := other.(*IntValue); ok {
		return i.Value == other.Value
	}

	return false
}

func (i *IntValue) _isValue() {}

type FloatValue struct {
	Value float64
}

func (f *FloatValue) Equals(other Value) bool {
	if other, ok := other.(*FloatValue); ok {
		return f.Value == other.Value
	}

	return false
}

func (f *FloatValue) _isValue() {}

type StringValue struct {
	Value *ir.String
}

func (s *StringValue) Equals(other Value) bool {
	if other, ok := other.(*StringValue); ok {
		return s.Value.Equals(other.Value)
	}

	return false
}

func (s *StringValue) _isValue() {}

type NullValue struct{}

func (n *NullValue) Equals(other Value) bool {
	_, ok := other.(*NullValue)
	return ok
}

func (n *NullValue) _isValue() {}

type AggregateValue struct {
	Values []Value
}

func (a *AggregateValue) Equals(other Value) bool {
	if other, ok := other.(*AggregateValue); ok {
		if len(a.Values) != len(other.Values) {
			return false
		}

		for i, value := range a.Values {
			if !value.Equals(other.Values[i]) {
				return false
			}
		}

		return true
	}

	return false
}

func (a *AggregateValue) _isValue() {}

type GlobalValue struct {
	Name  string
	Typ   ir.Type
	Flags ir.GlobalVarFlags
	Data  any
}

func (g *GlobalValue) Equals(other Value) bool {
	if other, ok := other.(*GlobalValue); ok {
		return g.Name == other.Name
	}

	return false
}

func (g *GlobalValue) _isValue() {}

type FuncValue struct {
	Name      string
	Signature *ir.Signature
	Data      any
}

func (f *FuncValue) Equals(other Value) bool {
	if other, ok := other.(*FuncValue); ok {
		return f.Name == other.Name
	}

	return false
}

func (f *FuncValue) _isValue() {}

func (e *Evaluator) GetValue(reg Register, typ ir.Type) Value {
	switch typ := typ.(type) {
	case *ir.SimpleType:
		switch typ.Kind {
		case ir.VoidKind:
			return nil

		case ir.FloatKind:
			return &FloatValue{Value: float64(math.Float32frombits(uint32(reg.Scalar)))}

		case ir.DoubleKind:
			return &FloatValue{Value: math.Float64frombits(reg.Scalar)}

		case ir.PointerKind:
			if reg.Scalar == 0 {
				return &NullValue{}
			}

			if reg.Scalar&FuncAddrFlag != 0 {
				if f, ok := e.funcByAddr[reg.Scalar]; ok {
					return &FuncValue{
						Name:      f.Name,
						Signature: f.Signature,
						Data:      f.Data,
					}
				}

				panic("ir.eval.Evaluator.GetValue() - Invalid function pointer")
			}

			if g, ok := e.globalByAddr[reg.Scalar]; ok {
				switch init := g.Initializer.(type) {
				case *ir.String:
					return &StringValue{Value: init}

				default:
					return &GlobalValue{
						Name:  g.Name,
						Typ:   g.Type(),
						Flags: g.Flags,
						Data:  g.Data,
					}
				}
			}

			panic("ir.eval.Evaluator.GetValue() - Invalid pointer")

		default:
			panic("ir.eval.Evaluator.GetValue() - Invalid SimpleType kind")
		}

	case *ir.IntegerType:
		return &IntValue{Value: reg.Scalar}

	case *ir.VectorType:
		values := make([]Value, len(reg.Aggregate))

		for i, register := range reg.Aggregate {
			values[i] = e.GetValue(register, typ.Element)
		}

		return &AggregateValue{Values: values}

	case *ir.ArrayType:
		values := make([]Value, len(reg.Aggregate))

		for i, register := range reg.Aggregate {
			values[i] = e.GetValue(register, typ.Element)
		}

		return &AggregateValue{Values: values}

	case *ir.StructType:
		values := make([]Value, len(reg.Aggregate))

		for i, register := range reg.Aggregate {
			values[i] = e.GetValue(register, typ.Fields[i].Type)
		}

		return &AggregateValue{Values: values}

	case *ir.RefStructType:
		return e.GetValue(reg, &typ.Struct)

	default:
		panic("ir.eval.Evaluator.GetValue() - Invalid type")
	}
}
