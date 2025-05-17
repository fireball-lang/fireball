package cst

import "fireball/lexer"

func (p *parser) typeNode() (Node, bool) {
	switch p.current.Kind {
	case lexer.Identifier:
		return p.declTypeNode()
	case lexer.Star:
		return p.pointerTypeNode()

	default:
		p.error("Expected a type name.")
		return Node{}, true
	}
}

func (p *parser) declTypeNode() (Node, bool) {
	node := Node{Kind: DeclType}

	// Type name
	node.append(p.advance())

	return node, false
}

func (p *parser) pointerTypeNode() (Node, bool) {
	node := Node{Kind: PointerType}

	// *
	node.append(p.advance())

	// Type
	{
		child, err := p.typeNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}
