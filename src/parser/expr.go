package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"slices"
	"strconv"
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

	case lexer.Null:
		return p.parseNull()

	case lexer.Identifier:
		return p.parseIdentifier()

	case lexer.Sizeof:
		return p.parseSizeOf()

	case lexer.Alignof:
		return p.parseAlignOf()

	case lexer.Offsetof:
		return p.parseOffsetOf()

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
	c.Range_ = p.current.Range

	recoverId = -1

	raw := []rune(p.advance().Text)
	raw = raw[1 : len(raw)-1]

	errRange := c.Range_
	errRange.Start.Column++
	errRange.End.Column--

	if raw[0] == '\\' {
		switch raw[1] {
		case 'n':
			c.Rune = '\n'
		case 'r':
			c.Rune = '\r'
		case 't':
			c.Rune = '\t'
		case '\\':
			c.Rune = '\\'
		case '"':
			c.Rune = '"'
		case '\'':
			c.Rune = '\''
		case 'a':
			c.Rune = '\a'
		case 'b':
			c.Rune = '\b'
		case 'f':
			c.Rune = '\f'
		case 'v':
			c.Rune = '\v'

		case 'x', 'X':
			if len(raw) != 4 {
				p.reportError(errRange, "invalid hexadecimal escape sequence")
			} else {
				value, err := strconv.ParseUint(string(raw[2:]), 16, 32)

				if err != nil {
					p.reportError(errRange, "invalid hexadecimal escape sequence")
				} else {
					c.Rune = rune(value)
				}
			}

		default:
			p.reportError(errRange, "invalid escape sequence")
		}
	} else {
		if len(raw) != 1 {
			p.reportError(errRange, "character literal has more than one character")
		} else {
			c.Rune = raw[0]
		}
	}

	return
}

func (p *parser) parseString() (s *ast.String, recoverId int) {
	s = &ast.String{}
	s.Range_ = p.current.Range

	recoverId = -1

	raw := []rune(p.advance().Text)
	s.Runes = make([]rune, 0, len(raw))

	pos := s.Range_.Start
	pos.Column++

	for i := 1; i < len(raw)-1; i++ {
		ch := raw[i]

		if ch == '\\' {
			i++
			ch = raw[i]
			pos.Column++

			switch ch {
			case 'n':
				s.Runes = append(s.Runes, '\n')
			case 'r':
				s.Runes = append(s.Runes, '\r')
			case 't':
				s.Runes = append(s.Runes, '\t')
			case '\\':
				s.Runes = append(s.Runes, '\\')
			case '"':
				s.Runes = append(s.Runes, '"')
			case '\'':
				s.Runes = append(s.Runes, '\'')
			case 'a':
				s.Runes = append(s.Runes, '\a')
			case 'b':
				s.Runes = append(s.Runes, '\b')
			case 'f':
				s.Runes = append(s.Runes, '\f')
			case 'v':
				s.Runes = append(s.Runes, '\v')

			case 'x', 'X':
				if i >= len(raw)-3 {
					p.reportError(core.Range{Start: pos.Shift(-1), End: pos}, "invalid escape sequence")
				} else {
					value, err := strconv.ParseUint(string(raw[i+1:i+3]), 16, 32)
					if err != nil {
						p.reportError(core.Range{Start: pos.Shift(-1), End: pos.Shift(2)}, "invalid hexadecimal escape sequence")
					} else {
						s.Runes = append(s.Runes, rune(value))
					}

					i += 2
					pos.Column += 2
				}

			default:
				p.reportError(core.Range{Start: pos.Shift(-1), End: pos}, "invalid escape sequence")
			}

			pos.Column++
		} else {
			s.Runes = append(s.Runes, ch)

			if ch == '\n' {
				pos.Line++
				pos.Column = 1
			} else {
				pos.Column++
			}
		}
	}

	return
}

