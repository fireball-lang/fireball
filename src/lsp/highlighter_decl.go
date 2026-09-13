package lsp

import (
	"fireball/ast"
)

func (hi *highlighter) VisitTypeAlias(t *ast.TypeAlias) {
	hi.AddFull(t.Name(), typeKind, 0)
	hi.VisitTypeParams(t.TypeParams)
	hi.VisitType(t.Type)
}

func (hi *highlighter) VisitStruct(s *ast.Struct) {
	hi.AddFull(s.Name(), classKind, 0)
	hi.VisitTypeParams(s.TypeParams)

	for _, field := range s.Fields {
		hi.AddFull(field.Name, propertyKind, 0)
		hi.VisitType(field.Type)
	}
}

func (hi *highlighter) VisitEnum(e *ast.Enum) {
	hi.AddFull(e.Name(), enumKind, 0)

	for _, c := range e.Cases {
		hi.AddFull(c.Name, enumMemberKind, 0)
	}
}

func (hi *highlighter) VisitInterface(in *ast.Interface) {
	hi.AddFull(in.Name(), interfaceKind, 0)
	hi.VisitTypeParams(in.TypeParams)

	for _, method := range in.Methods {
		hi.VisitFunc(method)
	}
}

func (hi *highlighter) VisitImpl(im *ast.Impl) {
	hi.VisitTypeParams(im.TypeParams)

	hi.AddFull(im.Type, typeKind, 0)
	hi.AddFull(im.Interface, interfaceKind, 0)

	for _, method := range im.Methods {
		hi.VisitFunc(method)
	}
}

func (hi *highlighter) VisitConst(c *ast.Const) {
	hi.AddFull(c.Name(), variableKind, readonlyKind)
	hi.VisitType(c.Type)
	hi.VisitExpr(c.Value)
}

func (hi *highlighter) VisitGlobalVar(g *ast.GlobalVar) {
	hi.AddFull(g.Name(), variableKind, 0)
	hi.VisitType(g.Type)
	hi.VisitExpr(g.Initializer)
}

func (hi *highlighter) VisitFunc(f *ast.Func) {
	hi.AddFull(f.Name(), functionKind, 0)
	hi.VisitTypeParams(f.TypeParams)

	hi.Add(f.Receiver, keywordKind, 0)

	for _, param := range f.Params {
		hi.AddFull(param.Name, parameterKind, 0)
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
		hi.AddFull(param.Name, genericKind, 0)

		for _, constraint := range param.Constraints {
			hi.AddFull(constraint, interfaceKind, 0)
		}
	}
}

func (hi *highlighter) VisitDecl(decl ast.Decl) {
	ast.VisitDecl(hi, decl)
}
