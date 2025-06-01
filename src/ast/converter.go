package ast

import (
	"fireball/cst"
)

type loopKind uint8

const (
	none loopKind = iota
	whileLoop
	forLoop
)

type converter struct {
	loopKind     loopKind
	forIncrement *cst.Node
}

func Convert(node *cst.Node) *File {
	if node.Kind != cst.File {
		panic("ast.Convert() - Can only convert File nodes")
	}

	c := converter{}

	f := &File{}
	f.range_ = node.Range

	for i := range node.Children {
		child := &node.Children[i]

		if child.Kind.IsDecl() {
			f.Decls = append(f.Decls, c.convertDecl(child))
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

func (c *converter) convertLeaf(node *cst.Node) *Leaf {
	return &Leaf{
		Token: node.Token,
	}
}
