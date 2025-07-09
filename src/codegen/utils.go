package codegen

import (
	"fireball/ast"
	"fireball/lexer"
	"strconv"
	"strings"
)

func GetLinkName(f *ast.Func) string {
	if f.GetAttribute("extern") != nil {
		return f.Name()
	}

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

func writePath(sb *strings.Builder, path *ast.Path) {
	for i := 0; i < path.SegmentCount(); i++ {
		sb.WriteString(path.SegmentAt(i))
		sb.WriteRune('.') // TODO: replace with :
	}
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
