package ast

import "fireball/core"

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
