package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/types"
)

func (p *parser) parseAttributes() (attributes []ast.Attribute, recoverId int) {
	recoverId = -1

	for p.current.Kind == lexer.Hashtag {
		attrs, rec := p.parseAttributeGroup()

		attributes = append(attributes, attrs...)
		recoverId = rec

		if recoverId >= 0 {
			return
		}
	}

	return
}

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
	case "init":
		return p.parseInit()
	case "test":
		return p.parseTest()
	case "extern":
		return p.parseExtern()
	case "link_name":
		return p.parseLinkName()
	case "repr":
		return p.parseRepr()
	case "cfg":
		return p.parseCfg()

	default:
		b := &ast.BadAttribute{}
		b.Range_ = p.current.Range

		p.reportError(p.current.Range, "unknown attribute '"+p.current.Text+"'")
		p.advance()

		return b, p.error("")
	}
}

func (p *parser) parseInit() (i *ast.Init, recoverId int) {
	i = &ast.Init{}
	i.Range_.Start = p.current.Range.Start
	defer func() {
		i.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'init'
	if recoverId = p.expect(lexer.Identifier, "expected 'init'"); recoverId >= 0 {
		return
	}

	return
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
		if recoverId = p.expect(lexer.LeftParen, "expected '(' before a test name"); recoverId >= 0 {
			return
		}

		// Name
		if t.Name, recoverId = p.parseString(); recoverId >= 0 {
			return
		}

		// ')'
		if recoverId = p.expect(lexer.RightParen, "expected ')' after a test name"); recoverId >= 0 {
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
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before a link name"); recoverId >= 0 {
		return
	}

	// Name
	if l.Name, recoverId = p.parseString(); recoverId >= 0 {
		return
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after a link name"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseRepr() (r *ast.Repr, recoverId int) {
	r = &ast.Repr{}
	r.Range_.Start = p.current.Range.Start
	defer func() {
		r.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'repr'
	if recoverId = p.expect(lexer.Identifier, "expected 'repr'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before a struct layout"); recoverId >= 0 {
		return
	}

	// Layout
	if recoverId = p.expect(lexer.Identifier, "expected an identifier"); recoverId >= 0 {
		return
	}

	switch p.previous.Text {
	case "Fireball":
		r.Layout = types.Fireball
	case "C":
		r.Layout = types.C

	default:
		p.reportError(p.previous.Range, "invalid struct layout value, expected 'fireball' or 'c'")
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after a struct layout"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseCfg() (c *ast.Cfg, recoverId int) {
	c = &ast.Cfg{}
	c.Range_.Start = p.current.Range.Start
	defer func() {
		c.Range_.End = p.previous.Range.End

		if core.IsNil(c.Predicate) {
			c.Predicate = &ast.BadCfg{}
		}
	}()

	recoverId = -1

	// 'cfg'
	if recoverId = p.expect(lexer.Identifier, "expected 'cfg'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before a predicate"); recoverId >= 0 {
		return
	}

	// Predicate
	if c.Predicate, recoverId = p.parseCfgPredicate(); recoverId >= 0 {
		return
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after a predicate"); recoverId >= 0 {
		return
	}

	return
}
