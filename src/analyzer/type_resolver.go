package analyzer

import (
	"fireball/ast"
	"fireball/utils"
)

func ResolveTypes(node ast.Node, scope Scope) (diagnostics []utils.Diagnostic) {
	switch node := node.(type) {
	case *ast.DeclType:
		decl := scope.GetTypeDecl(node.Name.Token.Text)
		if ast.IsValid(decl) {
			node.Decl = decl
		} else {
			diagnostics = append(diagnostics, utils.Diagnostic{
				Kind:    utils.Error,
				Message: "Type with the name '" + node.Name.Token.Text + "' doesn't exist.",
				Range:   node.Name.Range(),
			})
		}
	}

	for child := range node.Children() {
		diagnostics = append(diagnostics, ResolveTypes(child, scope)...)
	}

	return
}
