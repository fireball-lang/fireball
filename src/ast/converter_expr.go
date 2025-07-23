package ast

import (
	"fireball/cst"
	"fireball/lexer"
)

func (c *converter) convertExpr(node *cst.Node) Expr {
	switch node.Kind {
	case cst.Block:
		return c.convertBlock(node)
	case cst.Var:
		return c.convertVar(node)
	case cst.If:
		return c.convertIf(node)
	case cst.While:
		return c.convertWhile(node)
	case cst.For:
		return c.convertFor(node)
	case cst.Break:
		return c.convertBreak(node)
	case cst.Continue:
		return c.convertContinue(node)
	case cst.Return:
		return c.convertReturn(node)

	case cst.Literal:
		return c.convertLiteral(node)
	case cst.StructInitializer:
		return c.convertStructInitializer(node)
	case cst.Paren:
		return c.convertParen(node)
	case cst.Identifier:
		return c.convertIdentifier(node)
	case cst.Call:
		return c.convertCall(node)
	case cst.TypeCall:
		return c.convertTypeCall(node)
	case cst.Index:
		return c.convertIndex(node)
	case cst.Member:
		return c.convertMember(node)
	case cst.Unary:
		return c.convertUnary(node)
	case cst.Binary:
		return c.convertBinary(node)
	case cst.Is:
		return c.convertIs(node)
	case cst.Cast:
		return c.convertCast(node)

	default:
		panic("ast.convertExpr() - Invalid node kind")
	}
}

func (c *converter) convertBlock(node *cst.Node) *Block {
	b := &Block{}
	b.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			b.Exprs = append(b.Exprs, c.convertExpr(child))
		}
	}

	return b
}

func (c *converter) convertVar(node *cst.Node) *Var {
	v := &Var{}
	v.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			v.Name = c.convertLeaf(child)
		} else if child.Kind.IsType() {
			v.Type = c.convertType(child)
		} else if child.Kind.IsExpr() {
			v.Value = c.convertExpr(child)
		}
	}

	return v
}

func (c *converter) convertIf(node *cst.Node) *If {
	i := &If{}
	i.range_ = node.Range

	for i2 := range node.Children {
		child := &node.Children[i2]

		if child.Kind.IsExpr() {
			if !IsValid(i.Condition) {
				i.Condition = c.convertExpr(child)
			} else if !IsValid(i.Then) {
				i.Then = c.convertExpr(child)
			} else {
				i.Else = c.convertExpr(child)
			}
		}
	}

	return i
}

func (c *converter) convertWhile(node *cst.Node) *While {
	prevLoopKind := c.loopKind
	c.loopKind = whileLoop

	w := &While{}
	w.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			if !IsValid(w.Condition) {
				w.Condition = c.convertExpr(child)
			} else {
				w.Body = c.convertExpr(child)
			}
		}
	}

	c.loopKind = prevLoopKind

	return w
}

func (c *converter) convertFor(node *cst.Node) *Block {
	prevLoopKind := c.loopKind
	c.loopKind = forLoop

	prevForIncrement := c.forIncrement

	var initializer Expr
	var condition Expr
	var increment Expr
	var body Expr

	delimiterCount := 0

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			if delimiterCount == 0 {
				if !IsValid(initializer) {
					initializer = c.convertExpr(child)
				} else if !IsValid(condition) {
					condition = c.convertExpr(child)
				} else if !IsValid(increment) {
					c.forIncrement = child
					increment = c.convertExpr(child)
				} else {
					body = c.convertExpr(child)
				}
			} else if delimiterCount == 1 {
				if !IsValid(condition) {
					condition = c.convertExpr(child)
				} else if !IsValid(increment) {
					c.forIncrement = child
					increment = c.convertExpr(child)
				} else {
					body = c.convertExpr(child)
				}
			} else if delimiterCount == 2 {
				if !IsValid(increment) {
					c.forIncrement = child
					increment = c.convertExpr(child)
				} else {
					body = c.convertExpr(child)
				}
			} else {
				body = c.convertExpr(child)
			}
		} else if child.Kind == cst.Leaf && (child.Token.Kind == lexer.Semicolon || child.Token.Kind == lexer.RightParen) {
			delimiterCount++
		}
	}

	if !IsValid(condition) {
		condition = &Literal{Value: &Leaf{Token: lexer.Token{
			Kind: lexer.Identifier,
			Text: "true",
		}}}
	}

	b := &Block{}
	b.range_ = node.Range

	if IsValid(initializer) {
		b.Exprs = append(b.Exprs, initializer)
	}

	w := &While{}
	b.Exprs = append(b.Exprs, w)

	w.Condition = condition
	w.Body = body

	if IsValid(increment) {
		b := &Block{}

		if IsValid(w.Body) {
			b.Exprs = append(b.Exprs, w.Body)
		}

		b.Exprs = append(b.Exprs, increment)

		w.Body = b
	}

	c.loopKind = prevLoopKind
	c.forIncrement = prevForIncrement

	return b
}

