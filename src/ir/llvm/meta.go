package llvm

import (
	"fireball/ir"
	"fireball/utils"
	"path/filepath"
)

func (w *writer) meta(node ir.MetaNode) {
	switch node := node.(type) {
	// Raw meta node

	case *ir.RawMeta:
		w.string("!{")

		for i, value := range node.Values {
			if i > 0 {
				w.string(", ")
			}

			if value.Text != "" {
				w.string("!\"")
				w.string(value.Text)
				w.rune('"')
			} else {
				w.string("i32 ")
				w.uint(uint64(value.Number), 10)
			}
		}

		w.rune('}')
		return

	// Specialized meta nodes

	case *ir.CompileUnitMeta:
		w.beginMeta(true, "DICompileUnit")
		w.fieldRaw("language", "DW_LANG_C99")
		w.fieldMetaRef("file", node.File)
		w.fieldString("producer", node.Producer)
		w.fieldBool("isOptimized", node.IsOptimized)
		w.fieldUint("runtimeVersion", 0)
		w.fieldBool("splitDebugInlining", false)
		w.fieldRaw("nameTableKind", "None")
		w.fieldRaw("emissionKind", "FullDebug")
		w.fieldMetaRef("enums", node.Enums)
		w.fieldMetaRef("retainedTypes", node.RetainedTypes)
		w.fieldMetaRef("globals", node.Globals)
		w.fieldMetaRef("imports", node.Imports)

	case *ir.FileMeta:
		w.beginMeta(false, "DIFile")
		w.fieldString("filename", node.Path)
		w.fieldString("directory", filepath.Dir(node.Path))

	case *ir.BasicTypeMeta:
		encoding := ""

		switch node.Encoding {
		case ir.MetaAddress:
			encoding = "DW_ATE_address"
		case ir.MetaBoolean:
			encoding = "DW_ATE_boolean"
		case ir.MetaFloat:
			encoding = "DW_ATE_float"
		case ir.MetaSigned:
			encoding = "DW_ATE_signed"
		case ir.MetaSignedChar:
			encoding = "DW_ATE_signed_char"
		case ir.MetaUnsigned:
			encoding = "DW_ATE_unsigned"
		case ir.MetaUnsignedChar:
			encoding = "DW_ATE_unsigned_char"
		}

		w.beginMeta(false, "DIBasicType")
		w.fieldString("name", node.Name)
		w.fieldRaw("encoding", encoding)
		w.fieldUint("size", node.Size)
		w.fieldUint("align", node.Align)

	case *ir.SubroutineTypeMeta:
		w.beginMeta(false, "DISubroutineType")
		w.fieldSliceMetaRef("types", append([]ir.MetaRef{node.Returns}, node.Params...))

	case *ir.DerivedTypeMeta:
		tag := ""

		switch node.Kind {
		case ir.MetaMember:
			tag = "DW_TAG_member"
		case ir.MetaPointerType:
			tag = "DW_TAG_pointer_type"
		case ir.MetaReferenceType:
			tag = "DW_TAG_reference_type"
		case ir.MetaTypedef:
			tag = "DW_TAG_typedef"
		case ir.MetaInheritance:
			tag = "DW_TAG_inheritance"
		case ir.MetaPtrToMemberType:
			tag = "DW_TAG_ptr_to_member_type"
		case ir.MetaConstType:
			tag = "DW_TAG_const_Type"
		case ir.MetaFriend:
			tag = "DW_TAG_friend"
		case ir.MetaVolatileType:
			tag = "DW_TAG_volatile_type"
		case ir.MetaRestrictType:
			tag = "DW_TAG_restrict_type"
		case ir.MetaAtomicType:
			tag = "DW_TAG_atomic_type"
		case ir.MetaImmutableType:
			tag = "DW_TAG_immutable_type"
		}

		w.beginMeta(false, "DIDerivedType")
		w.fieldRaw("tag", tag)
		w.fieldMetaRef("baseType", node.Base)
		w.fieldUint("size", node.Size)
		w.fieldUint("align", node.Align)

	case *ir.CompositeTypeMeta:
		tag := ""

		switch node.Kind {
		case ir.MetaArrayType:
			tag = "DW_TAG_array_type"
		case ir.MetaClassType:
			tag = "DW_TAG_class_type"
		case ir.MetaEnumerationType:
			tag = "DW_TAG_enumeration_type"
		case ir.MetaStructureType:
			tag = "DW_TAG_structure_type"
		case ir.MetaUnionType:
			tag = "DW_TAG_union_type"
		case ir.MetaVariant:
			tag = "DW_TAG_variant"
		case ir.MetaVariantTart:
			tag = "DW_TAG_variant_part"
		}

		w.beginMeta(false, "DICompositeType")
		w.fieldString("name", node.Name)
		w.fieldRaw("tag", tag)
		w.fieldMetaRef("baseType", node.BaseType)
		w.fieldSliceMetaRef("elements", node.Elements)
		w.fieldMetaRef("file", node.File)
		w.fieldUint("line", node.Line)
		w.fieldUint("size", node.Size)
		w.fieldUint("align", node.Align)

	case *ir.SubrangeMeta:
		w.beginMeta(false, "DISubrange")
		w.fieldUint("count", node.Count)

	case *ir.EnumeratorMeta:
		w.beginMeta(false, "DIEnumerator")
		w.fieldString("name", node.Name)
		w.fieldInteger("value", node.Value)

	case *ir.GlobalVariableMeta:
		w.beginMeta(false, "DIGlobalVariable")
		w.fieldString("name", node.Name)
		w.fieldString("linkageName", node.LinkName)
		w.fieldMetaRef("type", node.Type)
		w.fieldMetaRef("scope", node.Scope)
		w.fieldMetaRef("file", node.File)
		w.fieldUint("line", node.Line)

	case *ir.GlobalVariableExpressionMeta:
		w.beginMeta(false, "DIGlobalVariableExpression")
		w.fieldMetaRef("var", node.Var)
		w.fieldRaw("expr", "!DIExpression()")

	case *ir.SubprogramMeta:
		w.beginMeta(true, "DISubprogram")
		w.fieldString("name", node.Name)
		w.fieldString("linkageName", node.LinkName)
		w.fieldMetaRef("type", node.Type)
		w.fieldMetaRef("scope", node.Scope)
		w.fieldMetaRef("unit", node.Unit)
		w.fieldMetaRef("file", node.File)
		w.fieldUint("line", node.Line)

	case *ir.LexicalBlockMeta:
		w.beginMeta(true, "DILexicalBlock")
		w.fieldMetaRef("scope", node.Scope)
		w.fieldMetaRef("file", node.File)
		w.fieldUint("line", node.Line)
		w.fieldUint("column", node.Column)

	case *ir.LocationMeta:
		w.beginMeta(false, "DILocation")
		w.fieldMetaRef("scope", node.Scope)
		w.fieldUint("line", node.Line)
		w.fieldUint("column", node.Column)

	case *ir.LocalVariableMeta:
		w.beginMeta(false, "DILocalVariable")
		w.fieldString("name", node.Name)
		w.fieldMetaRef("type", node.Type)
		w.fieldUint("arg", node.Arg)
		w.fieldMetaRef("scope", node.Scope)
		w.fieldMetaRef("file", node.File)
		w.fieldUint("line", node.Line)

	default:
		panic("llvm.writer.meta() - Invalid meta node")
	}

	w.rune(')')
}

