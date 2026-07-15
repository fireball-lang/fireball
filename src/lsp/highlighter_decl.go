package lsp

import (
	"fireball/ast"
)

func (hi *highlighter) VisitStruct(s *ast.Struct) {
	hi.AddFull(s.Name(), classKind)
	hi.VisitTypeParams(s.TypeParams)

	for _, field := range s.Fields {
		hi.AddFull(field.Name, propertyKind)
		hi.VisitType(field.Type)
	}
}

func (hi *highlighter) VisitEnum(e *ast.Enum) {
	hi.AddFull(e.Name(), enumKind)

	for _, c := range e.Cases {
		hi.AddFull(c.Name, enumMemberKind)
	}
}

func (hi *highlighter) VisitInterface(in *ast.Interface) {
	hi.AddFull(in.Name(), interfaceKind)
	hi.VisitTypeParams(in.TypeParams)

	for _, method := range in.Methods {
		hi.VisitFunc(method)
	}
}

func (hi *highlighter) VisitImpl(im *ast.Impl) {
	hi.VisitTypeParams(im.TypeParams)

	hi.AddFull(im.Type, typeKind)
	hi.AddFull(im.Interface, interfaceKind)

	for _, method := range im.Methods {
		hi.VisitFunc(method)
	}
}

func (hi *highlighter) VisitFunc(f *ast.Func) {
	hi.AddFull(f.Name(), functionKind)
	hi.VisitTypeParams(f.TypeParams)

	hi.AddFull(f.Receiver, keywordKind)

	for _, param := range f.Params {
		hi.AddFull(param.Name, parameterKind)
		hi.VisitType(param.Type)
	}

	hi.VisitType(f.Returns)

	hi.VisitStmt(f.Body)
}

func (hi *highlighter) VisitBadDecl(_ *ast.BadDecl) {
}

// Utils

func (hi *highlighter) VisitTypeParams(params []*ast.TypeParam) {
	for _, param := range params {
		hi.AddFull(param.Name, genericKind)

		for _, constraint := range param.Constraints {
			hi.AddFull(constraint, interfaceKind)
		}
	}
}

func (hi *highlighter) VisitDecl(decl ast.Decl) {
	ast.VisitDecl(hi, decl)
}
