package ast

import (
	"fireball/cst"
	"fireball/lexer"
)

func (c *converter) convertDecl(node *cst.Node) Decl {
	switch node.Kind {
	case cst.Mod:
		return c.convertMod(node)
	case cst.Import:
		return c.convertImport(node)
	case cst.Struct:
		return c.convertStruct(node)
	case cst.Enum:
		return c.convertEnum(node)
	case cst.Interface:
		return c.convertInterface(node)
	case cst.Impl:
		return c.convertImpl(node)
	case cst.GlobalVar:
		return c.convertGlobalVar(node)
	case cst.Func:
		method, _ := c.convertFunc(node)
		return method

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

func (c *converter) convertMod(node *cst.Node) *Mod {
	m := &Mod{}
	m.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Path {
			m.Path = c.convertPath(child)
		}
	}

	return m
}

func (c *converter) convertImport(node *cst.Node) *Import {
	i := &Import{}
	i.range_ = node.Range

	for index := range node.Children {
		child := &node.Children[index]

		//goland:noinspection GoSwitchMissingCasesForIotaConsts
		switch child.Kind {
		case cst.Attributes:
			i.Attributes = c.convertAttributes(child)
		case cst.Path:
			i.Path = c.convertPath(child)
		case cst.Leaf:
			if child.Token.Kind == lexer.Star || (child.Token.Kind == lexer.Identifier && child.Token.Text != "import") {
				i.Symbols = append(i.Symbols, c.convertLeaf(child))
			}
		}
	}

	return i
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

		if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			f.Name = c.convertLeaf(child)
		} else if child.Kind.IsType() {
			f.Type = c.convertType(child)
		}
	}

	return f
}

func (c *converter) convertEnum(node *cst.Node) *Enum {
	e := &Enum{}
	e.range_ = node.Range

	for index := range node.Children {
		child := &node.Children[index]

		if child.Kind == cst.Attributes {
			e.Attributes = c.convertAttributes(child)
		} else if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			e.NameN = c.convertLeaf(child)
		} else if child.Kind.IsType() {
			e.Type = c.convertType(child)
		} else if child.Kind == cst.EnumCase {
			e.Cases = append(e.Cases, c.convertEnumCase(child))
		}
	}

	return e
}

func (c *converter) convertEnumCase(node *cst.Node) *EnumCase {
	e := &EnumCase{}
	e.range_ = node.Range

	for index := range node.Children {
		child := &node.Children[index]

		if child.Kind == cst.Leaf {
			if child.Token.Kind == lexer.Identifier {
				e.Name = c.convertLeaf(child)
			} else if child.Token.Kind == lexer.Integer || child.Token.Kind == lexer.Hexadecimal || child.Token.Kind == lexer.Binary {
				e.Value = c.convertLeaf(child)
			}
		}
	}

	return e
}

func (c *converter) convertInterface(node *cst.Node) *Interface {
	i := &Interface{}
	i.range_ = node.Range

	for index := range node.Children {
		child := &node.Children[index]

		if child.Kind == cst.Attributes {
			i.Attributes = c.convertAttributes(child)
		} else if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			i.NameN = c.convertLeaf(child)
		} else if child.Kind == cst.Func {
			method, _ := c.convertFunc(child)
			i.Methods = append(i.Methods, method)
		}
	}

	return i
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
			// TODO
			if child.Token.Kind == lexer.Identifier && child.Token.Text != "impl" && child.Token.Text != "for" {
				if i.DeclName == nil {
					i.DeclName = c.convertLeaf(child)
				} else {
					i.InterfaceName = i.DeclName
					i.DeclName = c.convertLeaf(child)
				}
			}
		case cst.Func:
			method, static := c.convertFunc(child)

			if static {
				i.StaticMethods = append(i.StaticMethods, method)
			} else {
				i.Methods = append(i.Methods, method)
			}
		}
	}

	return i
}

func (c *converter) convertGlobalVar(node *cst.Node) *GlobalVar {
	g := &GlobalVar{}
	g.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Attributes {
			g.Attributes = c.convertAttributes(child)
		} else if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			g.NameN = c.convertLeaf(child)
		} else if child.Kind.IsType() {
			g.Type = c.convertType(child)
		}
	}

	return g
}

func (c *converter) convertFunc(node *cst.Node) (*Func, bool) {
	f := &Func{}
	f.range_ = node.Range

	static := false

	for i := range node.Children {
		child := &node.Children[i]

		switch child.Kind {
		case cst.Attributes:
			f.Attributes = c.convertAttributes(child)
		case cst.Leaf:
			if child.Token.Kind == lexer.Identifier {
				if child.Token.Text == "static" && !static {
					static = true
				} else {
					f.NameN = c.convertLeaf(child)
				}
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

	return f, static
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
