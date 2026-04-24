package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/types"
	"slices"
)

func (p *parser) parseDecl() (ast.Decl, int) {
	var attributes []*ast.Attribute

	for p.current.Kind == lexer.Hashtag {
		attrs, recoverId := p.parseAttributeGroup()
		attributes = append(attributes, attrs...)

		if recoverId >= 0 {
			break
		}
	}

	switch p.current.Kind {
	case lexer.Struct:
		return p.parseStruct(attributes)
	case lexer.Impl:
		if len(attributes) != 0 {
			p.reportError(sliceRange(attributes), "implementation blocks cannot have attributes")
		}
		return p.parseImpl()
	case lexer.Func:
		return p.parseFunc(attributes, false)

	default:
		b := &ast.BadDecl{}
		b.Range_ = p.current.Range
		return b, p.error("expected declaration")
	}
}

func (p *parser) parseAttributeGroup() (attributes []*ast.Attribute, recoverId int) {
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

func (p *parser) parseAttribute() (a *ast.Attribute, recoverId int) {
	a = &ast.Attribute{}
	a.Range_.Start = p.current.Range.Start
	defer func() {
		a.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// Name
	if a.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// '(' Arguments ')'
	if p.current.Kind == lexer.LeftParen {
		// '('
		if recoverId = p.expect(lexer.LeftParen, "expected '(' before attribute arguments"); recoverId >= 0 {
			return
		}

		// Arguments
		myRecoverId := p.pushRecoverPoint(lexer.RightParen)
		a.Arguments, recoverId = parseCommaList(p, lexer.Comma, lexer.RightParen, p.parseExpr)
		p.popRecoverPoint()

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}

		// ')'
		if recoverId = p.expect(lexer.RightParen, "expected ')' after attribute arguments"); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseStruct(attributes []*ast.Attribute) (s *ast.Struct, recoverId int) {
	s = &ast.Struct{}
	s.Range_.Start = p.current.Range.Start
	s.Attributes = attributes
	s.Name_ = emptyLeaf
	defer func() {
		s.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'struct'
	if recoverId = p.expect(lexer.Struct, "expected 'struct'"); recoverId >= 0 {
		return
	}

	// Name
	if s.Name_, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// '{' Fields '}'
	{
		// '{'
		if recoverId = p.expect(lexer.LeftBrace, "expected '{' before struct fields"); recoverId >= 0 {
			return
		}

		// Fields
		myRecoverId := p.pushRecoverPoint(lexer.RightBrace)
		s.Fields, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightBrace, p.parseNameType)
		p.popRecoverPoint()

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}

		// '}'
		if recoverId = p.expect(lexer.RightBrace, "expected '}' after struct fields"); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseImpl() (i *ast.Impl, recoverId int) {
	i = &ast.Impl{}
	i.Range_.Start = p.current.Range.Start
	defer func() {
		i.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'impl'
	if recoverId = p.expect(lexer.Impl, "expected 'impl'"); recoverId >= 0 {
		return
	}

	// Type
	if i.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	// '{'
	if recoverId = p.expect(lexer.LeftBrace, "expected '{' before members"); recoverId >= 0 {
		return
	}

	// Methods

	for p.current.Kind != lexer.RightBrace && p.current.Kind != lexer.EOF {
		myRecoverId := p.pushRecoverPoint(lexer.RightBrace, lexer.Func)

		var f *ast.Func
		f, recoverId = p.parseFunc(nil, true)
		i.Functions = append(i.Functions, f)

		p.popRecoverPoint()

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}
	}

	// '}'
	if recoverId = p.expect(lexer.RightBrace, "expected '}' after members"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseFunc(attributes []*ast.Attribute, allowReceiver bool) (f *ast.Func, recoverId int) {
	f = &ast.Func{}
	f.Range_.Start = p.current.Range.Start
	f.Attributes = attributes
	f.Name_ = emptyLeaf
	defer func() {
		f.Range_.End = p.previous.Range.End

		if core.IsNil(f.Returns) {
			f.Returns = p.badType()
		}
	}()

	recoverId = -1

	// 'func'
	if recoverId = p.expect(lexer.Func, "expected 'func'"); recoverId >= 0 {
		return
	}

	// Name
	if f.Name_, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// '(' Parameters ')'
	{
		// '('
		if recoverId = p.expect(lexer.LeftParen, "expected '(' before function parameters"); recoverId >= 0 {
			return
		}

		// Receiver
		if allowReceiver && (p.current.Kind == lexer.Mut || (p.current.Kind == lexer.Identifier && p.current.Text == "self")) {
			if f.Receiver, recoverId = p.parseReceiver(); recoverId >= 0 {
				return
			}

			if p.current.Kind != lexer.RightParen {
				if recoverId = p.expect(lexer.Comma, "expected ',' after receiver"); recoverId >= 0 {
					return
				}
			}
		}

		// Parameters
		myRecoverId := p.pushRecoverPoint(lexer.RightParen)
		f.Params, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightParen, p.parseFuncParam)
		p.popRecoverPoint()

		for i, param := range f.Params {
			if param.Name.Token.Kind == lexer.DotDotDot && i != len(f.Params)-1 {
				p.diagnostics = append(p.diagnostics, core.Diagnostic{
					Kind:    core.Error,
					Path:    p.path,
					Range:   param.Name.Range(),
					Message: "var args '...' needs to be the last parameter",
				})
			}
		}

		for {
			i := slices.IndexFunc(f.Params, func(n *ast.NameType) bool {
				return n.Name.Token.Kind == lexer.DotDotDot
			})

			if i == -1 {
				break
			}

			f.Params = append(f.Params[:i], f.Params[i+1:]...)
			f.VarArgs = true
		}

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}

		// ')'
		if recoverId = p.expect(lexer.RightParen, "expected ')' after function parameters"); recoverId >= 0 {
			return
		}
	}

	// Returns
	if p.current.Kind != lexer.LeftBrace {
		myRecoverId := p.pushRecoverPoint(lexer.LeftBrace)
		f.Returns, recoverId = p.parseType()
		p.popRecoverPoint()

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}
	} else {
		f.Returns = &ast.PrimitiveType{Kind: types.Void}
	}

	// Body
	if p.current.Kind == lexer.LeftBrace {
		if f.Body, recoverId = p.parseStmt(); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseReceiver() (r *ast.Receiver, recoverId int) {
	r = &ast.Receiver{}
	r.Range_.Start = p.current.Range.Start
	defer func() {
		r.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'mut'
	if p.current.Kind == lexer.Mut {
		p.advance()
		r.Mutable = true
	}

	// 'self'
	if p.current.Kind != lexer.Identifier || p.current.Text != "self" {
		recoverId = p.error("expected 'self'")
		return
	}

	p.advance()
	return
}

func (p *parser) parseFuncParam() (*ast.NameType, int) {
	if p.current.Kind == lexer.DotDotDot {
		n := &ast.NameType{}
		n.Name = &ast.Leaf{Token: p.advance()}
		n.Range_ = p.previous.Range
		return n, -1
	}

	return p.parseNameType()
}
