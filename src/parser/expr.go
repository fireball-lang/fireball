package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"slices"
)

func (p *parser) parseExpr() (ast.Expr, int) {
	return p.parseExprWithPower(0)
}

func (p *parser) parseExprWithPower(minPower int) (left ast.Expr, recoverId int) {
	// Prefix
	if left, recoverId = p.parsePrefixExpr(); recoverId >= 0 {
		return
	}

	for isIndexOrPostfix(p.current) {
		op := p.current

		// Postfix
		if leftPower := postfixExprPower(op); leftPower != -1 {
			if leftPower < minPower {
				break
			}

			if left, recoverId = p.parsePostfixExpr(left); recoverId >= 0 {
				return
			}

			continue
		}

		// Infix
		if leftPower, rightPower := infixExprPower(op); leftPower != -1 {
			if leftPower < minPower {
				break
			}

			if left, recoverId = p.parseInfixExpr(left, rightPower); recoverId >= 0 {
				return
			}
		}
	}

	return
}

// Prefix

func (p *parser) parsePrefixExpr() (ast.Expr, int) {
	switch p.current.Kind {
	case lexer.LeftParen:
		return parseParenWrapped(p, p.parseExpr)

	case lexer.True, lexer.False:
		return p.parseBool()

	case lexer.BinaryInteger, lexer.HexInteger, lexer.UnsignedInteger, lexer.SignedInteger, lexer.Decimal, lexer.Decimal32bit:
		return p.parseNumber()

	case lexer.Character:
		return p.parseCharacter()

	case lexer.String:
		return p.parseString()

	case lexer.Identifier:
		return p.parseIdentifier()

	default:
		rightPower := prefixExprPower(p.current)

		if rightPower == -1 {
			b := &ast.BadExpr{}
			b.Range_ = p.current.Range
			return b, p.error("expected expression")
		}

		return p.parsePrefix(rightPower)
	}
}

