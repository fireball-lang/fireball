package llvm

import (
	"fmt"
	"math"
)

type Function struct {
	m *Module

	name  string
	type_ *functionType

	localIdentifierNumber uint32

	line               uint32
	column             uint32
	locationDebugIndex uint32

	skipInstructions bool
}

func (f *Function) Block(identifier Identifier) Identifier {
	f.m.body.WriteString(identifier.name)
	f.m.body.WriteString(":\n")

	f.skipInstructions = false

	return identifier
}

func (f *Function) SetSourceLocation(line, column uint32) {
	f.line = line
	f.column = column
	f.locationDebugIndex = math.MaxUint32
}

func (f *Function) PushScope() {
	_, _ = fmt.Fprintf(
		&f.m.footer,
		"!%d = distinct !DILexicalBlock(scope: !%d, file: !%d, line: %d, column: %d)\n",
		f.m.debugIndex, f.m.getDebugScope(), f.m.fileDebugIndex, f.line, f.column,
	)

	f.m.pushDebugScope(f.m.debugIndex)
	f.m.debugIndex++
}

func (f *Function) PopScope() {
	f.m.popDebugScope()
}

func (f *Function) LocalVariable(v IdentifierValue, name string, arg uint32) {
	if _, ok := v.type_.(*pointerType); !ok {
		panic("llvm.Function.LocalVariable() - Value type needs to be a pointer")
	}

	locDebugIndex := f.getLocationDebugIndex()

	if locDebugIndex == math.MaxUint32 {
		panic("llvm.Function.LocalVariable() - Local variable needs source location information")
	}

	_, _ = fmt.Fprintf(
		&f.m.footer,
		"!%d = !DILocalVariable(name: \"%s\", type: !%d, arg: %d, scope: !%d, file: !%d, line: %d)\n",
		f.m.debugIndex, name, v.type_.(*pointerType).pointee.debugIndex(), arg, f.m.getDebugScope(), f.m.fileDebugIndex, f.line,
	)

	_, _ = fmt.Fprintf(
		&f.m.body,
		"    #dbg_declare(ptr %s, !%d, !DIExpression(), !%d)\n",
		v.String(), f.m.debugIndex, locDebugIndex,
	)

	f.m.debugIndex++
}

func (f *Function) End() {
	f.m.body.WriteString("}\n")
	f.m.popDebugScope()
}

// Value

func (f *Function) Type() Type {
	return f.type_
}

func (f *Function) String() string {
	return "@" + f.name
}

// Terminator Instructions

func Ret(f *Function) {
	if f.skipInstructions {
		return
	}

	f.instruction(IdentifierValue{}, "ret void")

	f.skipInstructions = true
}

func RetValue[V Value](f *Function, v V) {
	if f.skipInstructions {
		return
	}

	f.instruction(IdentifierValue{}, "ret %s %s", v.Type(), v.String())

	f.skipInstructions = true
}

func Br(f *Function, l Identifier) {
	if f.skipInstructions {
		return
	}

	f.instruction(IdentifierValue{}, "br label %s", l.String())

	f.skipInstructions = true
}

func BrCond[V Value](f *Function, v V, trueL, falseL Identifier) {
	if f.skipInstructions {
		return
	}

	f.instruction(IdentifierValue{}, "br %s %s, label %s, label %s", v.Type(), v.String(), trueL.String(), falseL.String())

	f.skipInstructions = true
}

// Unary Instructions

func NegF[V Value](f *Function, v V, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v.Type(), name)
	f.instruction(result, "fneg %s %s", v.Type(), v.String())
	return result
}

// Binary Instructions

func Add[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)

	inst := "add"
	if t, ok := v1.Type().(*simpleType); ok && (t.text == "float" || t.text == "double") {
		inst = "fadd"
	}

	f.instruction(result, "%s %s %s, %s", inst, v1.Type(), v1, v2.String())
	return result
}

