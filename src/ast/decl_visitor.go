package ast

type DeclVisitor interface {
	VisitTypeAlias(t *TypeAlias)
	VisitStruct(s *Struct)
	VisitEnum(e *Enum)
	VisitInterface(i *Interface)
	VisitImpl(i *Impl)

	VisitGlobalVar(g *GlobalVar)
	VisitFunc(f *Func)

	VisitBadDecl(b *BadDecl)
}

func VisitDecl[V DeclVisitor](visitor V, decl Decl) {
	switch decl := decl.(type) {
	case *TypeAlias:
		visitor.VisitTypeAlias(decl)
	case *Struct:
		visitor.VisitStruct(decl)
	case *Enum:
		visitor.VisitEnum(decl)
	case *Interface:
		visitor.VisitInterface(decl)
	case *Impl:
		visitor.VisitImpl(decl)

	case *GlobalVar:
		visitor.VisitGlobalVar(decl)
	case *Func:
		visitor.VisitFunc(decl)

	case *BadDecl:
		visitor.VisitBadDecl(decl)

	default:
		panic("ast.VisitDecl() - Invalid decl")
	}
}
