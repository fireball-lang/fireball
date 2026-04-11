package ast

type DeclVisitor interface {
	VisitStruct(s *Struct)
	VisitImpl(i *Impl)
	VisitFunc(f *Func)

	VisitBadDecl(b *BadDecl)
}

func VisitDecl[V DeclVisitor](visitor V, decl Decl) {
	switch decl := decl.(type) {
	case *Struct:
		visitor.VisitStruct(decl)
	case *Impl:
		visitor.VisitImpl(decl)
	case *Func:
		visitor.VisitFunc(decl)

	case *BadDecl:
		visitor.VisitBadDecl(decl)

	default:
		panic("ast.VisitDecl() - Invalid decl")
	}
}
