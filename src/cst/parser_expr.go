package cst

import (
	"fireball/lexer"
	"slices"
)

func (p *parser) exprNode(minPower int) (Node, bool) {
	lhs, err := p.prefixExprNode()
	if err {
		return lhs, true
	}

	for slices.Contains(infixAndPostfixOperators, p.current.Kind) {
		op := p.current.Kind

		if leftPower := postfixExprPower(op); leftPower != -1 {
			if leftPower < minPower {
				break
			}

			lhs, err = p.postfixExprNode(op, lhs)
			if err {
				return lhs, true
			}
		}

		if leftPower, rightPower := infixExprPower(op); leftPower != -1 {
			if leftPower < minPower {
				break
			}

			lhs, err = p.infixExprNode(op, lhs, rightPower)
			if err {
				return lhs, true
			}
		}
	}

	return lhs, false
}

// Prefix

func (p *parser) prefixExprNode() (Node, bool) {
	switch p.current.Kind {
	case lexer.Number, lexer.String:
		return p.literalNode()

	case lexer.LeftParen:
		return p.parenNode()
	case lexer.LeftBrace:
		return p.blockNode()

	case lexer.Identifier:
		switch p.current.Text {
		case "var":
			return p.varNode()
		case "if":
			return p.ifNode()
		case "while":
			return p.whileNode()
		case "for":
			return p.forNode()
		case "return":
			return p.returnNode()
		default:
			return p.identifierNode()
		}

	default:
		return p.prefixUnaryNode()
	}
}

func (p *parser) literalNode() (Node, bool) {
	node := Node{Kind: Literal}

	node.append(p.advance())

	return node, false
}

