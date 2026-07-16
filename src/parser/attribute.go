package parser

import (
	"fireball/ast"
	"fireball/lexer"
)

func (p *parser) parseAttributeGroup() (attributes []ast.Attribute, recoverId int) {
	recoverId = -1

	// '#'
	if recoverId = p.expect(lexer.Hashtag, "expected '#' before attribute group"); recoverId >= 0 {
		return
	}

	// '['
	if recoverId = p.expect(lexer.LeftBracket, "expected '[' before attributes"); recoverId >= 0 {
		return
	}

	// Attributes
	myRecoverId := p.pushRecoverPoint(lexer.RightBracket)
	attributes, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightBracket, p.parseAttribute)
	p.popRecoverPoint()

	if recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			return
		}
	}

	// ']'
	if recoverId = p.expect(lexer.RightBracket, "expected ']' after attributes"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseAttribute() (ast.Attribute, int) {
	if p.current.Kind != lexer.Identifier {
		b := &ast.BadAttribute{}
		b.Range_ = p.current.Range
		return b, p.error("expected attribute")
	}

	switch p.current.Text {
	case "test":
		return p.parseTest()
	case "extern":
		return p.parseExtern()
	case "link_name":
		return p.parseLinkName()

	default:
		b := &ast.BadAttribute{}
		b.Range_ = p.current.Range

		recoverId := p.error("unknown attribute '" + p.current.Text + "'")
		p.advance()

		return b, recoverId
	}
}

func (p *parser) parseTest() (t *ast.Test, recoverId int) {
	t = &ast.Test{}
	t.Range_.Start = p.current.Range.Start
	defer func() {
		t.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'test'
	if recoverId = p.expect(lexer.Identifier, "expected 'test'"); recoverId >= 0 {
		return
	}

	// '(' Name ')'
	if p.current.Kind == lexer.LeftParen {
		// '('
		if recoverId = p.expect(lexer.LeftParen, "expected '(' before test name"); recoverId >= 0 {
			return
		}

		// Name
		if t.Name, recoverId = p.parseString(); recoverId >= 0 {
			return
		}

		// ')'
		if recoverId = p.expect(lexer.RightParen, "expected ')' after test name"); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseExtern() (e *ast.Extern, recoverId int) {
	e = &ast.Extern{}
	e.Range_.Start = p.current.Range.Start
	defer func() {
		e.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'extern'
	if recoverId = p.expect(lexer.Identifier, "expected 'extern'"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseLinkName() (l *ast.LinkName, recoverId int) {
	l = &ast.LinkName{}
	l.Range_.Start = p.current.Range.Start
	defer func() {
		l.Range_.End = p.previous.Range.End

		if l.Name == nil {
			l.Name = &ast.String{}
		}
	}()

	recoverId = -1

	// 'link_name'
	if recoverId = p.expect(lexer.Identifier, "expected 'link_name'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before link name"); recoverId >= 0 {
		return
	}

	// Name
	if l.Name, recoverId = p.parseString(); recoverId >= 0 {
		return
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after link name"); recoverId >= 0 {
		return
	}

	return
}
