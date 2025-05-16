package cst

import "fireball/lexer"

func (p *parser) typeNode() (Node, bool) {
	if p.current.Kind == lexer.Identifier {
		return p.declTypeNode()
	}

	p.error("Expected a type name.")
	return Node{}, true
}

func (p *parser) declTypeNode() (Node, bool) {
	node := Node{Kind: DeclType}

	// Type name
	node.append(p.advance())

	return node, false
}