// Utils

func (w *writer) beginMeta(distinct bool, name string) {
	if distinct {
		w.string("distinct !")
	} else {
		w.rune('!')
	}

	w.string(name)
	w.rune('(')

	w.metaHasField = false
}

func (w *writer) fieldBool(name string, value bool) {
	if w.metaHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": ")
	w.string(utils.Ternary(value, "true", "false"))

	w.metaHasField = true
}

func (w *writer) fieldUint(name string, value uint32) {
	if w.metaHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": ")
	w.uint(uint64(value), 10)

	w.metaHasField = true
}

func (w *writer) fieldInteger(name string, value utils.Integer) {
	if w.metaHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": ")
	if value.Negative() {
		w.rune('-')
	}
	w.uint(value.Raw(), 10)

	w.metaHasField = true
}

func (w *writer) fieldRaw(name, value string) {
	if w.metaHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": ")
	w.string(value)

	w.metaHasField = true
}

func (w *writer) fieldString(name, value string) {
	if w.metaHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": \"")
	w.string(value)
	w.rune('"')

	w.metaHasField = true
}

func (w *writer) fieldMetaRef(name string, ref ir.MetaRef) {
	if w.metaHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": ")
	w.metaRef(ref)

	w.metaHasField = true
}

func (w *writer) fieldSliceMetaRef(name string, refs []ir.MetaRef) {
	if w.metaHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": !{")

	for i, ref := range refs {
		if i > 0 {
			w.string(", ")
		}
		w.metaRef(ref)
	}

	w.rune('}')

	w.metaHasField = true
}

func (w *writer) metaRef(ref ir.MetaRef) {
	if ref.Valid() {
		w.rune('!')
		w.uint(uint64(ref.Value()), 10)
	} else {
		w.string("null")
	}
}
