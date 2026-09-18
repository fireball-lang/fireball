package lsp

import (
	"fireball/core"
	"fireball/ir/eval"
	"fireball/types"
	"strconv"
	"strings"
)

const (
	evalValueBudget     = 96
	evalValueMaxDepth   = 4
	evalValueStringSize = 24
)

func formatEvalValue(v eval.Value, typ types.Type) (string, bool) {
	p := &valuePrinter{left: evalValueBudget}
	p.value(v, typ)

	if p.sb.Len() == 0 {
		return "", false
	}

	return p.sb.String(), true
}

type valuePrinter struct {
	sb    strings.Builder
	left  int
	depth int
	cut   bool
}

func (p *valuePrinter) atom(s string) {
	if p.cut {
		return
	}

	runes := []rune(s)

	if len(runes) > p.left {
		p.cut = true
		return
	}

	p.sb.WriteString(s)
	p.left -= len(runes)
}

func (p *valuePrinter) atomString(s string) {
	if p.cut {
		return
	}

	runes := []rune(s)
	maxV := min(evalValueStringSize, p.left-2)

	if maxV < 0 {
		p.cut = true
		return
	}

	if len(runes) > maxV {
		p.sb.WriteString(strconv.Quote(string(runes[:maxV])))
		p.sb.WriteRune('…')

		p.left = 0
		p.cut = true
		return
	}

	p.atom(strconv.Quote(s))
}

func (p *valuePrinter) value(v eval.Value, typ types.Type) {
	if p.cut || p.left <= 0 {
		p.cut = true
		return
	}

	switch v := v.(type) {
	case *eval.IntValue:
		p.atom(formatIntValue(v.Value, typ))

	case *eval.FloatValue:
		p.atom(formatFloatValue(v.Value, typ))

	case *eval.StringValue:
		if v.Value != nil {
			p.atomString(string(v.Value.Runes))
		} else {
			p.cut = true
		}

	case *eval.NullValue:
		p.atom("null")

	case *eval.FuncValue:
		if v.Name != "" {
			p.atom(v.Name)
		} else {
			p.cut = true
		}

	case *eval.GlobalValue:
		if v.Name != "" {
			p.atom("&" + v.Name)
		} else {
			p.cut = true
		}

	case *eval.AggregateValue:
		if s, ok := stringValueView(v); ok {
			p.atomString(s)
			return
		}

		p.aggregate(v, typ)

	default:
		p.cut = true
	}
}

func (p *valuePrinter) aggregate(v *eval.AggregateValue, typ types.Type) {
	if p.depth >= evalValueMaxDepth {
		p.cut = true
		return
	}

	openCh, closeCh := "{ ", " }"
	elem := func(int) (string, types.Type) { return "", nil }

	if a, ok := typ.(*types.Array); ok {
		openCh, closeCh = "[ ", " ]"
		elem = func(int) (string, types.Type) { return "", a.Element }
	} else if s, ok := typ.(*types.Struct); ok {
		fields := s.Fields
		if s.Generic != nil {
			fields = s.Generic.Fields
		}

		elem = func(i int) (string, types.Type) {
			if i < len(fields) {
				return fields[i].Name, fields[i].Type
			}
			return "", nil
		}
	}

	p.depth++

	p.sb.WriteString(openCh)
	p.left -= len(openCh)

	for i, val := range v.Values {
		if p.cut || p.left <= 1 {
			p.cut = true
			break
		}

		if i > 0 {
			p.sb.WriteString(", ")
			p.left -= 2

			if p.cut || p.left <= 1 {
				p.cut = true
				break
			}
		}

		name, typ := elem(i)

		if name != "" {
			p.sb.WriteString(name)
			p.sb.WriteString(": ")

			p.left -= len(name) + 2
		}

		p.value(val, typ)
	}

	if p.cut {
		p.sb.WriteString("…")
	}

	p.sb.WriteString(closeCh)
	p.left -= len(closeCh)

	p.depth--
}

func stringValueView(v *eval.AggregateValue) (string, bool) {
	if len(v.Values) != 2 {
		return "", false
	}

	a, b := v.Values[0], v.Values[1]

	str, ok := a.(*eval.StringValue)
	if !ok {
		str, ok = b.(*eval.StringValue)
		if !ok {
			return "", false
		}

		a, b = b, a
	}

	if str.Value == nil {
		return "", false
	}

	length, ok := b.(*eval.IntValue)
	if !ok || length.Value != uint64(str.Value.Size) {
		return "", false
	}

	return string(str.Value.Runes), true
}

func formatIntValue(v uint64, typ types.Type) string {
	if typ != nil {
		switch typ := typ.(type) {
		case *types.Primitive:
			if typ.Kind == types.Bool {
				if v != 0 {
					return "true"
				}
				return "false"
			}

			if types.IsInteger(typ.Kind) {
				return formatIntegerBits(v, typ.Kind)
			}

		case *types.Integer:
			return formatIntegerBits(v, typ.ToPrimitive().Kind)

		case *types.Enum:
			if name, ok := enumCaseName(typ, v); ok {
				return typ.Name + "." + name
			}

			if prim, ok := typ.CaseType.(*types.Primitive); ok && types.IsInteger(prim.Kind) {
				return formatIntegerBits(v, prim.Kind)
			}
		}
	}

	return core.TwosComplement(v).String()
}

func formatIntegerBits(v uint64, kind types.PrimitiveKind) string {
	return integerFromBits(v, kind).String()
}

func integerFromBits(v uint64, kind types.PrimitiveKind) core.Integer {
	if types.IsUnsignedInteger(kind) {
		return core.Unsigned(false, v)
	}

	return core.TwosComplementWidth(v, kind.Size()*8)
}

func enumCaseName(e *types.Enum, v uint64) (string, bool) {
	candidates := make([]core.Integer, 0, 2)

	if prim, ok := e.CaseType.(*types.Primitive); ok && types.IsInteger(prim.Kind) {
		candidates = append(candidates, integerFromBits(v, prim.Kind))
	} else {
		candidates = append(candidates, core.Unsigned(false, v), core.TwosComplement(v))
	}

	for _, candidate := range candidates {
		for _, cas := range e.Cases {
			if cas.Value == candidate {
				return cas.Name, true
			}
		}
	}

	return "", false
}

func formatFloatValue(v float64, typ types.Type) string {
	bits := 64

	if prim, ok := typ.(*types.Primitive); ok && types.IsFloating(prim.Kind) {
		bits = int(prim.Kind.Size() * 8)
	}

	return strconv.FormatFloat(v, 'g', -1, bits)
}
