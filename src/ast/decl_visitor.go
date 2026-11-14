package ast

type DeclVisitor interface {
	VisitTypeAlias(t *TypeAlias)
	VisitStruct(s *Struct)
	VisitEnum(e *Enum)
	VisitUnion(u *Union)
	VisitInterface(i *Interface)
	VisitImpl(i *Impl)

	VisitConst(c *Const)
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
	case *Union:
		visitor.VisitUnion(decl)
	case *Interface:
		visitor.VisitInterface(decl)
	case *Impl:
		visitor.VisitImpl(decl)

	case *Const:
		visitor.VisitConst(decl)
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
