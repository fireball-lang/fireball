package ir

import "fireball/utils"

type MetaRef uint32

func (m MetaRef) Valid() bool {
	return m != 0
}

func (m MetaRef) Value() uint32 {
	if m == 0 {
		panic("ir.MetaRef.Value() - Meta reference is not valid")
	}

	return uint32(m) - 1
}

type MetaNode interface {
	next() MetaNode
	setNext(node MetaNode)
}

type baseMetaNode struct {
	nextNode MetaNode
}

func (b *baseMetaNode) next() MetaNode {
	return b.nextNode
}

func (b *baseMetaNode) setNext(node MetaNode) {
	b.nextNode = node
}

// Raw meta node

type RawMetaValue struct {
	Text   string
	Number uint32
}

type RawMeta struct {
	baseMetaNode

	Values []RawMetaValue
}

// Specialized meta nodes

type CompileUnitMeta struct {
	baseMetaNode

	File          MetaRef
	Producer      string
	IsOptimized   bool
	Enums         MetaRef
	RetainedTypes MetaRef
	Globals       MetaRef
	Imports       MetaRef
}

type FileMeta struct {
	baseMetaNode

	Path string
}

type MetaBasicTypeEncoding uint8

const (
	MetaAddress      MetaBasicTypeEncoding = 1
	MetaBoolean      MetaBasicTypeEncoding = 2
	MetaFloat        MetaBasicTypeEncoding = 4
	MetaSigned       MetaBasicTypeEncoding = 5
	MetaSignedChar   MetaBasicTypeEncoding = 6
	MetaUnsigned     MetaBasicTypeEncoding = 7
	MetaUnsignedChar MetaBasicTypeEncoding = 8
)

type BasicTypeMeta struct {
	baseMetaNode

	Name     string
	Encoding MetaBasicTypeEncoding
	Size     uint32
	Align    uint32
}

type SubroutineTypeMeta struct {
	baseMetaNode

	Returns MetaRef
	Params  []MetaRef
}

type MetaDerivedTypeKind uint8

const (
	MetaMember          MetaDerivedTypeKind = 13
	MetaPointerType     MetaDerivedTypeKind = 15
	MetaReferenceType   MetaDerivedTypeKind = 16
	MetaTypedef         MetaDerivedTypeKind = 22
	MetaInheritance     MetaDerivedTypeKind = 28
	MetaPtrToMemberType MetaDerivedTypeKind = 31
	MetaConstType       MetaDerivedTypeKind = 38
	MetaFriend          MetaDerivedTypeKind = 42
	MetaVolatileType    MetaDerivedTypeKind = 53
	MetaRestrictType    MetaDerivedTypeKind = 55
	MetaAtomicType      MetaDerivedTypeKind = 71
	MetaImmutableType   MetaDerivedTypeKind = 75
)

type DerivedTypeMeta struct {
	baseMetaNode

	Kind   MetaDerivedTypeKind
	Base   MetaRef
	Offset uint32
	Size   uint32
	Align  uint32
}

type MetaCompositeTypeKind uint8

const (
	MetaArrayType       MetaCompositeTypeKind = 1
	MetaClassType       MetaCompositeTypeKind = 2
	MetaEnumerationType MetaCompositeTypeKind = 4
	MetaStructureType   MetaCompositeTypeKind = 19
	MetaUnionType       MetaCompositeTypeKind = 23
	MetaVariant         MetaCompositeTypeKind = 25
	MetaVariantTart     MetaCompositeTypeKind = 51
)

type CompositeTypeMeta struct {
	baseMetaNode

	Name     string
	Kind     MetaCompositeTypeKind
	BaseType MetaRef
	Elements []MetaRef
	File     MetaRef
	Line     uint32
	Size     uint32
	Align    uint32
}

type SubrangeMeta struct {
	baseMetaNode

	Count uint32
}

type EnumeratorMeta struct {
	baseMetaNode

	Name  string
	Value utils.Integer
}

type GlobalVariableMeta struct {
	baseMetaNode

	Name     string
	LinkName string
	Type     MetaRef
	Scope    MetaRef
	File     MetaRef
	Line     uint32
}

type GlobalVariableExpressionMeta struct {
	baseMetaNode

	Var MetaRef
}

type SubprogramMeta struct {
	baseMetaNode

	Name     string
	LinkName string
	Type     MetaRef
	Scope    MetaRef
	Unit     MetaRef
	File     MetaRef
	Line     uint32
}

type LexicalBlockMeta struct {
	baseMetaNode

	Scope  MetaRef
	File   MetaRef
	Line   uint32
	Column uint32
}

type LocationMeta struct {
	baseMetaNode

	Scope  MetaRef
	Line   uint32
	Column uint32
}

type LocalVariableMeta struct {
	baseMetaNode

	Name  string
	Type  MetaRef
	Arg   uint32
	Scope MetaRef
	File  MetaRef
	Line  uint32
}
