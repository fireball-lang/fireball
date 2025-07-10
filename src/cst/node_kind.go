package cst

type NodeKind uint8

const (
	Leaf NodeKind = iota
	Path

	File

	DeclType
	ArrayType
	PointerType
	FuncType

	Mod
	Import
	Struct
	Field
	Impl
	GlobalVar
	Func
	Param
	Attributes
	Attribute

	Block
	Var
	If
	While
	For
	Break
	Continue
	Return

	Literal
	StructInitializer
	StructInitializerField
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
	return n == Mod || n == Import || n == Struct || n == Impl || n == GlobalVar || n == Func
}

func (n NodeKind) IsExpr() bool {
	return n >= Block && n <= Cast
}

func (n NodeKind) String() string {
	switch n {
	case Leaf:
		return "Leaf"
	case Path:
		return "Path"

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

	case Mod:
		return "Mod"
	case Import:
		return "Import"
	case Struct:
		return "Struct"
	case Field:
		return "Field"
	case Impl:
		return "Impl"
	case GlobalVar:
		return "GlobalVar"
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
	case Break:
		return "Break"
	case Continue:
		return "Continue"
	case Return:
		return "Return"

	case Literal:
		return "Literal"
	case StructInitializer:
		return "StructInitializer"
	case StructInitializerField:
		return "StructInitializerField"
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
