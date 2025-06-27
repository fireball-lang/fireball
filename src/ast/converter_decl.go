package ast

import (
	"fireball/cst"
	"fireball/lexer"
)

func (c *converter) convertDecl(node *cst.Node) Decl {
	switch node.Kind {
	case cst.Struct:
		return c.convertStruct(node)
	case cst.Impl:
		return c.convertImpl(node)
	case cst.Func:
		return c.convertFunc(node)

	default:
		panic("ast.convertDecl() - Invalid node kind")
	}
}

func (c *converter) convertAttributes(node *cst.Node) []*Attribute {
	var attributes []*Attribute

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Attribute {
			attributes = append(attributes, c.convertAttribute(child))
		}
	}

	return attributes
}

func (c *converter) convertAttribute(node *cst.Node) *Attribute {
	a := &Attribute{}
	a.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			if child.Token.Kind == lexer.Identifier {
				if !IsValid(a.Name) {
					a.Name = c.convertLeaf(child)
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

func (c *converter) convertStruct(node *cst.Node) *Struct {
	s := &Struct{}
	s.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		//goland:noinspection GoSwitchMissingCasesForIotaConsts
		switch child.Kind {
		case cst.Attributes:
			s.Attributes = c.convertAttributes(child)
		case cst.Leaf:
			if child.Token.Kind == lexer.Identifier {
				s.NameN = c.convertLeaf(child)
			}
		case cst.Field:
			s.Fields = append(s.Fields, c.convertField(child))
		}
	}

	return s
}

func (c *converter) convertField(node *cst.Node) *Field {
	f := &Field{}
	f.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			f.Name = c.convertLeaf(child)
		} else if child.Kind.IsType() {
			f.Type = c.convertType(child)
		}
	}

	return f
}

func (c *converter) convertImpl(node *cst.Node) *Impl {
	i := &Impl{}
	i.range_ = node.Range

	for index := range node.Children {
		child := &node.Children[index]

		//goland:noinspection GoSwitchMissingCasesForIotaConsts
		switch child.Kind {
		case cst.Attributes:
			i.Attributes = c.convertAttributes(child)
		case cst.Leaf:
			if child.Token.Kind == lexer.Identifier {
				i.NameN = c.convertLeaf(child)
			}
		case cst.Func:
			i.Methods = append(i.Methods, c.convertFunc(child))
		}
	}

	return i
}

func (c *converter) convertFunc(node *cst.Node) *Func {
	f := &Func{}
	f.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		switch child.Kind {
		case cst.Attributes:
			f.Attributes = c.convertAttributes(child)
		case cst.Leaf:
			if child.Token.Kind == lexer.Identifier {
				f.NameN = c.convertLeaf(child)
			} else if child.Token.Kind == lexer.DotDotDot {
				f.varArgs = true
			}
		case cst.Param:
			f.Params = append(f.Params, c.convertParam(child))
		case cst.Block:
			f.Body = c.convertBlock(child)
		default:
			if child.Kind.IsType() {
				f.Returns = c.convertType(child)
			}
		}
	}

	return f
}

func (c *converter) convertParam(node *cst.Node) *Param {
	p := &Param{}
	p.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			p.Name = c.convertLeaf(child)
		} else if child.Kind.IsType() {
			p.Type = c.convertType(child)
		}
	}

	return p
}
