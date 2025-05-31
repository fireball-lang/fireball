package analyzer

import (
	"fireball/ast"
	"fireball/utils"
)

func ResolveTypes(node ast.Node, scope Scope) (diagnostics []utils.Diagnostic) {
	if declType, ok := node.(*ast.DeclType); ok {
		decl := scope.GetTypeDecl(declType.Name.Token.Text)

		if ast.IsValid(decl) {
			declType.Decl = decl
		} else {
			diagnostics = append(diagnostics, utils.Diagnostic{
				Kind:    utils.Error,
				Message: "Type with the name '" + declType.Name.Token.Text + "' doesn't exist.",
				Range:   declType.Name.Range(),
			})
		}
	} else {
		for child := range node.Children() {
			diagnostics = ResolveTypes(child, scope)
		}
	}

	return
}