func Sub[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)

	inst := "sub"
	if t, ok := v1.Type().(*simpleType); ok && (t.text == "float" || t.text == "double") {
		inst = "fsub"
	}

	f.instruction(result, "%s %s %s, %s", inst, v1.Type(), v1, v2.String())
	return result
}

func Mul[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)

	inst := "mul"
	if t, ok := v1.Type().(*simpleType); ok && (t.text == "float" || t.text == "double") {
		inst = "fmul"
	}

	f.instruction(result, "%s %s %s, %s", inst, v1.Type(), v1, v2.String())
	return result
}

func Div[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)

	inst := "fdiv"
	if t, ok := v1.Type().(*integerType); ok {
		if t.signed {
			inst = "sdiv"
		} else {
			inst = "udiv"
		}
	}

	f.instruction(result, "%s %s %s, %s", inst, v1.Type(), v1, v2.String())
	return result
}

func Rem[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)

	inst := "frem"
	if t, ok := v1.Type().(*integerType); ok {
		if t.signed {
			inst = "srem"
		} else {
			inst = "urem"
		}
	}

	f.instruction(result, "%s %s %s, %s", inst, v1.Type(), v1, v2.String())
	return result
}

// Bitwise Binary Instructions

func Shl[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)
	f.instruction(result, "shl %s %s, %s", v1.Type(), v1.String(), v2.String())
	return result
}

func Shr[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)

	inst := "lshr"
	if t, ok := v1.Type().(*integerType); ok && t.signed {
		inst = "ashr"
	}

	f.instruction(result, "%s %s %s, %s", inst, v1.Type(), v1, v2.String())
	return result
}

func And[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)
	f.instruction(result, "and %s %s, %s", v1.Type(), v1.String(), v2.String())
	return result
}

func Or[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)
	f.instruction(result, "or %s %s, %s", v1.Type(), v1.String(), v2.String())
	return result
}

func Xor[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)
	f.instruction(result, "xor %s %s, %s", v1.Type(), v1.String(), v2.String())
	return result
}

// Aggregate Instructions

func ExtractValue[V Value](f *Function, v V, index uint32, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	var t Type

	if a, ok := v.Type().(*arrayType); ok {
		t = a.element
	} else if s, ok := v.Type().(*structType); ok {
		t = s.fields[index].Type
	} else {
		panic("llvm.ExtractValue() - Invalid value type")
	}

	result := f.getIdentifierValue(t, name)
	f.instruction(result, "extractvalue %s %s, %d", v.Type(), v.String(), index)
	return result
}

func InsertValue[V1, V2 Value](f *Function, v1 V1, v2 V2, index uint32, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)
	f.instruction(result, "insertvalue %s %s, %s %s, %d", v1.Type(), v1.String(), v2.Type(), v2.String(), index)
	return result
}

// Memory Instructions

func Alloca(f *Function, type_ Type, count uint32, align uint32, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(&pointerType{
		baseType: baseType{
			size_:  64,
			align_: 64,
			dbg:    math.MaxUint32,
		},
		pointee: type_,
	}, name)

	f.instruction(result, "alloca %s, i32 %d, align %d", type_, count, align)
	return result
}

func Load[V Value](f *Function, v V, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	t := v.Type().(*pointerType).pointee
	result := f.getIdentifierValue(t, name)
	f.instruction(result, "load %s, ptr %s", t, v.String())
	return result
}

func Store[V1, V2 Value](f *Function, valueV V1, ptrV V2) {
	if f.skipInstructions {
		return
	}

	f.instruction(IdentifierValue{}, "store %s %s, ptr %s", valueV.Type(), valueV.String(), ptrV.String())
}

func GetElementPtr1[V, I Value](f *Function, v V, i I, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	pointee := v.Type().(*pointerType).pointee

	result := f.getIdentifierValue(v.Type(), name)
	f.instruction(result, "getelementptr %s, ptr %s, %s %s", pointee, v.String(), i.Type(), i.String())
	return result
}

