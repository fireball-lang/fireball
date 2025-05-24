package cst

import "fireball/lexer"

func (p *parser) declNode(invalidKeyword *bool) (Node, bool) {
	switch p.current.Text {
	case "struct":
		return p.structNode()
	case "func":
		return p.funcNode()

	default:
		*invalidKeyword = true
		return Node{}, true
	}
}

func (p *parser) structNode() (Node, bool) {
	node := Node{Kind: Struct}

	// Keyword
	node.append(p.advance())

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected struct name.") {
		return node, true
	}

	// {
	if p.appendAdvance(&node, lexer.LeftBrace, "Expected '{' before struct fields.") {
		return node, true
	}

	// Fields
	for p.current.Kind != lexer.RightBrace {
		child, err := p.fieldNode()
		node.append(child)

		if err {
			return node, true
		}

		if p.appendAdvance(&node, lexer.Semicolon, "Expected ';' after struct field.") {
			return node, true
		}
	}

	// }
	if p.appendAdvance(&node, lexer.RightBrace, "Expected '}' after struct fields.") {
		return node, true
	}

	return node, false
}

func (p *parser) fieldNode() (Node, bool) {
	node := Node{Kind: Field}

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected field name.") {
		return node, true
	}

	// Type
	child, err := p.typeNode()
	node.append(child)

	return node, err
}

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
			if p.appendAdvance(&node, lexer.Comma, "Expected ',' between function parameters.") {
				return node, true
			}
		}

		if p.current.Kind == lexer.DotDotDot {
			node.append(p.advance())
			break
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

	// Return type
	if p.current.Kind != lexer.LeftBrace {
		child, err := p.typeNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	// Body
	if p.current.Kind == lexer.LeftBrace {
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
