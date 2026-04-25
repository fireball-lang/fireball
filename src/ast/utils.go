package ast

import "fireball/core"

func GetNodeAtPos(node Node, pos core.Pos) Node {
	if !node.Range().Contains(pos) {
		return nil
	}

outer:
	for {
		for child := range node.Children() {
			if child.Range().Contains(pos) {
				node = child
				continue outer
			}
		}

		return node
	}
}

func GetFile(node Node) *File {
	return GetClosestParent[*File](node)
}

func GetClosestParent[T Node](node Node) T {
	for {
		if n, ok := node.(T); ok {
			return n
		}

		parent := node.Parent()
		if core.IsNil(parent) {
			break
		}

		node = parent
	}

	var empty T
	return empty
}
