package llvm

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
)

type Module struct {
	header strings.Builder

	body strings.Builder

	footer     strings.Builder
	debugIndex uint32

	fileDebugIndex uint32
	cuDebugIndex   uint32
	debugScope     []uint32
}

func NewModule(filename, dataLayout, triple string) *Module {
	m := &Module{}

	m.fileDebugIndex = m.debugIndex
	m.debugIndex++

	m.cuDebugIndex = m.debugIndex
	m.debugIndex++

	flagsDebugIndex := m.debugIndex
	m.debugIndex += 7

	identDebugIndex := m.debugIndex
	m.debugIndex++

	m.pushDebugScope(m.fileDebugIndex)

	// Header
	_, _ = fmt.Fprintf(&m.header, "source_filename = \"%s\"\n", filename)
	_, _ = fmt.Fprintf(&m.header, "target datalayout = \"%s\"\n", dataLayout)
	_, _ = fmt.Fprintf(&m.header, "target triple = \"%s\"\n", triple)

	// Footer
	m.footer.WriteRune('\n')

	_, _ = fmt.Fprintf(
		&m.footer,
		"!llvm.dbg.cu = !{!%d}\n",
		m.cuDebugIndex,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!llvm.module.flags = !{!%d, !%d, !%d, !%d, !%d, !%d, !%d}\n",
		flagsDebugIndex+0, flagsDebugIndex+1, flagsDebugIndex+2, flagsDebugIndex+3, flagsDebugIndex+4, flagsDebugIndex+5, flagsDebugIndex+6,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!llvm.ident = !{!%d}\n",
		identDebugIndex,
	)

	m.footer.WriteRune('\n')

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DIFile(filename: \"%s\", directory: \"%s\")\n",
		m.fileDebugIndex, filepath.Base(filename), filepath.Dir(filename),
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = distinct !DICompileUnit(language: DW_LANG_C, file: !%d, producer: \"fireball\", isOptimized: false, runtimeVersion: 0, emissionKind: FullDebug, splitDebugInlining: false, nameTableKind: None)\n",
		m.cuDebugIndex, m.fileDebugIndex,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !{i32 7, !\"Dwarf Version\", i32 4}\n",
		flagsDebugIndex+0,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !{i32 2, !\"Debug Info Version\", i32 3}\n",
		flagsDebugIndex+1,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !{i32 1, !\"wchar_size\", i32 4}\n",
		flagsDebugIndex+2,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !{i32 8, !\"PIC Level\", i32 2}\n",
		flagsDebugIndex+3,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !{i32 7, !\"PIE Level\", i32 2}\n",
		flagsDebugIndex+4,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !{i32 7, !\"uwtable\", i32 2}\n",
		flagsDebugIndex+5,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !{i32 7, !\"frame-pointer\", i32 2}\n",
		flagsDebugIndex+6,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !{!\"fireball\"}\n",
		identDebugIndex,
	)

	return m
}

func (m *Module) pushDebugScope(scope uint32) {
	m.debugScope = append(m.debugScope, scope)
}

func (m *Module) getDebugScope() uint32 {
	return m.debugScope[len(m.debugScope)-1]
}

func (m *Module) popDebugScope() {
	m.debugScope = m.debugScope[:len(m.debugScope)-1]
}

// Types

func (m *Module) NewVoidType() Type {
	t := &simpleType{
		baseType: baseType{
			size_:  0,
			align_: 0,
			dbg:    math.MaxUint32,
		},
		text: "void",
	}

	return t
}

func (m *Module) NewIntegerType(signed bool, bitCount uint32) Type {
	t := &integerType{
		baseType: baseType{
			size_:  bitCount,
			align_: bitCount,
			dbg:    m.debugIndex,
		},
		signed:   signed,
		bitCount: bitCount,
	}

	name := "bool"
	encoding := "DW_ATE_boolean"
	sizeAlign := uint32(8)

	if bitCount > 1 {
		if signed {
			name = fmt.Sprintf("i%d", bitCount)
			encoding = "DW_ATE_signed"
		} else {
			name = fmt.Sprintf("u%d", bitCount)
			encoding = "DW_ATE_unsigned"
		}

		sizeAlign = t.size_
	}

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DIBasicType(name: \"%s\", encoding: %s, size: %d, align: %d)\n",
		m.debugIndex, name, encoding, sizeAlign, sizeAlign,
	)

	m.debugIndex++
	return t
}

func (m *Module) NewFloatingType(double bool) Type {
	t := &simpleType{
		baseType: baseType{
			size_:  32,
			align_: 32,
			dbg:    m.debugIndex,
		},
		text: "float",
	}

	if double {
		t.text = "double"
		t.size_ = 64
		t.align_ = 64
	}

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DIBasicType(name: \"%s\", encoding: %s, size: %d, align: %d)\n",
		m.debugIndex, t.text, "DW_ATE_float", t.size_, t.align_,
	)

	m.debugIndex++
	return t
}

func (m *Module) NewArrayType(count uint32, element Type) Type {
	t := &arrayType{
		baseType: baseType{
			size_:  element.size() * count,
			align_: element.align(),
			dbg:    m.debugIndex,
		},
		count:   count,
		element: element,
	}

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DICompositeType(tag: DW_TAG_array_type, baseType: !%d, elements: !{!%d}, size: %d, align: %d)\n",
		m.debugIndex, element.debugIndex(), m.debugIndex+1, t.size_, t.align_,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DISubrange(count: %d)\n",
		m.debugIndex+1, count,
	)

	m.debugIndex += 2
	return t
}

func (m *Module) NewPointerType(pointee Type) Type {
	t := &pointerType{
		baseType: baseType{
			size_:  64,
			align_: 64,
			dbg:    m.debugIndex,
		},
		pointee: pointee,
	}

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !%d, size: %d, align: %d)\n",
		m.debugIndex, pointee.debugIndex(), t.size_, t.align_,
	)

	m.debugIndex++
	return t
}

func (m *Module) NewStructType(name string, fields []Field, size uint32, align uint32) Type {
	t := &structType{
		baseType: baseType{
			size_:  size * 8,
			align_: align * 8,
			dbg:    m.debugIndex,
		},
		name:   name,
		fields: fields,
	}

	// Declaration
	t.write(&m.header)
	m.header.WriteString(" = type { ")

	for i, field := range fields {
		if i > 0 {
			m.header.WriteString(", ")
		}

		field.Type.write(&m.header)
	}

	m.header.WriteString(" }\n")

	// Metadata
	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = distinct !DICompositeType(tag: DW_TAG_structure_type, name: \"%s\", size: %d, align: %d, elements: !{",
		m.debugIndex, name, t.size_, t.align_,
	)

	for i := range fields {
		if i > 0 {
			m.footer.WriteString(", ")
		}

		m.footer.WriteRune('!')
		_, _ = fmt.Fprint(&m.footer, m.debugIndex+1+uint32(i))
	}

	m.footer.WriteString("})\n")

	for i, field := range fields {
		_, _ = fmt.Fprintf(
			&m.footer,
			"!%d = !DIDerivedType(tag: DW_TAG_member, name: \"%s\", baseType: !%d, offset: %d, size: %d, align: %d)\n",
			m.debugIndex+1+uint32(i), field.Name, field.Type.debugIndex(), field.Offset*8, field.Type.size(), field.Type.align(),
		)
	}

	m.debugIndex += 1 + uint32(len(fields))
	return t
}

func (m *Module) NewFunctionType(returns Type, params []Type, vararg bool) Type {
	t := &functionType{
		baseType: baseType{
			size_:  64,
			align_: 64,
			dbg:    m.debugIndex,
		},
		returns: returns,
		params:  params,
		vararg:  vararg,
	}

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DISubroutineType(types: !{",
		m.debugIndex,
	)

	if returns.debugIndex() == math.MaxUint32 {
		m.footer.WriteString("null")
	} else {
		_, _ = fmt.Fprintf(&m.footer, "!%d", returns.debugIndex())
	}

	for _, param := range params {
		_, _ = fmt.Fprintf(&m.footer, ", !%d", param.debugIndex())
	}

	m.footer.WriteString("})\n")

	m.debugIndex += 1 + 1 + uint32(len(params))
	return t
}

// Global Variables

func (m *Module) NewStringConstant(name, value string) *GlobalValue {
	m.header.WriteRune('\n')

	type_ := m.NewArrayType(uint32(len(value))+1, U8)
	var sb strings.Builder

	for _, b := range value {
		switch b {
		case '\000':
			sb.WriteString("\\00")
		case '\n':
			sb.WriteString("\\0A")
		case '\r':
			sb.WriteString("\\0D")
		case '\t':
			sb.WriteString("\\09")

		default:
			sb.WriteRune(b)
		}
	}

	sb.WriteString("\\00")
	value = sb.String()

	_, _ = fmt.Fprintf(
		&m.header,
		"@%s = private unnamed_addr constant %s c\"%s\", align 1, !dbg !%d\n",
		name, type_, value, m.debugIndex,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DIGlobalVariableExpression(var: !%d, expr: !DIExpression())\n",
		m.debugIndex, m.debugIndex+1,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = distinct !DIGlobalVariable(name: \"%s\", type: !%d, file: !%d, isLocal: true, isDefinition: true)\n",
		m.debugIndex+1, name, type_.debugIndex(), m.fileDebugIndex,
	)

	m.debugIndex += 2

	return &GlobalValue{
		name: name,
		type_: &pointerType{
			baseType: baseType{size_: 64, align_: 64, dbg: math.MaxUint32},
			pointee:  U8,
		},
	}
}

func (m *Module) NewGlobalVariable(name string, type_ Type, definition bool) *GlobalValue {
	m.header.WriteRune('\n')

	_, _ = fmt.Fprintf(
		&m.header,
		"@%s = external global %s, align %d, !dbg !%d\n",
		name, type_, type_.align(), m.debugIndex,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = !DIGlobalVariableExpression(var: !%d, expr: !DIExpression())\n",
		m.debugIndex, m.debugIndex+1,
	)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = distinct !DIGlobalVariable(name: \"%s\", type: !%d, file: !%d, isLocal: %t, isDefinition: %t)\n",
		m.debugIndex+1, name, type_.debugIndex(), m.fileDebugIndex, definition, definition,
	)

	m.debugIndex += 2

	return &GlobalValue{
		name:  name,
		type_: type_,
	}
}

// Functions

func (m *Module) NewFunction(name string, type_ Type, paramNames []string) *Function {
	if _, ok := type_.(*functionType); !ok {
		panic("llvm.Module.NewFunction() - Needs to be a function type")
	}

	f := type_.(*functionType)

	if len(f.params) != len(paramNames) {
		panic("llvm.Module.NewFunction() - Count of parameter types and names needs to be the same")
	}

	m.body.WriteRune('\n')

	m.body.WriteString("define dso_local ")
	m.function(name, f, paramNames)
	_, _ = fmt.Fprintf(&m.body, " !dbg !%d {\n", m.debugIndex)

	_, _ = fmt.Fprintf(
		&m.footer,
		"!%d = distinct !DISubprogram(name: \"%s\", unit: !%d, scope: !%d, file: !%d, type: !%d, flags: DIFlagPrototyped, spFlags: DISPFlagDefinition)\n",
		m.debugIndex, name, m.cuDebugIndex, m.getDebugScope(), m.fileDebugIndex, type_.debugIndex(),
	)

	m.pushDebugScope(m.debugIndex)
	m.debugIndex++

	return &Function{
		m:     m,
		name:  name,
		type_: f,
	}
}

func (m *Module) NewExternFunction(name string, type_ Type) *ExternFunction {
	if _, ok := type_.(*functionType); !ok {
		panic("llvm.Module.NewFunction() - Needs to be a function type")
	}

	f := type_.(*functionType)

	m.body.WriteRune('\n')

	m.body.WriteString("declare ")
	m.function(name, f, nil)
	m.body.WriteRune('\n')

	return &ExternFunction{
		name:  name,
		type_: f,
	}
}

func (m *Module) function(name string, f *functionType, paramNames []string) {
	f.returns.write(&m.body)
	m.body.WriteString(" @")
	m.body.WriteString(name)
	m.body.WriteRune('(')

	for i, param := range f.params {
		if i > 0 {
			m.body.WriteString(", ")
		}

		param.write(&m.body)

		if len(paramNames) > 0 {
			m.body.WriteString(" %")
			m.body.WriteString(paramNames[i])
		}
	}

	if f.vararg {
		if len(f.params) > 0 {
			m.body.WriteString(", ...")
		} else {
			m.body.WriteString("...")
		}
	}

	m.body.WriteRune(')')
}

// Write

func (m *Module) Write(w io.Writer) error {
	bw := bufio.NewWriter(w)

	//goland:noinspection GoUnhandledErrorResult
	defer bw.Flush()

	if _, err := bw.WriteString(m.header.String()); err != nil {
		return err
	}
	if _, err := bw.WriteString(m.body.String()); err != nil {
		return err
	}
	if _, err := bw.WriteString(m.footer.String()); err != nil {
		return err
	}

	return nil
}
