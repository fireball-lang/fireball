package ast

func Root(node Node) *File {
	for {
		if !IsValid(node) {
			return nil
		}

		if f, ok := node.(*File); ok {
			return f
		}

		node = node.Parent()
	}
}
