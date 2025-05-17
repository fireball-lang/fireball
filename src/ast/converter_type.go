package ast

import "fireball/cst"

func convertType(node *cst.Node) Type {
	switch node.Kind {
	case cst.DeclType:
		return convertDeclType(node)

	default:
		panic("ast.convertType() - Invalid node kind")
	}
}

func convertDeclType(node *cst.Node) Type {
	d := &DeclType{}

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind == cst.Leaf {
			d.Name = convertLeaf(child)
		}
	}

	return d
}
