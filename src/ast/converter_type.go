package ast

import (
	"fireball/cst"
	"fireball/lexer"
	"strconv"
)

func convertType(node *cst.Node) Type {
	switch node.Kind {
	case cst.DeclType:
		return convertDeclType(node)
	case cst.ArrayType:
		return convertArrayType(node)
	case cst.PointerType:
		return convertPointerType(node)
	case cst.FuncType:
		return convertFuncType(node)

	default:
		panic("ast.convertType() - Invalid node kind")
	}
}

func convertDeclType(node *cst.Node) Type {
	// PrimitiveType
	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			if kind, ok := getPrimitiveKind(child.Token.Text); ok {
				return &PrimitiveType{
					baseRangeNode: baseRangeNode{range_: node.Range},
					Kind:          kind,
				}
			}
		}
	}

	// DeclType
	d := &DeclType{}

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			d.Name = convertLeaf(child)
		}
	}

	return d
}

func convertArrayType(node *cst.Node) Type {
	a := &ArrayType{}
	a.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf && child.Token.Kind == lexer.Integer {
			count, _ := strconv.ParseUint(child.Token.Text, 10, 32)
			a.Count = uint32(count)
		} else if child.Kind.IsType() {
			a.Element = convertType(child)
		}
	}

	return a
}

func convertPointerType(node *cst.Node) Type {
	p := &PointerType{}
	p.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsType() {
			p.Pointee = convertType(child)
		}
	}

	return p
}

func convertFuncType(node *cst.Node) Type {
	f := &SimpleFuncType{}
	f.range_ = node.Range

	var lastType Type

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsType() {
			if IsValid(lastType) {
				f.params = append(f.params, lastType)
			}

			lastType = convertType(child)
		} else if child.Kind == cst.Leaf && child.Token.Kind == lexer.DotDotDot {
			f.varArgs = true
		}
	}

	if IsValid(lastType) {
		f.returns = lastType
	}

	return f
}

func getPrimitiveKind(text string) (PrimitiveKind, bool) {
	switch text {
	case "void":
		return Void, true
	case "bool":
		return Bool, true

	case "u8":
		return U8, true
	case "u16":
		return U16, true
	case "u32":
		return U32, true
	case "u64":
		return U64, true

	case "i8":
		return I8, true
	case "i16":
		return I16, true
	case "i32":
		return I32, true
	case "i64":
		return I64, true

	case "f32":
		return F32, true
	case "f64":
		return F64, true

	default:
		return Void, false
	}
}
