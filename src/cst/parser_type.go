package cst

import (
	"fireball/lexer"
	"fireball/utils"
	"strconv"
)

func (p *parser) typeNode() (Node, bool) {
	switch p.current.Kind {
	case lexer.Identifier:
		if p.current.Text == "fn" {
			return p.funcTypeNode()
		}

		return p.declTypeNode()

	case lexer.LeftBracket:
		return p.arrayTypeNode()

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

func (p *parser) arrayTypeNode() (Node, bool) {
	node := Node{Kind: ArrayType}

	// [
	node.append(p.advance())

	// Count
	if p.appendAdvance(&node, lexer.Number, "Expected array count.") {
		return node, true
	}

	if num, err := strconv.ParseUint(node.Children[len(node.Children)-1].Token.Text, 10, 32); err != nil || num == 0 {
		p.diagnostics = append(p.diagnostics, utils.Diagnostic{
			Kind:    utils.Error,
			Message: "Invalid array count",
			Range:   node.Children[len(node.Children)-1].Range,
		})

		node.Children = node.Children[:len(node.Children)-1]
	}

	// ]
	if p.appendAdvance(&node, lexer.RightBracket, "Expected ']' after array count.") {
		return node, true
	}

	// Element
	{
		child, err := p.typeNode()
		node.append(child)

		if err {
			return node, true
		}
	}

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

func (p *parser) funcTypeNode() (Node, bool) {
	node := Node{Kind: FuncType}

	// fn
	node.append(p.advance())

	// (
	if p.appendAdvance(&node, lexer.LeftParen, "Expected '(' before function parameters.") {
		return node, true
	}

	// Params
	hasParams := false

	for p.current.Kind != lexer.RightParen && p.current.Kind != lexer.DotDotDot {
		if hasParams {
			if p.appendAdvance(&node, lexer.Comma, "Expected ',' between function parameters.") {
				return node, true
			}
		}

		if p.current.Kind == lexer.DotDotDot {
			node.append(p.advance())
			break
		}

		child, err := p.typeNode()
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
	{
		child, err := p.typeNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}
