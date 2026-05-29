package ast

type DeclVisitor interface {
	VisitStruct(s *Struct)
	VisitInterface(i *Interface)
	VisitImpl(i *Impl)
	VisitFunc(f *Func)

	VisitBadDecl(b *BadDecl)
}

func VisitDecl[V DeclVisitor](visitor V, decl Decl) {
	switch decl := decl.(type) {
	case *Struct:
		visitor.VisitStruct(decl)
	case *Interface:
		visitor.VisitInterface(decl)
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