func (p *parser) parseBool() (b *ast.Bool, recoverId int) {
	b = &ast.Bool{}
	b.Range_.Start = p.current.Range.Start
	defer func() {
		b.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'true' | 'false'
	if p.advance().Kind == lexer.True {
		b.Value = true
	}

	return
}

func (p *parser) parseNumber() (n *ast.Number, recoverId int) {
	n = &ast.Number{}
	n.Token = p.advance()

	recoverId = -1

	return
}

func (p *parser) parseCharacter() (c *ast.Character, recoverId int) {
	c = &ast.Character{}
	c.Token = p.advance()

	recoverId = -1

	return
}

func (p *parser) parseString() (s *ast.String, recoverId int) {
	s = &ast.String{}
	s.Token = p.advance()

	recoverId = -1

	return
}

func (p *parser) parsePrefix(rightPower int) (u *ast.Prefix, recoverId int) {
	u = &ast.Prefix{}
	u.Range_.Start = p.current.Range.Start
	defer func() {
		u.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// Operator
	switch p.advance().Kind {
	case lexer.Minus:
		u.Op = ast.Negate
	case lexer.Bang:
		u.Op = ast.Not

	default:
		panic("parser.parsePrefix() - Invalid operator token '" + p.previous.Text + "'")
	}

	// Expression
	if u.Expr, recoverId = p.parseExprWithPower(rightPower); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseIdentifier() (i *ast.Identifier, recoverId int) {
	i = &ast.Identifier{}
	i.Token = p.advance()

	recoverId = -1

	return
}

// Infix

func (p *parser) parseInfixExpr(left ast.Expr, rightPower int) (ast.Expr, int) {
	switch p.current.Kind {
	case lexer.Dot:
		return p.parseMember(left, rightPower)

	default:
		return p.parseBinary(left, rightPower)
	}
}

func (p *parser) parseBinary(left ast.Expr, rightPower int) (b *ast.Binary, recoverId int) {
	b = &ast.Binary{}
	b.Range_.Start = left.Range().Start
	b.Left = left
	defer func() {
		b.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// Operator
	switch p.advance().Kind {
	case lexer.Equal:
		b.Op = ast.Assign

	case lexer.PipePipe:
		b.Op = ast.BoolOr
	case lexer.AmpersandAmpersand:
		b.Op = ast.BoolAnd

	case lexer.Plus:
		b.Op = ast.Add
	case lexer.Minus:
		b.Op = ast.Subtract
	case lexer.Star:
		b.Op = ast.Multiply
	case lexer.Slash:
		b.Op = ast.Divide
	case lexer.Percentage:
		b.Op = ast.Modulo

	case lexer.Pipe:
		b.Op = ast.BitOr
	case lexer.Caret:
		b.Op = ast.BitXor
	case lexer.Ampersand:
		b.Op = ast.BitAnd

	case lexer.EqualEqual:
		b.Op = ast.Equal
	case lexer.BangEqual:
		b.Op = ast.NotEqual

	case lexer.Less:
		b.Op = ast.Less
	case lexer.LessEqual:
		b.Op = ast.LessEqual
	case lexer.Greater:
		b.Op = ast.Greater
	case lexer.GreaterEqual:
		b.Op = ast.GreaterEqual

	default:
		panic("parser.parseBinary() - Invalid operator token '" + p.previous.Text + "'")
	}

	// Right
	if b.Right, recoverId = p.parseExprWithPower(rightPower); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseMember(left ast.Expr, _ int) (m *ast.Member, recoverId int) {
	m = &ast.Member{}
	m.Range_.Start = left.Range().Start
	m.Expr = left
	m.Name = emptyLeaf
	defer func() {
		m.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// '.'
	if recoverId = p.expect(lexer.Dot, "expected '.' before member name"); recoverId >= 0 {
		return
	}

	// Name
	if m.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	return
}

// Postfix

func (p *parser) parsePostfixExpr(left ast.Expr) (ast.Expr, int) {
	switch p.current.Kind {
	case lexer.LeftBracket:
		return p.parseIndex(left)
	case lexer.LeftParen:
		return p.parseCall(left)

	default:
		b := &ast.BadExpr{}
		b.Range_ = p.current.Range
		return b, p.error("expected expression")
	}
}

func (p *parser) parseIndex(left ast.Expr) (i *ast.Index, recoverId int) {
	i = &ast.Index{}
	i.Expr = left
	i.Range_.Start = left.Range().Start
	defer func() {
		i.Range_.End = p.previous.Range.End

		if core.IsNil(i.Index) {
			i.Index = p.badExpr()
		}
	}()

	recoverId = -1

	// '['
	if recoverId = p.expect(lexer.LeftBracket, "expected '[' before index"); recoverId >= 0 {
		return
	}

	// Index
	myRecoverId := p.pushRecoverPoint(lexer.RightBracket)
	i.Index, recoverId = p.parseExpr()
	p.popRecoverPoint()

	if recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			return
		}
	}

	// ']'
	if recoverId = p.expect(lexer.RightBracket, "expected ']' after index"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseCall(left ast.Expr) (c *ast.Call, recoverId int) {
	c = &ast.Call{}
	c.Callee = left
	c.Range_.Start = left.Range().Start
	defer func() {
		c.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// '(' Arguments ')'
	{
		// '('
		if recoverId = p.expect(lexer.LeftParen, "expected '(' before arguments"); recoverId >= 0 {
			return
		}

		// Arguments
		myRecoverId := p.pushRecoverPoint(lexer.RightParen)
		c.Args, recoverId = parseCommaList(p, lexer.Comma, lexer.RightParen, p.parseExpr)
		p.popRecoverPoint()

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}

		// ')'
		if recoverId = p.expect(lexer.RightParen, "expected ')' after arguments"); recoverId >= 0 {
			return
		}
	}

	return
}

// Powers

type tokenPowers struct {
	prefixRightPower int

	infixLeftPower  int
	infixRightPower int

	postfixLeftPower int
}

var tokenPowerTable = make([]tokenPowers, lexer.Last)
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

	// =
	infix(true, lexer.Equal)
	// ||
	infix(false, lexer.PipePipe)
	// &&
	infix(false, lexer.AmpersandAmpersand)
	// |
	infix(false, lexer.Pipe)
	// ^
	infix(false, lexer.Caret)
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
	// -x, !x
	prefix(lexer.Minus, lexer.Bang)
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

func isIndexOrPostfix(token lexer.Token) bool {
	return slices.Contains(infixAndPostfixOperators, token.Kind)
}

func prefixExprPower(token lexer.Token) int {
	return tokenPowerTable[token.Kind].prefixRightPower
}

func infixExprPower(token lexer.Token) (int, int) {
	powers := tokenPowerTable[token.Kind]
	return powers.infixLeftPower, powers.infixRightPower
}

func postfixExprPower(token lexer.Token) int {
	return tokenPowerTable[token.Kind].postfixLeftPower
}
