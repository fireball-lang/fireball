package ast

type TypeVisitor[T any] interface {
	VisitPrimitiveType(p *PrimitiveType) T
	VisitArrayType(a *ArrayType) T
	VisitPointerType(p *PointerType) T
	VisitFuncType(f *FuncType) T

	VisitIdentifierType(i *IdentifierType) T
	VisitSelfType(s *SelfType) T

	VisitOptionType(o *OptionType) T
	VisitSliceType(s *SliceType) T

	VisitBadType(b *BadType) T
}

func VisitType[V TypeVisitor[T], T any](visitor V, typ Type) T {
	switch typ := typ.(type) {
	case *PrimitiveType:
		return visitor.VisitPrimitiveType(typ)
	case *ArrayType:
		return visitor.VisitArrayType(typ)
	case *PointerType:
		return visitor.VisitPointerType(typ)
	case *FuncType:
		return visitor.VisitFuncType(typ)

	case *IdentifierType:
		return visitor.VisitIdentifierType(typ)
	case *SelfType:
		return visitor.VisitSelfType(typ)

	case *OptionType:
		return visitor.VisitOptionType(typ)
	case *SliceType:
		return visitor.VisitSliceType(typ)

	case *BadType:
		return visitor.VisitBadType(typ)

	default:
		panic("ast.VisitType() - Invalid type")
	}
}
