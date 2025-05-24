package analyzer

import "fireball/ast"

type Scope interface {
	GetTypeDecl(name string) ast.Decl
	GetFuncDecl(name string) *ast.Func
}
