package cst

import "fireball/lexer"

func (p *parser) declNode(invalidKeyword *bool) (Node, bool) {
	var attributes Node

	if p.current.Kind == lexer.Hashtag {
		attrs, err := p.attributesNode()
		if err {
			return attributes, true
		}

		attributes = attrs
	}

	switch p.current.Text {
	case "mod":
		return p.modNode(attributes)
	case "import":
		return p.importNode(attributes)
	case "struct":
		return p.structNode(attributes)
	case "enum":
		return p.enumNode(attributes)
	case "impl":
		return p.implNode(attributes)
	case "var":
		return p.globalVarNode(attributes)
	case "func":
		return p.funcNode(attributes)

	default:
		*invalidKeyword = true
		return Node{}, true
	}
}

func (p *parser) attributesNode() (Node, bool) {
	node := Node{Kind: Attributes}

	// #
	node.append(p.advance())

	// [
	if p.appendAdvance(&node, lexer.LeftBracket, "Expected '[' before attributes.") {
		return node, true
	}

	// <attributes>
	hasAttribute := false

	for p.current.Kind != lexer.RightBracket {
		if hasAttribute {
			if p.appendAdvance(&node, lexer.Comma, "Expected ',' between attributes.") {
				return node, true
			}
		}

		child, err := p.attributeNode()
		node.append(child)

		if err {
			return node, true
		}

		hasAttribute = true
	}

	// ]
	if p.appendAdvance(&node, lexer.RightBracket, "Expected ']' after attributes.") {
		return node, true
	}

	return node, false
}

func (p *parser) attributeNode() (Node, bool) {
	node := Node{Kind: Attribute}

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected attribute name.") {
		return node, true
	}

	// Parameter
	if p.current.Kind == lexer.LeftParen {
		// (
		node.append(p.advance())

		// param
		switch p.current.Kind {
		case lexer.Identifier, lexer.String:
			node.append(p.advance())

		default:
			p.error("Expected attribute parameter.")
			return node, true
		}

		// )
		if p.appendAdvance(&node, lexer.RightParen, "Expected ')' after attribute parameter.") {
			return node, true
		}
	}

	return node, false
}

