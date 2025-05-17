package ast

import (
	"fireball/cst"
	"fireball/lexer"
)

func convertDecl(node *cst.Node) Decl {
	switch node.Kind {
	case cst.Func:
		return convertFunc(node)

	default:
		panic("ast.convertDecl() - Invalid node kind")
	}
}

func convertFunc(node *cst.Node) *Func {
	f := &Func{}
	f.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		switch child.Kind {
		case cst.Leaf:
			if child.Token.Kind == lexer.Identifier {
				f.NameN = convertLeaf(child)
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
