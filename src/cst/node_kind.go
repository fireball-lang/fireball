package cst

type NodeKind uint8

const (
	Leaf NodeKind = iota

	File

	DeclType

	Func
	Param

	Block
	Var
	If
	While
	For

	Literal
	Paren
	Identifier
	Call
	Index
	Member
	Unary
	Binary
)

func (n NodeKind) IsType() bool {
	return n == DeclType
}

func (n NodeKind) IsDecl() bool {
	return n == Func
}

func (n NodeKind) IsExpr() bool {
	return n >= Block && n <= Binary
}

func (n NodeKind) String() string {
	switch n {
	case Leaf:
		return "Leaf"

	case File:
		return "File"

	case DeclType:
		return "DeclType"

	case Func:
		return "Func"
	case Param:
		return "Param"

	case Block:
		return "Block"
	case Var:
		return "Var"
	case If:
		return "If"
	case While:
		return "While"
	case For:
		return "For"

	case Literal:
		return "Literal"
	case Paren:
		return "Paren"
	case Identifier:
		return "Identifier"
	case Call:
		return "Call"
	case Index:
		return "Index"
	case Member:
		return "Member"
	case Unary:
		return "Unary"
	case Binary:
		return "Binary"

	default:
		panic("cst.NodeKind.String() - Invalid")
	}
}