func (p *parser) parseSizeOf() (s *ast.SizeOf, recoverId int) {
	s = &ast.SizeOf{}
	s.Range_.Start = p.current.Range.Start
	defer func() {
		s.Range_.End = p.previous.Range.End

		if core.IsNil(s.Type) {
			s.Type = p.badType()
		}
	}()

	recoverId = -1

	// 'sizeof'
	if recoverId = p.expect(lexer.Sizeof, "expected 'sizeof'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before type"); recoverId >= 0 {
		return
	}

	// Type
	if s.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after type"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseAlignOf() (a *ast.AlignOf, recoverId int) {
	a = &ast.AlignOf{}
	a.Range_.Start = p.current.Range.Start
	defer func() {
		a.Range_.End = p.previous.Range.End

		if core.IsNil(a.Type) {
			a.Type = p.badType()
		}
	}()

	recoverId = -1

	// 'alignof'
	if recoverId = p.expect(lexer.Alignof, "expected 'alignof'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before type"); recoverId >= 0 {
		return
	}

	// Type
	if a.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after type"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseOffsetOf() (o *ast.OffsetOf, recoverId int) {
	o = &ast.OffsetOf{}
	o.Range_.Start = p.current.Range.Start
	defer func() {
		o.Range_.End = p.previous.Range.End

		if core.IsNil(o.Type) {
			o.Type = p.badType()
		}

		if o.Field == nil {
			o.Field = p.badLeaf()
		}
	}()

	recoverId = -1

	// 'offsetof'
	if recoverId = p.expect(lexer.Offsetof, "expected 'offsetof'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before type"); recoverId >= 0 {
		return
	}

	// Type
	if o.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	// ','
	if recoverId = p.expect(lexer.Comma, "expected ',' after type"); recoverId >= 0 {
		return
	}

	// Field
	if o.Field, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after field"); recoverId >= 0 {
		return
	}

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

	case lexer.PlusPlus:
		u.Op = ast.IncrementE
	case lexer.MinusMinus:
		u.Op = ast.DecrementE

	case lexer.Ampersand:
		u.Op = ast.AddressOf
	case lexer.Star:
		u.Op = ast.Dereference

	default:
		panic("parser.parsePrefix() - Invalid operator token '" + p.previous.Text + "'")
	}

	// Expression
	if u.Expr, recoverId = p.parseExprWithPower(rightPower); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseNull() (n *ast.Null, recoverId int) {
	n = &ast.Null{}
	n.Range_.Start = p.current.Range.Start
	defer func() {
		n.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'null'
	if recoverId = p.expect(lexer.Null, "expected 'null'"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseIdentifier() (i *ast.Identifier, recoverId int) {
	i = &ast.Identifier{}
	i.Range_.Start = p.current.Range.Start
	defer func() {
		i.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// Path
	if i.Path, recoverId = p.parseIdentifierPath(false); recoverId >= 0 {
		return
	}

	return
}

// Infix

func (p *parser) parseInfixExpr(left ast.Expr, rightPower int) (ast.Expr, int) {
	switch p.current.Kind {
	case lexer.Dot:
		return p.parseMember(left, rightPower)
	case lexer.As:
		return p.parseCast(left, rightPower)

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

	case lexer.LessLess:
		b.Op = ast.ShiftLeft
	case lexer.GreaterGreater:
		b.Op = ast.ShiftRightSignExt
	case lexer.GreaterGreaterGreater:
		b.Op = ast.ShiftRightZeroExt

	case lexer.Pipe:
		b.Op = ast.BitOr
	case lexer.Caret:
		b.Op = ast.BitXor
	case lexer.Ampersand:
		b.Op = ast.BitAnd

	case lexer.PipePipe:
		b.Op = ast.BoolOr
	case lexer.AmpersandAmpersand:
		b.Op = ast.BoolAnd

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

	case lexer.Equal:
		b.Op = ast.Assign

	case lexer.PlusEqual:
		b.Op = ast.AddAssign
	case lexer.MinusEqual:
		b.Op = ast.SubtractAssign
	case lexer.StarEqual:
		b.Op = ast.MultiplyAssign
	case lexer.SlashEqual:
		b.Op = ast.DivideAssign
	case lexer.PercentageEqual:
		b.Op = ast.ModuloAssign

	case lexer.LessLessEqual:
		b.Op = ast.ShiftLeftAssign
	case lexer.GreaterGreaterEqual:
		b.Op = ast.ShiftRightSignExtAssign
	case lexer.GreaterGreaterGreaterEqual:
		b.Op = ast.ShiftRightZeroExtAssign

	case lexer.PipeEqual:
		b.Op = ast.BitOrAssign
	case lexer.CaretEqual:
		b.Op = ast.BitXorAssign
	case lexer.AmpersandEqual:
		b.Op = ast.BitAndAssign

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

func (p *parser) parseCast(left ast.Expr, _ int) (c *ast.Cast, recoverId int) {
	c = &ast.Cast{}
	c.Range_.Start = left.Range().Start
	c.Expr = left
	defer func() {
		c.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'as'
	if recoverId = p.expect(lexer.As, "expected 'as' before target type"); recoverId >= 0 {
		return
	}

	// Type
	if c.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	return
}

// Postfix

func (p *parser) parsePostfixExpr(left ast.Expr) (ast.Expr, int) {
	switch p.current.Kind {
	case lexer.PlusPlus, lexer.MinusMinus:
		return p.parsePostfix(left)

	case lexer.LeftBracket:
		return p.parseIndex(left)

	case lexer.LeftParen:
		return p.parseCall(left, nil)

	case lexer.LeftBrace:
		return p.parseStructInitializer(left, nil)

	case lexer.ColonColon:
		var typeArgs []ast.Type
		recoverId := -1

		if typeArgs, recoverId = p.parseTurbofish(); recoverId >= 0 {
			return nil, recoverId
		}

		switch p.current.Kind {
		case lexer.LeftParen:
			return p.parseCall(left, typeArgs)
		case lexer.LeftBrace:
			return p.parseStructInitializer(left, typeArgs)

		default:
			b := &ast.BadExpr{}
			b.Range_ = p.current.Range
			return b, p.error("expected '(' or '{' after generic arguments")
		}

	default:
		b := &ast.BadExpr{}
		b.Range_ = p.current.Range
		return b, p.error("expected expression")
	}
}

func (p *parser) parsePostfix(left ast.Expr) (e *ast.Postfix, recoverId int) {
	e = &ast.Postfix{}
	e.Expr = left
	e.Range_.Start = left.Range().Start
	defer func() {
		e.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	switch p.advance().Kind {
	case lexer.PlusPlus:
		e.Op = ast.IncrementO
	case lexer.MinusMinus:
		e.Op = ast.DecrementO

	default:
		panic("parser.parser.parsePostfix() - Invalid operator token '" + p.previous.Text + "'")
	}

	return
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

func (p *parser) parseCall(left ast.Expr, typeArgs []ast.Type) (c *ast.Call, recoverId int) {
	c = &ast.Call{}
	c.Callee = left
	c.TypeArgs = typeArgs
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

func (p *parser) parseStructInitializer(left ast.Expr, typeArgs []ast.Type) (s *ast.StructInitializer, recoverId int) {
	s = &ast.StructInitializer{}
	s.Range_.Start = left.Range().Start
	defer func() {
		s.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// Type
	path := left.(*ast.Identifier).Path

	if len(path.Entries) == 1 && path.Entries[0].Token.Text == "Self" && len(typeArgs) == 0 {
		t := &ast.SelfType{}
		t.Range_ = left.Range()

		s.Type = t
	} else {
		t := &ast.IdentifierType{Path: path, TypeArgs: typeArgs}
		t.Range_ = left.Range()

		s.Type = t
	}

	// '{'
	if recoverId = p.expect(lexer.LeftBrace, "expected '{' before fields"); recoverId >= 0 {
		return
	}

	// Fields
	myRecoverId := p.pushRecoverPoint(lexer.RightBrace)
	s.Fields, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightBrace, p.parseFieldInitializer)
	p.popRecoverPoint()

	if recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			return
		}
	}

	// '}'
	if recoverId = p.expect(lexer.RightBrace, "expected '}' after fields"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseFieldInitializer() (f *ast.FieldInitializer, recoverId int) {
	f = &ast.FieldInitializer{}
	f.Range_.Start = p.current.Range.Start
	defer func() {
		f.Range_.End = p.previous.Range.End

		if core.IsNil(f.Value) {
			f.Value = p.badExpr()
		}
	}()

	recoverId = -1

	// Name
	if f.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// ':'
	if recoverId = p.expect(lexer.Colon, "expected ':' before field value"); recoverId >= 0 {
		return
	}

	// Value
	if f.Value, recoverId = p.parseExpr(); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseTurbofish() (typeArgs []ast.Type, recoverId int) {
	recoverId = -1

	// '::'
	if recoverId = p.expect(lexer.ColonColon, "expected '::' before type arguments"); recoverId >= 0 {
		return
	}

	// '['
	if recoverId = p.expect(lexer.LeftBracket, "expected '[' before type arguments"); recoverId >= 0 {
		return
	}

	// Type Arguments
	myRecoverId := p.pushRecoverPoint(lexer.RightBracket)
	typeArgs, recoverId = parseCommaList(p, lexer.Comma, lexer.RightBracket, p.parseType)
	p.popRecoverPoint()

	if recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			return
		}
	}

	// ']'
	if recoverId = p.expect(lexer.RightBracket, "expected ']' after type arguments"); recoverId >= 0 {
		return
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

	// =, +=, -=, *=, /=, %=, <<=, >>=, >>>= |=, ^=, &=
	infix(true, lexer.Equal, lexer.PlusEqual, lexer.MinusEqual, lexer.StarEqual, lexer.SlashEqual, lexer.PercentageEqual, lexer.LessLessEqual, lexer.GreaterGreaterEqual, lexer.GreaterGreaterGreaterEqual, lexer.PipeEqual, lexer.CaretEqual, lexer.AmpersandEqual)
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
	// as
	infix(false, lexer.As)
	// <<, >>, >>>
	infix(false, lexer.LessLess, lexer.Greater, lexer.GreaterGreater, lexer.GreaterGreaterGreater)
	// +, -
	infix(false, lexer.Plus, lexer.Minus)
	// *, /, %
	infix(false, lexer.Star, lexer.Slash, lexer.Percentage)
	// -x, !x, ++x, --x, &x, *x
	prefix(lexer.Minus, lexer.Bang, lexer.PlusPlus, lexer.MinusMinus, lexer.Ampersand, lexer.Star)
	// x++, x--
	postfix(lexer.PlusPlus, lexer.MinusMinus)
	// x[], x(), x {}, x::[]
	postfix(lexer.LeftBracket, lexer.LeftParen, lexer.LeftBrace, lexer.ColonColon)
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
