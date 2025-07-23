package codegen

import (
	"fireball/ast"
	"fireball/lexer"
	"strconv"
	"strings"
)

func GetGlobalVarLinkName(g *ast.GlobalVar) string {
	// Extern
	if g.GetAttribute("extern") != nil {
		return g.Name()
	}

	// Normal
	var sb strings.Builder
	sb.WriteString("fb$")

	modPath := ast.Root(g).ModulePath()
	writePath(&sb, modPath)

	sb.WriteString(g.Name())
	return sb.String()
}

func GetFuncLinkName(f *ast.Func) string {
	// Extern
	if f.GetAttribute("extern") != nil {
		return f.Name()
	}

	// Normal
	var sb strings.Builder
	sb.WriteString("fb$")

	if impl, ok := f.Parent().(*ast.Impl); ok {
		modPath := ast.Root(impl).ModulePath()
		writePath(&sb, modPath)

		sb.WriteString(impl.Name())
		sb.WriteRune('.')
	} else {
		modPath := ast.Root(f).ModulePath()
		writePath(&sb, modPath)
	}

	sb.WriteString(f.Name())
	return sb.String()
}

func GetDeclLinkName(decl ast.Decl) string {
	var sb strings.Builder

	modPath := ast.Root(decl).ModulePath()
	writePath(&sb, modPath)

	sb.WriteString(decl.Name())
	return sb.String()
}

func GetTypeInfoLinkName(decl ast.Decl) string {
	return "type_info - " + GetDeclLinkName(decl)
}

func GetVTableLinkName(decl ast.Decl, in *ast.Interface) string {
	var sb strings.Builder

	sb.WriteString("vtable - ")
	sb.WriteString(GetDeclLinkName(in))
	sb.WriteString(" for ")
	sb.WriteString(GetDeclLinkName(decl))

	return sb.String()
}

func writePath(sb *strings.Builder, path *ast.Path) {
	for i := 0; i < path.SegmentCount(); i++ {
		sb.WriteString(path.SegmentAt(i))
		sb.WriteRune(':')
	}
}

func getClosestValidRange(node ast.Node) lexer.Range {
	for node.Range().IsZero() {
		node = node.Parent()

		if !ast.IsValid(node) {
			panic("codegen.getClosestValidRange() - Failed to find valid range")
		}
	}

	return node.Range()
}

// stringBuilder

type stringBuilder struct {
	builder strings.Builder
	length  uint32
}

func (s *stringBuilder) WriteRune(r rune) {
	s.builder.WriteRune(r)
	s.length++
}

func (s *stringBuilder) WriteEscapeBackslash() {
	s.builder.WriteString("\\\\")
	s.length++
}

func (s *stringBuilder) WriteEscapeSequence(esc uint8) {
	s.builder.WriteRune('\\')

	str := strconv.FormatUint(uint64(esc), 16)
	if len(str) == 1 {
		s.builder.WriteRune('0')
	}
	s.builder.WriteString(str)

	s.length++
}

func (s *stringBuilder) String() string {
	return s.builder.String()
}

func (s *stringBuilder) Error(r lexer.Range, msg string) {
	panic("codegen.stringBuilder.Error() - Shouldn't happen")
}
