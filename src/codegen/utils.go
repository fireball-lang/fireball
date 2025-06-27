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
		sb.WriteString(impl.Name())
		sb.WriteRune('.')
	}

	sb.WriteString(f.Name())
	return sb.String()
}
