package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
)

func (p *parser) parseNameType() (n *ast.NameType, recoverId int) {
	n = &ast.NameType{}
	n.Range_.Start = p.current.Range.Start
	defer func() {
		n.Range_.End = p.previous.Range.End

		if core.IsNil(n.Type) {
			n.Type = p.badType()
		}
	}()

	recoverId = -1

	// Name
	if n.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// ':'
	if recoverId = p.expect(lexer.Colon, "expected ':' before type"); recoverId >= 0 {
		return
	}

	// Type
	if n.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseIdentifierPath(stopAtLeftBrace bool) (i *ast.IdentifierPath, recoverId int) {
	i = &ast.IdentifierPath{}
	i.Range_.Start = p.current.Range.Start
	defer func() {
		i.Range_.End = p.previous.Range.End
	}()

	var entry *ast.Leaf

	// First entry
	entry, recoverId = p.parseLeaf()
	i.Entries = append(i.Entries, entry)
	if recoverId >= 0 {
		return
	}

	// ('::' entry)*
	for p.current.Kind == lexer.ColonColon && (!stopAtLeftBrace || p.next.Kind != lexer.LeftBrace) {
		// '::'
		if recoverId = p.expect(lexer.ColonColon, "expected '::' before identifier"); recoverId >= 0 {
			return
		}

		// Entry
		entry, recoverId = p.parseLeaf()
		i.Entries = append(i.Entries, entry)
		if recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseLeaf() (l *ast.Leaf, recoverId int) {
	l = &ast.Leaf{}

	if recoverId = p.expect(lexer.Identifier, "expected an identifier"); recoverId == -1 {
		l.Token = p.previous
	}

	return
}

// Helper

var emptyLeaf = &ast.Leaf{Token: lexer.Token{Kind: lexer.Identifier, Text: ""}}

func (p *parser) badDecl() *ast.BadDecl {
	b := &ast.BadDecl{}
	b.Range_ = p.previous.Range
	return b
}

func (p *parser) badStmt() *ast.BadStmt {
	b := &ast.BadStmt{}
	b.Range_ = p.previous.Range
	return b
}

func (p *parser) badExpr() *ast.BadExpr {
	b := &ast.BadExpr{}
	b.Range_ = p.previous.Range
	return b
}

func (p *parser) badType() *ast.BadType {
	b := &ast.BadType{}
	b.Range_ = p.previous.Range
	return b
}

func parseCommaList[T any](p *parser, itemStartToken, endToken lexer.TokenKind, parseItem func() (T, int)) (items []T, recoverId int) {
	recoverId = -1

	for p.current.Kind != endToken && p.current.Kind != lexer.EOF {
		myRecoverId := p.pushRecoverPoint(itemStartToken, lexer.Comma)

		var item T
		item, recoverId = parseItem()
		items = append(items, item)

		if recoverId == -1 {
			if p.current.Kind != endToken {
				recoverId = p.expect(lexer.Comma, "expected ','")
			}
		} else if p.current.Kind == lexer.Comma {
			p.advance()
		}

		p.popRecoverPoint()

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}
	}

	return
}

func parseParenWrapped[T any](p *parser, parseItem func() (T, int)) (item T, recoverId int) {
	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '('"); recoverId >= 0 {
		return
	}

	// Item
	myRecoverId := p.pushRecoverPoint(lexer.RightParen)

	if item, recoverId = parseItem(); recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			p.popRecoverPoint()
			return
		}
	}

	p.popRecoverPoint()

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')'"); recoverId >= 0 {
		return
	}

	return
}