func (c *converter) convertBreak(node *cst.Node) *Break {
	b := &Break{}
	b.range_ = node.Range

	return b
}

func (c *converter) convertContinue(node *cst.Node) Expr {
	co := &Continue{}
	co.range_ = node.Range

	if c.loopKind == forLoop {
		b := &Block{}

		b.Exprs = append(b.Exprs, c.convertExpr(c.forIncrement))
		b.Exprs = append(b.Exprs, co)

		return b
	}

	return co
}

func (c *converter) convertReturn(node *cst.Node) *Return {
	r := &Return{}
	r.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			r.Value = c.convertExpr(child)
		}
	}

	return r
}

func (c *converter) convertLiteral(node *cst.Node) *Literal {
	return &Literal{
		Value: c.convertLeaf(&node.Children[0]),
	}
}

func (c *converter) convertStructInitializer(node *cst.Node) *StructInitializer {
	s := &StructInitializer{}
	s.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			s.Name = c.convertLeaf(child)
		} else if child.Kind == cst.StructInitializerField {
			s.Fields = append(s.Fields, c.convertStructInitializerField(child))
		}
	}

	return s
}

func (c *converter) convertStructInitializerField(node *cst.Node) *StructInitializerField {
	s := &StructInitializerField{}
	s.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			s.Name = c.convertLeaf(child)
		} else if child.Kind.IsExpr() {
			s.Value = c.convertExpr(child)
		}
	}

	return s
}

func (c *converter) convertParen(node *cst.Node) *Paren {
	p := &Paren{}
	p.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			p.Expr = c.convertExpr(child)
		}
	}

	return p
}

func (c *converter) convertIdentifier(node *cst.Node) *Identifier {
	i := &Identifier{}

	for index := range node.Children {
		child := &node.Children[index]

		if child.Kind == cst.Path {
			i.Path = c.convertPath(child)
		}
	}

	return i
}

func (c *converter) convertCall(node *cst.Node) *Call {
	call := &Call{}
	call.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			if !IsValid(call.Callee) {
				call.Callee = c.convertExpr(child)
			} else {
				call.Args = append(call.Args, c.convertExpr(child))
			}
		}
	}

	return call
}

func (c *converter) convertTypeCall(node *cst.Node) *TypeCall {
	call := &TypeCall{}
	call.range_ = node.Range

	call.Kind = Sizeof

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			if child.Token.Text == "alignof" {
				call.Kind = Alignof
			}
		} else if child.Kind.IsType() {
			call.Arg = c.convertType(child)
		}
	}

	return call
}

func (c *converter) convertIndex(node *cst.Node) *Index {
	i := &Index{}
	i.range_ = node.Range

	for i2 := range node.Children {
		child := &node.Children[i2]

		if child.Kind.IsExpr() {
			if !IsValid(i.Value) {
				i.Value = c.convertExpr(child)
			} else {
				i.Index = c.convertExpr(child)
			}
		}
	}

	return i
}

func (c *converter) convertMember(node *cst.Node) *Member {
	m := &Member{}
	m.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			m.Value = c.convertExpr(child)
		} else if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			m.Name = c.convertLeaf(child)
		}
	}

	return m
}

func (c *converter) convertUnary(node *cst.Node) *Unary {
	u := &Unary{}
	u.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			u.Expr = c.convertExpr(child)
		} else if child.Kind == cst.Leaf && child.Token.Kind.IsOperator() {
			u.Op = child.Token.Kind
			u.Postfix = IsValid(u.Expr)
		}
	}

	return u
}

func (c *converter) convertBinary(node *cst.Node) *Binary {
	b := &Binary{}
	b.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			if !IsValid(b.Left) {
				b.Left = c.convertExpr(child)
			} else {
				b.Right = c.convertExpr(child)
			}
		} else if child.Kind == cst.Leaf && child.Token.Kind.IsOperator() {
			b.Op = child.Token.Kind
		}
	}

	return b
}

func (c *converter) convertIs(node *cst.Node) *Is {
	is := &Is{}
	is.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			is.Value = c.convertExpr(child)
		} else if child.Kind.IsType() {
			is.Type = c.convertType(child)
		}
	}

	return is
}

func (c *converter) convertCast(node *cst.Node) *Cast {
	cast := &Cast{}
	cast.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			cast.Value = c.convertExpr(child)
		} else if child.Kind.IsType() {
			cast.Type = c.convertType(child)
		}
	}

	return cast
}
