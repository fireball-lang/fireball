package cst

type NodeKind uint8

const (
	Leaf NodeKind = iota

	File

	DeclType
	ArrayType
	PointerType
	FuncType

	Struct
	Field
	Func
	Param
	Attributes
	Attribute

	Block
	Var
	If
	While
	For
	Return

	Literal
	Paren
	Identifier
	Call
	Index
	Member
	Unary
	Binary
	Cast
)

func (n NodeKind) IsType() bool {
	return n >= DeclType && n <= FuncType
}

func (n NodeKind) IsDecl() bool {
	return n == Struct || n == Func
}

func (n NodeKind) IsExpr() bool {
	return n >= Block && n <= Cast
}

func (n NodeKind) String() string {
	switch n {
	case Leaf:
		return "Leaf"

	case File:
		return "File"

	case DeclType:
		return "DeclType"
	case ArrayType:
		return "ArrayType"
	case PointerType:
		return "PointerType"
	case FuncType:
		return "FuncType"

	case Struct:
		return "Struct"
	case Field:
		return "Field"
	case Func:
		return "Func"
	case Param:
		return "Param"
	case Attributes:
		return "Attributes"
	case Attribute:
		return "Attribute"

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
	case Return:
		return "Return"

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
	case Cast:
		return "Cast"

	default:
		panic("cst.NodeKind.String() - Invalid")
	}
}
