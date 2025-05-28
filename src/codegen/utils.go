package codegen

import "fireball/ast"

func GetLinkName(f *ast.Func) string {
	if f.GetAttribute("extern") != nil {
		return f.Name()
	}

	return "fb$" + f.Name()
}
