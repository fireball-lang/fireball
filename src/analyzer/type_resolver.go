package analyzer

import (
	"fireball/ast"
	"fireball/utils"
)

type typeResolver struct {
	ctx   Context
	scope Scope

	diagnostics []utils.Diagnostic
}

func ResolveTypes(file *ast.File, ctx Context, scope Scope) []utils.Diagnostic {
	resolver := &typeResolver{ctx: ctx}
	resolver.scope = getFileScope(file, ctx, scope, resolver)

	resolver.visit(file)

	return resolver.diagnostics
}

func (t *typeResolver) visit(node ast.Node) {
	switch node := node.(type) {
	case *ast.Impl:
		if node.NameN == nil {
			node.Struct = nil
		} else {
			decl := t.scope.GetTypeDecl(node.Name())

			if s, ok := decl.(*ast.Struct); ok {
				node.Struct = s
			} else {
				addError(t, node.NameN, "Type with the name '"+node.Name()+"' is not a struct.")
			}
		}

	case *ast.StructInitializer:
		decl := t.scope.GetTypeDecl(node.Name.Token.Text)

		if s, ok := decl.(*ast.Struct); ok {
			node.Struct = s
		} else {
			addError(t, node.Name, "Type with the name '"+node.Name.Token.Text+"' is not a struct.")
		}

	case *ast.DeclType:
		node.Decl = nil

		lookup := getSymbolLookup(t.ctx, t.scope, node.Path)

		if !utils.IsNil(lookup) {
			node.Decl = lookup.GetTypeDecl(node.Path.SegmentAt(node.Path.SegmentCount() - 1))
		}

		if utils.IsNil(node.Decl) {
			addError(t, node.Path, "Type with the path '"+ast.PathString(node.Path)+"' doesn't exist in the current scope.")
		}
	}

	for child := range node.Children() {
		t.visit(child)
	}
}

func (t *typeResolver) Add(diag utils.Diagnostic) {
	t.diagnostics = append(t.diagnostics, diag)
}
