package analyzer

import "fireball/ast"

type Scope interface {
	GetTypeDecl(name string) ast.Decl
	GetFuncDecl(name string) *ast.Func

	GetStructMethod(s *ast.Struct, name string) *ast.Func
}
