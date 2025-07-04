package codegen

import (
	"fireball/ast"
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
