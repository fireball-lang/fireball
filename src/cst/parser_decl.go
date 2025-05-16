package cst

import "fireball/lexer"

func (p *parser) funcNode() (Node, bool) {
	node := Node{Kind: Func}

	// Keyword
	node.append(p.advance())

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected function name.") {
		return node, true
	}

	// (
	if p.appendAdvance(&node, lexer.LeftParen, "Expected '(' before function parameters.") {
		return node, true
	}

	// Params
	hasParams := false

	for p.current.Kind != lexer.RightParen {
		if hasParams {
			if p.appendAdvance(&node, lexer.Colon, "Expected ',' between function parameters.") {
				return node, true
			}
		}

		child, err := p.paramNode()
		node.append(child)

		if err {
			return node, true
		}

		hasParams = true
	}

	// )
	if p.appendAdvance(&node, lexer.RightParen, "Expected ')' after function parameters.") {
		return node, true
	}

	// Body
	if p.current.Kind != lexer.LeftBrace {
		p.error("Expected '{' before function body.")
		return node, true
	} else {
		child, err := p.blockNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}

func (p *parser) paramNode() (Node, bool) {
	node := Node{Kind: Param}

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected param name.") {
		return node, true
	}

	// Type
	child, err := p.typeNode()
	node.append(child)

	return node, err
}