func (p *parser) modNode(attributes Node) (Node, bool) {
	node := Node{Kind: Mod}
	node.append(attributes)

	// Keyword
	node.append(p.advance())

	// Path
	{
		child, err := p.pathNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	// ;
	if p.appendAdvance(&node, lexer.Semicolon, "Expected ';' after path segments.") {
		return node, true
	}

	return node, false
}

func (p *parser) importNode(attributes Node) (Node, bool) {
	node := Node{Kind: Import}
	node.append(attributes)

	// Keyword
	node.append(p.advance())

	// Path
	hasSymbols := false

	{
		child, err, colon := p.importPathNode()
		node.append(child)
		node.append(colon)

		if colon.Kind == Leaf && colon.Token.Kind == lexer.Colon {
			hasSymbols = true
		}

		if err {
			return node, true
		}
	}

	// Symbols
	if hasSymbols {
		if p.appendAdvance(&node, lexer.LeftBrace, "Expected '{' before symbols to import.") {
			return node, true
		}

		hasSymbol := false

		for p.current.Kind != lexer.RightBrace {
			if hasSymbol {
				if p.appendAdvance(&node, lexer.Comma, "Expected ',' before symbols.") {
					return node, true
				}
			}

			if p.current.Kind == lexer.Star {
				node.append(p.advance())
			} else {
				if p.appendAdvance(&node, lexer.Identifier, "Expected symbol name or *.") {
					return node, true
				}
			}

			hasSymbol = true
		}

		if p.appendAdvance(&node, lexer.RightBrace, "Expected '}' after symbols to import.") {
			return node, true
		}
	}

	// ;
	if p.appendAdvance(&node, lexer.Semicolon, "Expected ';' after import.") {
		return node, true
	}

	return node, false
}

func (p *parser) importPathNode() (Node, bool, Node) {
	node := Node{Kind: Path}

	// First segment
	if p.appendAdvance(&node, lexer.Identifier, "Expected an identifier.") {
		return node, true, Node{}
	}

	// Additional segments
	for p.current.Kind == lexer.Colon {
		colon := p.advance()

		if p.current.Kind == lexer.LeftBrace {
			return node, false, colon
		}

		node.append(colon)

		if p.appendAdvance(&node, lexer.Identifier, "Expected an identifier as a path segment after ':'.") {
			return node, true, Node{}
		}
	}

	return node, false, Node{}
}

func (p *parser) structNode(attributes Node) (Node, bool) {
	node := Node{Kind: Struct}
	node.append(attributes)

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

func (p *parser) enumNode(attributes Node) (Node, bool) {
	node := Node{Kind: Enum}
	node.append(attributes)

	// Keyword
	node.append(p.advance())

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected enum name.") {
		return node, true
	}

	// : <type>
	if p.current.Kind == lexer.Colon {
		// :
		node.append(p.advance())

		// <type>
		{
			child, err := p.typeNode()
			node.append(child)

			if err {
				return node, true
			}
		}
	}

	// {
	if p.appendAdvance(&node, lexer.LeftBrace, "Expected '{' before enum cases.") {
		return node, true
	}

	// Cases
	hasCase := false

	for p.current.Kind != lexer.RightBrace {
		// ,
		if hasCase {
			if p.appendAdvance(&node, lexer.Comma, "Expected ',' between enum cases.") {
				return node, true
			}
		}

		// <case>
		{
			child, err := p.enumCaseNode()
			node.append(child)

			if err {
				return node, true
			}
		}

		hasCase = true
	}

	// }
	if p.appendAdvance(&node, lexer.RightBrace, "Expected '}' after enum cases.") {
		return node, true
	}

	return node, false
}

func (p *parser) enumCaseNode() (Node, bool) {
	node := Node{Kind: EnumCase}

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected enum case name.") {
		return node, true
	}

	// = <integer>
	if p.current.Kind == lexer.Equal {
		// =
		node.append(p.advance())

		// <integer>
		if p.current.Kind != lexer.Integer && p.current.Kind != lexer.Hexadecimal && p.current.Kind != lexer.Binary {
			p.error("Expected integer for enum case value.")
			return node, true
		}

		node.append(p.advance())
	}

	return node, false
}

func (p *parser) implNode(attributes Node) (Node, bool) {
	node := Node{Kind: Impl}
	node.append(attributes)

	// Keyword
	node.append(p.advance())

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected struct name.") {
		return node, true
	}

	// {
	if p.appendAdvance(&node, lexer.LeftBrace, "Expected '{' before methods.") {
		return node, true
	}

	// Methods
	for p.current.Kind != lexer.RightBrace {
		var attributes Node

		if p.current.Kind == lexer.Hashtag {
			attrs, err := p.attributesNode()
			if err {
				return node, true
			}

			attributes = attrs
		}

		child, err := p.funcNode(attributes)
		node.append(child)

		if err {
			return node, true
		}
	}

	// }
	if p.appendAdvance(&node, lexer.RightBrace, "Expected '}' after methods.") {
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

func (p *parser) globalVarNode(attributes Node) (Node, bool) {
	node := Node{Kind: GlobalVar}
	node.append(attributes)

	// Keyword
	node.append(p.advance())

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected a global variable name.") {
		return node, true
	}

	// <type>
	{
		child, err := p.typeNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	// ;
	if p.appendAdvance(&node, lexer.Semicolon, "Expected ';' global variable.") {
		return node, true
	}

	return node, false
}

func (p *parser) funcNode(attributes Node) (Node, bool) {
	node := Node{Kind: Func}
	node.append(attributes)

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