func (p *parser) parenNode() (Node, bool) {
	node := Node{Kind: Paren}

	// (
	node.append(p.advance())

	// <expr>
	{
		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	// )
	if p.appendAdvance(&node, lexer.RightParen, "Expected ')' after expression.") {
		return node, true
	}

	return node, false
}

func (p *parser) blockNode() (Node, bool) {
	node := Node{Kind: Block}

	// {
	node.append(p.advance())

	// Body
	for p.current.Kind != lexer.RightBrace {
		child, err := p.exprSemicolonNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	// }
	if p.appendAdvance(&node, lexer.RightBrace, "Expected '}' after block.") {
		return node, true
	}

	return node, false
}

func (p *parser) varNode() (Node, bool) {
	node := Node{Kind: Var}

	// Keyword
	node.append(p.advance())

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected a variable name.") {
		return node, true
	}

	// : <type>
	if p.current.Kind == lexer.Colon {
		node.append(p.advance())

		child, err := p.typeNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	// = <expr>
	if p.current.Kind == lexer.Equal {
		node.append(p.advance())

		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}

func (p *parser) ifNode() (Node, bool) {
	node := Node{Kind: If}

	// Keyword
	node.append(p.advance())

	// (
	if p.appendAdvance(&node, lexer.LeftParen, "Expected '(' before condition.") {
		return node, true
	}

	// <expr>
	{
		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	// )
	if p.appendAdvance(&node, lexer.RightParen, "Expected ')' after condition.") {
		return node, true
	}

	// <expr>
	{
		child, err := p.exprSemicolonNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	// else <expr>
	if p.current.Text == "else" {
		node.append(p.advance())

		child, err := p.exprSemicolonNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}

func (p *parser) whileNode() (Node, bool) {
	node := Node{Kind: While}

	// Keyword
	node.append(p.advance())

	// (
	if p.appendAdvance(&node, lexer.LeftParen, "Expected '(' before condition.") {
		return node, true
	}

	// <expr>
	{
		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	// )
	if p.appendAdvance(&node, lexer.RightParen, "Expected ')' after condition.") {
		return node, true
	}

	// <expr>
	{
		child, err := p.exprSemicolonNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}

func (p *parser) forNode() (Node, bool) {
	node := Node{Kind: For}

	// Keyword
	node.append(p.advance())

	// (
	if p.appendAdvance(&node, lexer.LeftParen, "Expected '(' before for clauses.") {
		return node, true
	}

	// <expr>
	if p.current.Kind != lexer.Semicolon {
		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	// ;
	if p.appendAdvance(&node, lexer.Semicolon, "Expected ';' between for clauses.") {
		return node, true
	}

	// <expr>
	if p.current.Kind != lexer.Semicolon {
		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	// ;
	if p.appendAdvance(&node, lexer.Semicolon, "Expected ';' between for clauses.") {
		return node, true
	}

	// <expr>
	if p.current.Kind != lexer.RightParen {
		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	// )
	if p.appendAdvance(&node, lexer.RightParen, "Expected ')' after for clauses.") {
		return node, true
	}

	// <expr>
	{
		child, err := p.exprSemicolonNode()
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}

func (p *parser) returnNode() (Node, bool) {
	node := Node{Kind: Return}

	// Keyword
	node.append(p.advance())

	// <expr>
	if p.current.Kind != lexer.Semicolon {
		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}

func (p *parser) identifierNode() (Node, bool) {
	node := Node{Kind: Identifier}

	child := p.advance()
	node.append(child)

	if child.Token.Text == "true" || child.Token.Text == "false" || child.Token.Text == "nil" {
		node.Kind = Literal
	}

	return node, false
}

func (p *parser) prefixUnaryNode() (Node, bool) {
	rightPower := prefixExprPower(p.current.Kind)

	if rightPower == -1 {
		p.error("Expected an expression.")
		return Node{}, true
	}

	node := Node{Kind: Unary}

	// Operator
	node.append(p.advance())

	// <expr>
	{
		child, err := p.exprNode(rightPower)
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}

// Infix

func (p *parser) infixExprNode(op lexer.TokenKind, lhs Node, rightPower int) (Node, bool) {
	switch op {
	case lexer.Dot:
		return p.memberNode(lhs)
	default:
		return p.binaryNode(lhs, rightPower)
	}
}

func (p *parser) memberNode(lhs Node) (Node, bool) {
	node := Node{Kind: Member}

	// <expr>
	node.append(lhs)

	// .
	node.append(p.advance())

	// Name
	if p.appendAdvance(&node, lexer.Identifier, "Expected member name.") {
		return node, true
	}

	return node, false
}

func (p *parser) binaryNode(lhs Node, rightPower int) (Node, bool) {
	node := Node{Kind: Binary}

	// Left
	node.append(lhs)

	// Operator
	node.append(p.advance())

	// Right
	{
		child, err := p.exprNode(rightPower)
		node.append(child)

		if err {
			return node, true
		}
	}

	return node, false
}

// Postfix

func (p *parser) postfixExprNode(op lexer.TokenKind, lhs Node) (Node, bool) {
	switch op {
	case lexer.LeftBracket:
		return p.indexNode(lhs)
	case lexer.LeftParen:
		return p.callNode(lhs)
	default:
		return p.postfixUnaryNode(lhs)
	}
}

func (p *parser) indexNode(lhs Node) (Node, bool) {
	node := Node{Kind: Index}

	// <expr>
	node.append(lhs)

	// [
	node.append(p.advance())

	// <expr>
	{
		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}
	}

	// ]
	if p.appendAdvance(&node, lexer.RightBracket, "Expected ']' after index expression.") {
		return node, true
	}

	return node, false
}

func (p *parser) callNode(lhs Node) (Node, bool) {
	node := Node{Kind: Call}

	// <expr>
	node.append(lhs)

	// (
	node.append(p.advance())

	// Arguments
	hasArg := false

	for p.current.Kind != lexer.RightParen {
		if hasArg {
			if p.appendAdvance(&node, lexer.Comma, "Expected ',' between function arguments.") {
				return node, true
			}
		}

		child, err := p.exprNode(0)
		node.append(child)

		if err {
			return node, true
		}

		hasArg = true
	}

	// )
	if p.appendAdvance(&node, lexer.RightParen, "Expected ')' after function arguments.") {
		return node, true
	}

	return node, false
}

func (p *parser) postfixUnaryNode(lhs Node) (Node, bool) {
	node := Node{Kind: Unary}

	// <expr>
	node.append(lhs)

	// Operator
	node.append(p.advance())

	return node, false
}

// Utils

func (p *parser) exprSemicolonNode() (Node, bool) {
	node, err := p.exprNode(0)

	if err {
		return node, true
	}

	if needsSemicolon(node.Kind) {
		if p.appendAdvance(&node, lexer.Semicolon, "Expected ';' after expression.") {
			return node, true
		}
	}

	return node, false
}

func needsSemicolon(kind NodeKind) bool {
	return (kind >= Literal && kind <= Binary) || kind == Var || kind == Return
}

// Powers

type tokenPowers struct {
	prefixRightPower int

	infixLeftPower  int
	infixRightPower int

	postfixLeftPower int
}

var tokenPowerTable = make([]tokenPowers, lexer.Eof+1)
var tokenPowerTableCount = 0

var infixAndPostfixOperators []lexer.TokenKind

func init() {
	// Set every power to -1
	for i := 0; i < len(tokenPowerTable); i++ {
		tokenPowerTable[i] = tokenPowers{
			prefixRightPower: -1,
			infixLeftPower:   -1,
			infixRightPower:  -1,
			postfixLeftPower: -1,
		}
	}

	// =, +=, -=, *=, /=, %=, |=, ^=, &=
	infix(false, lexer.Equal, lexer.PlusEqual, lexer.MinusEqual, lexer.StarEqual, lexer.SlashEqual, lexer.PercentageEqual, lexer.PipeEqual, lexer.XorEqual, lexer.AmpersandEqual)
	// ||
	infix(false, lexer.PipePipe)
	// &&
	infix(false, lexer.AmpersandAmpersand)
	// |
	infix(false, lexer.Pipe)
	// ^
	infix(false, lexer.Xor)
	// &
	infix(false, lexer.Ampersand)
	// ==, !=
	infix(false, lexer.EqualEqual, lexer.BangEqual)
	// >, <=, >, >=
	infix(false, lexer.Less, lexer.LessEqual, lexer.Greater, lexer.GreaterEqual)
	// +, -
	infix(false, lexer.Plus, lexer.Minus)
	// *, /, %
	infix(false, lexer.Star, lexer.Slash, lexer.Percentage)
	// -x, !x, ++x, --x, &x, *x
	prefix(lexer.Minus, lexer.Bang, lexer.PlusPlus, lexer.MinusMinus, lexer.Ampersand, lexer.Star)
	// x++, x--
	postfix(lexer.PlusPlus, lexer.MinusMinus)
	// x[], x()
	postfix(lexer.LeftBracket, lexer.LeftParen)
	// x.y
	infix(false, lexer.Dot)

}

func prefix(kinds ...lexer.TokenKind) {
	for _, kind := range kinds {
		tokenPowerTable[kind].prefixRightPower = (tokenPowerTableCount * 2) + 1
	}

	tokenPowerTableCount++
}

func infix(rightAssociative bool, kinds ...lexer.TokenKind) {
	for _, kind := range kinds {
		if rightAssociative {
			tokenPowerTable[kind].infixLeftPower = (tokenPowerTableCount * 2) + 2
			tokenPowerTable[kind].infixRightPower = (tokenPowerTableCount * 2) + 1
		} else {
			tokenPowerTable[kind].infixLeftPower = (tokenPowerTableCount * 2) + 1
			tokenPowerTable[kind].infixRightPower = (tokenPowerTableCount * 2) + 2
		}

		infixAndPostfixOperators = append(infixAndPostfixOperators, kind)
	}

	tokenPowerTableCount++
}

func postfix(kinds ...lexer.TokenKind) {
	for _, kind := range kinds {
		tokenPowerTable[kind].postfixLeftPower = (tokenPowerTableCount * 2) + 1

		infixAndPostfixOperators = append(infixAndPostfixOperators, kind)
	}

	tokenPowerTableCount++
}

func prefixExprPower(kind lexer.TokenKind) int {
	return tokenPowerTable[kind].prefixRightPower
}

func infixExprPower(kind lexer.TokenKind) (int, int) {
	powers := tokenPowerTable[kind]
	return powers.infixLeftPower, powers.infixRightPower
}

func postfixExprPower(kind lexer.TokenKind) int {
	return tokenPowerTable[kind].postfixLeftPower
}
