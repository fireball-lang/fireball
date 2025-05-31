package ast

import (
	"fireball/cst"
	"fireball/lexer"
)

func convertExpr(node *cst.Node) Expr {
	switch node.Kind {
	case cst.Block:
		return convertBlock(node)
	case cst.Var:
		return convertVar(node)
	case cst.If:
		return convertIf(node)
	case cst.While:
		return convertWhile(node)
	case cst.For:
		return convertFor(node)
	case cst.Return:
		return convertReturn(node)

	case cst.Literal:
		return convertLiteral(node)
	case cst.Paren:
		return convertParen(node)
	case cst.Identifier:
		return convertIdentifier(node)
	case cst.Call:
		return convertCall(node)
	case cst.Index:
		return convertIndex(node)
	case cst.Member:
		return convertMember(node)
	case cst.Unary:
		return convertUnary(node)
	case cst.Binary:
		return convertBinary(node)
	case cst.Cast:
		return convertCast(node)

	default:
		panic("ast.convertExpr() - Invalid node kind")
	}
}

func convertBlock(node *cst.Node) *Block {
	b := &Block{}
	b.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			b.Exprs = append(b.Exprs, convertExpr(child))
		}
	}

	return b
}

func convertVar(node *cst.Node) *Var {
	v := &Var{}
	v.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			v.Name = convertLeaf(child)
		} else if child.Kind.IsType() {
			v.Type = convertType(child)
		} else if child.Kind.IsExpr() {
			v.Value = convertExpr(child)
		}
	}

	return v
}

func convertIf(node *cst.Node) *If {
	i := &If{}
	i.range_ = node.Range

	for i2 := range node.Children {
		child := &node.Children[i2]

		if child.Kind.IsExpr() {
			if !IsValid(i.Condition) {
				i.Condition = convertExpr(child)
			} else if !IsValid(i.Then) {
				i.Then = convertExpr(child)
			} else {
				i.Else = convertExpr(child)
			}
		}
	}

	return i
}

func convertWhile(node *cst.Node) *While {
	w := &While{}
	w.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			if !IsValid(w.Condition) {
				w.Condition = convertExpr(child)
			} else {
				w.Body = convertExpr(child)
			}
		}
	}

	return w
}

func convertFor(node *cst.Node) *Block {
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
					initializer = convertExpr(child)
				} else if !IsValid(condition) {
					condition = convertExpr(child)
				} else if !IsValid(increment) {
					increment = convertExpr(child)
				} else {
					body = convertExpr(child)
				}
			} else if delimiterCount == 1 {
				if !IsValid(condition) {
					condition = convertExpr(child)
				} else if !IsValid(increment) {
					increment = convertExpr(child)
				} else {
					body = convertExpr(child)
				}
			} else if delimiterCount == 2 {
				if !IsValid(increment) {
					increment = convertExpr(child)
				} else {
					body = convertExpr(child)
				}
			} else {
				body = convertExpr(child)
			}
		} else if child.Kind == cst.Leaf && (child.Token.Kind == lexer.Semicolon || child.Token.Kind == lexer.RightParen) {
			delimiterCount++
		}
	}

	b := &Block{}
	b.range_ = node.Range

	if IsValid(initializer) {
		b.Exprs = append(b.Exprs, initializer)
	}

	if IsValid(condition) || IsValid(body) {
		w := &While{}
		b.Exprs = append(b.Exprs, w)

		w.Condition = condition
		w.Body = body

		if IsValid(increment) {
			b := &Block{}

			b.Exprs = append(b.Exprs, w.Body)
			b.Exprs = append(b.Exprs, increment)

			w.Body = b
		}
	}

	return b
}

func convertReturn(node *cst.Node) *Return {
	r := &Return{}
	r.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			r.Value = convertExpr(child)
		}
	}

	return r
}

func convertLiteral(node *cst.Node) *Literal {
	return &Literal{
		Value: convertLeaf(&node.Children[0]),
	}
}

func convertParen(node *cst.Node) *Paren {
	p := &Paren{}
	p.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			p.Expr = convertExpr(child)
		}
	}

	return p
}

func convertIdentifier(node *cst.Node) *Identifier {
	return &Identifier{
		Name: convertLeaf(&node.Children[0]),
	}
}

func convertCall(node *cst.Node) *Call {
	c := &Call{}
	c.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			if !IsValid(c.Callee) {
				c.Callee = convertExpr(child)
			} else {
				c.Args = append(c.Args, convertExpr(child))
			}
		}
	}

	return c
}

func convertIndex(node *cst.Node) *Index {
	i := &Index{}
	i.range_ = node.Range

	for i2 := range node.Children {
		child := &node.Children[i2]

		if child.Kind.IsExpr() {
			if !IsValid(i.Value) {
				i.Value = convertExpr(child)
			} else {
				i.Index = convertExpr(child)
			}
		}
	}

	return i
}

func convertMember(node *cst.Node) *Member {
	m := &Member{}
	m.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			m.Value = convertExpr(child)
		} else if child.Kind == cst.Leaf && child.Token.Kind == lexer.Identifier {
			m.Name = convertLeaf(child)
		}
	}

	return m
}

func convertUnary(node *cst.Node) *Unary {
	u := &Unary{}
	u.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			u.Expr = convertExpr(child)
		} else if child.Kind == cst.Leaf && child.Token.Kind.IsOperator() {
			u.Op = child.Token.Kind
			u.Postfix = IsValid(u.Expr)
		}
	}

	return u
}

func convertBinary(node *cst.Node) *Binary {
	b := &Binary{}
	b.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			if !IsValid(b.Left) {
				b.Left = convertExpr(child)
			} else {
				b.Right = convertExpr(child)
			}
		} else if child.Kind == cst.Leaf && child.Token.Kind.IsOperator() {
			b.Op = child.Token.Kind
		}
	}

	return b
}

func convertCast(node *cst.Node) *Cast {
	c := &Cast{}
	c.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsExpr() {
			c.Value = convertExpr(child)
		} else if child.Kind.IsType() {
			c.Type = convertType(child)
		}
	}

	return c
}
