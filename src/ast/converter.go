package ast

import (
	"fireball/cst"
)

func Convert(node *cst.Node) *File {
	if node.Kind != cst.File {
		panic("ast.Convert() - Can only convert File nodes")
	}

	f := &File{}
	f.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsDecl() {
			f.Decls = append(f.Decls, convertDecl(child))
		}
	}

	setParents(f)

	return f
}

func setParents(node Node) {
	for child := range node.Children() {
		child.setParent(node)
		setParents(child)
	}
}

func convertLeaf(node *cst.Node) *Leaf {
	return &Leaf{
		Token: node.Token,
	}
}