func GetElementPtr2[V Value](f *Function, v V, i1, i2 uint32, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	pointee := v.Type().(*pointerType).pointee
	var t Type

	if a, ok := pointee.(*arrayType); ok {
		t = a.element
	} else if s, ok := pointee.(*structType); ok {
		t = s.fields[i2].Type
	} else {
		panic("llvm.GetElementPtr() - Invalid pointee type")
	}

	result := f.getIdentifierValue(&pointerType{baseType: baseType{size_: 64, align_: 64, dbg: math.MaxUint32}, pointee: t}, name)
	f.instruction(result, "getelementptr %s, ptr %s, i32 %d, i32 %d", pointee, v.String(), i1, i2)
	return result
}

// Conversion Instructions

func Trunc[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v2.Type(), name)

	inst := "trunc"
	if t, ok := v2.Type().(*simpleType); ok && (t.text == "float" || t.text == "double") {
		inst = "ftrunc"
	}

	f.instruction(result, "%s %s %s to %s", inst, v1.Type(), v1, v2.Type())
	return result
}

func Ext[V Value](f *Function, v V, t Type, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(t, name)

	inst := "fpext"
	if t, ok := t.(*integerType); ok {
		if t.signed {
			inst = "sext"
		} else {
			inst = "zext"
		}
	}

	f.instruction(result, "%s %s %s to %s", inst, v.Type(), v, t)
	return result
}

func FloatingToInt[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v2.Type(), name)

	inst := "fptoui"
	if t, ok := v2.Type().(*integerType); ok && t.signed {
		inst = "fptosi"
	}

	f.instruction(result, "%s %s %s to %s", inst, v1.Type(), v1, v2.Type())
	return result
}

func IntToFloating[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v2.Type(), name)

	inst := "uitofp"
	if t, ok := v2.Type().(*integerType); ok && t.signed {
		inst = "sitofp"
	}

	f.instruction(result, "%s %s %s to %s", inst, v1.Type(), v1, v2.Type())
	return result
}

func PtrToInt[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v2.Type(), name)
	f.instruction(result, "ptrtoint ptr %s to %s", v1, v2.Type())
	return result
}

func IntToPtr[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v2.Type(), name)
	f.instruction(result, "inttoptr %s %s to ptr", v1.Type(), v1)
	return result
}

func BitCast[V1, V2 Value](f *Function, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v2.Type(), name)
	f.instruction(result, "bitcast %s %s to %s", v1.Type(), v1, v2.Type())
	return result
}

// Other Instructions

type CmpIOp string

const (
	IEQ  CmpIOp = "eq"
	INQ  CmpIOp = "ne"
	IUGT CmpIOp = "ugt"
	IUGE CmpIOp = "uge"
	IULT CmpIOp = "ult"
	IULE CmpIOp = "ule"
	ISGT CmpIOp = "sgt"
	ISGE CmpIOp = "sge"
	ISLT CmpIOp = "slt"
	ISLE CmpIOp = "sle"
)

func CmpI[V1, V2 Value](f *Function, op CmpIOp, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(I1, name)
	f.instruction(result, "icmp %s %s %s, %s", op, v1.Type(), v1.String(), v2.String())
	return result
}

type CmpFOp string

const (
	FOEQ CmpFOp = "oeq"
	FONQ CmpFOp = "onq"
	FOGT CmpFOp = "ugt"
	FOGE CmpFOp = "uge"
	FOLT CmpFOp = "ult"
	FOLE CmpFOp = "ule"
	FUEQ CmpFOp = "ueq"
	FUNQ CmpFOp = "unq"
	FUGT CmpFOp = "ugt"
	FUGE CmpFOp = "uge"
	FULT CmpFOp = "ult"
	FULE CmpFOp = "ule"
)

