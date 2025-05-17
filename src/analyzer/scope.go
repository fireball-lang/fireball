package analyzer

import "fireball/ast"

type Scope interface {
	GetTypeDecl(name string) ast.Decl
}

type simpleScope struct {
	types map[string]ast.Decl
}

func NewSimpleScope(decls []ast.Decl) Scope {
	s := &simpleScope{types: make(map[string]ast.Decl)}

	for _, decl := range decls {
		s.types[decl.Name()] = decl
	}

	return s
}

func (s *simpleScope) GetTypeDecl(name string) ast.Decl {
	return s.types[name]
}
