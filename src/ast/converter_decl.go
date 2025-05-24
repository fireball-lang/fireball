package ast

import (
	"fireball/cst"
	"fireball/lexer"
)

func convertDecl(node *cst.Node) Decl {
	switch node.Kind {
	case cst.Struct:
		return convertStruct(node)
	case cst.Func:
		return convertFunc(node)

	default:
		panic("ast.convertDecl() - Invalid node kind")
	}
}

func convertAttributes(node *cst.Node) []*Attribute {
	var attributes []*Attribute

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Attribute {
			attributes = append(attributes, convertAttribute(child))
		}
	}

	return attributes
}

func convertAttribute(node *cst.Node) *Attribute {
	a := &Attribute{}
	a.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			if child.Token.Kind == lexer.Identifier {
				if !IsValid(a.Name) {
					a.Name = convertLeaf(child)
				} else {
					a.Param = child.Token.Text
				}
			} else if child.Token.Kind == lexer.String {
				a.Param = child.Token.Text[1 : len(child.Token.Text)-1]
			}
		}
	}

	return a
}

func convertStruct(node *cst.Node) *Struct {
	s := &Struct{}
	s.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		//goland:noinspection GoSwitchMissingCasesForIotaConsts
		switch child.Kind {
		case cst.Attributes:
			s.Attributes = convertAttributes(child)
		case cst.Leaf:
			if child.Token.Kind == lexer.Identifier {
				s.NameN = convertLeaf(child)
			}
		case cst.Field:
			s.Fields = append(s.Fields, convertField(child))
		}
	}

	return s
}

func convertField(node *cst.Node) *Field {
	f := &Field{}
	f.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			f.Name = convertLeaf(child)
		} else if child.Kind.IsType() {
			f.Type = convertType(child)
		}
	}

	return f
}

func convertFunc(node *cst.Node) *Func {
	f := &Func{}
	f.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		switch child.Kind {
		case cst.Attributes:
			f.Attributes = convertAttributes(child)
		case cst.Leaf:
			if child.Token.Kind == lexer.Identifier {
				f.NameN = convertLeaf(child)
			} else if child.Token.Kind == lexer.DotDotDot {
				f.varArgs = true
			}
		case cst.Param:
			f.Params = append(f.Params, convertParam(child))
		case cst.Block:
			f.Body = convertBlock(child)
		default:
			if child.Kind.IsType() {
				f.Returns = convertType(child)
			}
		}
	}

	return f
}

func convertParam(node *cst.Node) *Param {
	p := &Param{}
	p.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			p.Name = convertLeaf(child)
		} else if child.Kind.IsType() {
			p.Type = convertType(child)
		}
	}

	return p
}