func CmpF[V1, V2 Value](f *Function, op CmpFOp, v1 V1, v2 V2, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(I1, name)
	f.instruction(result, "fcmp %s %s %s, %s", op, v1.Type(), v1.String(), v2.String())
	return result
}

func Phi[V1, V2 Value](f *Function, v1 V1, l1 Identifier, v2 V2, l2 Identifier, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(v1.Type(), name)
	f.instruction(result, "phi %s [ %s, %s ], [ %s, %s ]", v1.Type(), v1.String(), l1.String(), v2.String(), l2.String())
	return result
}

func Select[V1, V2, V3 Value](f *Function, condV V1, trueV V2, falseV V3, name string) IdentifierValue {
	if f.skipInstructions {
		return IdentifierValue{}
	}

	result := f.getIdentifierValue(trueV.Type(), name)
	f.instruction(result, "select %s %s, %s %s, %s %s", condV.Type(), condV.String(), trueV.Type(), trueV.String(), falseV.Type(), falseV.String())
	return result
}

type CallInstruction struct {
	f      *Function
	result IdentifierValue
	argI   uint32
}

func Call[V Value](f *Function, v V, name string) CallInstruction {
	if f.skipInstructions {
		return CallInstruction{f: nil}
	}

	t := getFunctionType(v.Type())

	result := IdentifierValue{}
	if s, ok := t.returns.(*simpleType); !ok || s.text != "void" {
		result = f.getIdentifierValue(t.returns, name)
	}

	f.instructionStart(result)
	_, _ = fmt.Fprintf(&f.m.body, "call %s %s(", t.returns, v.String())

	return CallInstruction{
		f:      f,
		result: result,
		argI:   0,
	}
}

func Arg[V Value](c *CallInstruction, v V) {
	if c.f == nil {
		return
	}

	if c.argI > 0 {
		c.f.m.body.WriteString(", ")
	}

	_, _ = fmt.Fprintf(&c.f.m.body, "%s %s", v.Type(), v.String())

	c.argI++
}

func (c CallInstruction) End() IdentifierValue {
	if c.f == nil {
		return IdentifierValue{}
	}

	c.f.m.body.WriteRune(')')
	c.f.instructionEnd()

	return c.result
}

// Utils

func (f *Function) getIdentifierValue(type_ Type, name string) IdentifierValue {
	v := IdentifierValue{
		type_:      type_,
		identifier: Identifier{number: math.MaxUint32, name: name},
	}

	if name == "" {
		v.identifier.number = f.localIdentifierNumber
		f.localIdentifierNumber++
	}

	return v
}

func (f *Function) instruction(result IdentifierValue, format string, args ...any) {
	f.instructionStart(result)
	_, _ = fmt.Fprintf(&f.m.body, format, args...)
	f.instructionEnd()
}

func (f *Function) instructionStart(result IdentifierValue) {
	f.m.body.WriteString("  ")

	if result.type_ != nil {
		f.m.body.WriteString(result.String())
		f.m.body.WriteString(" = ")
	}
}

func (f *Function) instructionEnd() {
	if dbg := f.getLocationDebugIndex(); dbg != math.MaxUint32 {
		_, _ = fmt.Fprintf(&f.m.body, ", !dbg !%d\n", dbg)
	} else {
		f.m.body.WriteRune('\n')
	}
}

func (f *Function) getLocationDebugIndex() uint32 {
	if f.line == 0 && f.column == 0 {
		return math.MaxUint32
	}

	if f.locationDebugIndex == math.MaxUint32 {
		f.locationDebugIndex = f.m.debugIndex
		f.m.debugIndex++

		_, _ = fmt.Fprintf(
			&f.m.footer,
			"!%d = !DILocation(scope: !%d, line: %d, column: %d)\n",
			f.locationDebugIndex, f.m.getDebugScope(), f.line, f.column,
		)
	}

	return f.locationDebugIndex
}
