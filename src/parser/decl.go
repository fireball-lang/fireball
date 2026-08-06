package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/types"
	"slices"
)

func (p *parser) parseDecl(documentation []*ast.Leaf, attributes []ast.Attribute) (decl ast.Decl, recoverId int) {
	public := false
	var publicToken lexer.Token

	if p.current.Kind == lexer.Pub {
		publicToken = p.advance()
		public = true
	}

	switch p.current.Kind {
	case lexer.Struct:
		return p.parseStruct(documentation, attributes, public)
	case lexer.Enum:
		return p.parseEnum(documentation, attributes, public)
	case lexer.Interface:
		return p.parseInterface(documentation, attributes, public)
	case lexer.Impl:
		if public {
			p.reportError(publicToken.Range, "implementation blocks cannot be marked as public")
		}
		return p.parseImpl(documentation, attributes)

	case lexer.Var:
		return p.parseGlobalVar(documentation, attributes, public)
	case lexer.Func:
		return p.parseFunc(documentation, attributes, public, false)

	default:
		b := &ast.BadDecl{}
		b.Range_ = p.current.Range
		return b, p.error("expected declaration")
	}
}

func (p *parser) parseStruct(documentation []*ast.Leaf, attributes []ast.Attribute, public bool) (s *ast.Struct, recoverId int) {
	s = &ast.Struct{}
	s.Range_.Start = p.current.Range.Start
	s.Documentation_ = documentation
	s.Attributes_ = attributes
	s.Public = public
	defer func() {
		s.Range_.End = p.previous.Range.End

		if s.Name_ == nil {
			s.Name_ = p.badLeaf()
		}
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

	// '[' Type Parameters ']'
	if p.current.Kind == lexer.LeftBracket {
		if s.TypeParams, recoverId = p.parseTypeParams(); recoverId >= 0 {
			return
		}
	}

	// '{' Fields '}'
	{
		// '{'
		if recoverId = p.expect(lexer.LeftBrace, "expected '{' before struct fields"); recoverId >= 0 {
			return
		}

		// Fields
		myRecoverId := p.pushRecoverPoint(lexer.RightBrace)
		s.Fields, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightBrace, p.parseField)
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

func (p *parser) parseField() (f *ast.Field, recoverId int) {
	f = &ast.Field{}
	f.Range_.Start = p.current.Range.Start
	defer func() {
		f.Range_.End = p.previous.Range.End

		if core.IsNil(f.Type) {
			f.Type = p.badType()
		}
	}()

	recoverId = -1

	// Documentation
	if p.current.Kind == lexer.Documentation {
		f.Documentation = p.parseDocumentation()
	}

	// 'pub'
	if p.current.Kind == lexer.Pub {
		f.Public = true
		p.advance()
	}

	// Name
	if f.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// ':'
	if recoverId = p.expect(lexer.Colon, "expected ':' before type"); recoverId >= 0 {
		return
	}

	// Type
	if f.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseEnum(documentation []*ast.Leaf, attributes []ast.Attribute, public bool) (e *ast.Enum, recoverId int) {
	e = &ast.Enum{}
	e.Range_.Start = p.current.Range.Start
	e.Documentation_ = documentation
	e.Attributes_ = attributes
	e.Public = public
	defer func() {
		e.Range_.End = p.previous.Range.End

		if e.Name_ == nil {
			e.Name_ = p.badLeaf()
		}
	}()

	recoverId = -1

	// 'enum'
	if recoverId = p.expect(lexer.Enum, "expected 'enum'"); recoverId >= 0 {
		return
	}

	// Name
	if e.Name_, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// ':' Type
	if p.current.Kind == lexer.Colon {
		// ':'
		p.advance()

		// Type
		if e.Type, recoverId = p.parseType(); recoverId >= 0 {
			return
		}
	}

	// '{'
	if recoverId = p.expect(lexer.LeftBrace, "expected '{' before enum cases"); recoverId >= 0 {
		return
	}

	// Cases
	myRecoverId := p.pushRecoverPoint(lexer.RightBrace)
	e.Cases, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightBrace, p.parseCase)
	p.popRecoverPoint()

	if recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			return
		}
	}

	// '}'
	if recoverId = p.expect(lexer.RightBrace, "expected '}' after enum cases"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseCase() (c *ast.Case, recoverId int) {
	c = &ast.Case{}
	c.Range_.Start = p.current.Range.Start
	defer func() {
		c.Range_.End = p.previous.Range.End

		if c.Name == nil {
			c.Name = p.badLeaf()
		}
	}()

	recoverId = -1

	// Documentation
	if p.current.Kind == lexer.Documentation {
		c.Documentation = p.parseDocumentation()
	}

	// Name
	if c.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// '=' Value
	if p.current.Kind == lexer.Equal {
		// '='
		p.advance()

		// Value
		switch p.current.Kind {
		case lexer.BinaryInteger, lexer.HexInteger, lexer.UnsignedInteger, lexer.SignedInteger:
			c.Value = &ast.Leaf{Token: p.advance()}
		default:
			recoverId = p.error("expected an integer constant")
		}
	}

	return
}

func (p *parser) parseInterface(documentation []*ast.Leaf, attributes []ast.Attribute, public bool) (i *ast.Interface, recoverId int) {
	i = &ast.Interface{}
	i.Range_.Start = p.current.Range.Start
	i.Documentation_ = documentation
	i.Attributes_ = attributes
	i.Public = public
	defer func() {
		i.Range_.End = p.previous.Range.End

		if i.Name_ == nil {
			i.Name_ = p.badLeaf()
		}
	}()

	recoverId = -1

	// 'interface'
	if recoverId = p.expect(lexer.Interface, "expected 'interface'"); recoverId >= 0 {
		return
	}

	// Name
	if i.Name_, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// '[' Type Parameters ']'
	if p.current.Kind == lexer.LeftBracket {
		if i.TypeParams, recoverId = p.parseTypeParams(); recoverId >= 0 {
			return
		}
	}

	// '{'
	if recoverId = p.expect(lexer.LeftBrace, "expected '{' before members"); recoverId >= 0 {
		return
	}

	// Methods
	for p.current.Kind != lexer.RightBrace && p.current.Kind != lexer.EOF {
		myRecoverId := p.pushRecoverPoint(lexer.RightBrace, lexer.Documentation, lexer.Type, lexer.Func)

		var documentation []*ast.Leaf

		if p.current.Kind == lexer.Documentation {
			documentation = p.parseDocumentation()
		}

		if p.current.Kind == lexer.Type {
			// Associate type
			var a *ast.AssociatedType
			a, recoverId = p.parseAssociatedType(documentation, false)
			i.AssociatedTypes = append(i.AssociatedTypes, a)
		} else if p.current.Kind == lexer.Func {
			// Method
			var f *ast.Func
			f, recoverId = p.parseFunc(documentation, nil, true, true)
			i.Methods = append(i.Methods, f)
		} else {
			// Invalid
			recoverId = p.error("expected 'type' or 'func'")
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

	// '}'
	if recoverId = p.expect(lexer.RightBrace, "expected '}' after members"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseImpl(documentation []*ast.Leaf, attributes []ast.Attribute) (i *ast.Impl, recoverId int) {
	i = &ast.Impl{}
	i.Range_.Start = p.current.Range.Start
	i.Documentation_ = documentation
	i.Attributes_ = attributes
	defer func() {
		i.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'impl'
	if recoverId = p.expect(lexer.Impl, "expected 'impl'"); recoverId >= 0 {
		return
	}

	// '[' Type Parameters ']'
	if p.current.Kind == lexer.LeftBracket {
		if i.TypeParams, recoverId = p.parseTypeParams(); recoverId >= 0 {
			return
		}
	}

	// Type
	if i.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	// (':' Interface)?
	if p.current.Kind == lexer.Colon {
		p.advance()

		if i.Interface, recoverId = p.parseNonPrimitiveIdentifierType(false); recoverId >= 0 {
			return
		}
	}

	// '{'
	if recoverId = p.expect(lexer.LeftBrace, "expected '{' before members"); recoverId >= 0 {
		return
	}

	// Methods
	for p.current.Kind != lexer.RightBrace && p.current.Kind != lexer.EOF {
		myRecoverId := p.pushRecoverPoint(lexer.RightBrace, lexer.Documentation, lexer.Type, lexer.Pub, lexer.Func)

		var documentation []*ast.Leaf

		if p.current.Kind == lexer.Documentation {
			documentation = p.parseDocumentation()
		}

		if p.current.Kind == lexer.Type {
			// Associate type
			var a *ast.AssociatedType
			a, recoverId = p.parseAssociatedType(documentation, true)
			i.AssociatedTypes = append(i.AssociatedTypes, a)
		} else if p.current.Kind == lexer.Pub || p.current.Kind == lexer.Func {
			// Method
			public := false

			if p.current.Kind == lexer.Pub {
				p.advance()
				public = true
			}

			var f *ast.Func
			f, recoverId = p.parseFunc(documentation, nil, public, true)
			i.Methods = append(i.Methods, f)
		} else {
			// Invalid
			recoverId = p.error("expected 'type', 'pub' or 'func'")
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

	// '}'
	if recoverId = p.expect(lexer.RightBrace, "expected '}' after members"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseAssociatedType(documentation []*ast.Leaf, hasType bool) (a *ast.AssociatedType, recoverId int) {
	a = &ast.AssociatedType{}
	a.Range_.Start = p.current.Range.Start
	a.Documentation = documentation
	a.Name = emptyLeaf
	defer func() {
		a.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'type'
	if recoverId = p.expect(lexer.Type, "expected 'type'"); recoverId >= 0 {
		return
	}

	// Name
	if a.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// '=' Type
	if hasType {
		// '='
		if recoverId = p.expect(lexer.Equal, "expected '=' before type"); recoverId >= 0 {
			return
		}

		// Type
		if a.Type, recoverId = p.parseType(); recoverId >= 0 {
			return
		}
	}

	// ';'
	if recoverId = p.expect(lexer.Semicolon, "expected ';'"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseGlobalVar(documentation []*ast.Leaf, attributes []ast.Attribute, public bool) (g *ast.GlobalVar, recoverId int) {
	g = &ast.GlobalVar{}
	g.Range_.Start = p.current.Range.Start
	g.Documentation_ = documentation
	g.Attributes_ = attributes
	g.Public = public
	g.Name_ = emptyLeaf
	defer func() {
		g.Range_.End = p.previous.Range.End

		if core.IsNil(g.Type) {
			g.Type = p.badType()
		}
	}()

	recoverId = -1

	// 'var'
	if recoverId = p.expect(lexer.Var, "expected 'var'"); recoverId >= 0 {
		return
	}

	// Name
	if g.Name_, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// ':'
	if recoverId = p.expect(lexer.Colon, "expected ':' before type"); recoverId >= 0 {
		return
	}

	// Type
	if g.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	// ';'
	if recoverId = p.expect(lexer.Semicolon, "expected ';' after global variable"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseFunc(documentation []*ast.Leaf, attributes []ast.Attribute, public bool, allowReceiver bool) (f *ast.Func, recoverId int) {
	f = &ast.Func{}
	f.Range_.Start = p.current.Range.Start
	f.Documentation_ = documentation
	f.Attributes_ = attributes
	f.Public = public
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

	// '[' Type Parameters ']'
	if p.current.Kind == lexer.LeftBracket {
		if f.TypeParams, recoverId = p.parseTypeParams(); recoverId >= 0 {
			return
		}
	}

	// Signature
	if f.Receiver, f.Params, f.VarArgs, f.Returns, recoverId = p.parseFuncSignature(allowReceiver, true); recoverId >= 0 {
		return
	}

	// Body
	if p.current.Kind == lexer.LeftBrace {
		if f.Body, recoverId = p.parseStmt(); recoverId >= 0 {
			return
		}
	} else {
		if recoverId = p.expect(lexer.Semicolon, "expected ';' after a function with no body"); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseFuncSignature(allowReceiver, paramNameRequired bool) (receiver *ast.Receiver, params []*ast.Param, varArgs bool, returns ast.Type, recoverId int) {
	defer func() {
		if core.IsNil(returns) {
			returns = p.badType()
		}
	}()

	// '(' Parameters ')'
	{
		// '('
		if recoverId = p.expect(lexer.LeftParen, "expected '(' before function parameters"); recoverId >= 0 {
			return
		}

		// Receiver
		if allowReceiver && (p.current.Kind == lexer.Mut || (p.current.Kind == lexer.Identifier && p.current.Text == "self")) {
			if receiver, recoverId = p.parseReceiver(); recoverId >= 0 {
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
		params, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightParen, func() (*ast.Param, int) {
			return p.parseFuncParam(paramNameRequired)
		})
		p.popRecoverPoint()

		for i, param := range params {
			if param.Name != nil && param.Name.Token.Kind == lexer.DotDotDot && i != len(params)-1 {
				p.diagnostics = append(p.diagnostics, core.Diagnostic{
					Kind:    core.Error,
					Path:    p.path,
					Range:   param.Name.Range(),
					Message: "var args '...' needs to be the last parameter",
				})
			}
		}

		for {
			i := slices.IndexFunc(params, func(n *ast.Param) bool {
				return n.Name != nil && n.Name.Token.Kind == lexer.DotDotDot
			})

			if i == -1 {
				break
			}

			params = append(params[:i], params[i+1:]...)
			varArgs = true
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
	if p.canStartType() {
		myRecoverId := p.pushRecoverPoint(lexer.LeftBrace)
		returns, recoverId = p.parseType()
		p.popRecoverPoint()

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}
	} else {
		returns = &ast.PrimitiveType{Kind: types.Void}
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

func (p *parser) parseFuncParam(nameRequired bool) (param *ast.Param, recoverId int) {
	if p.current.Kind == lexer.DotDotDot {
		param = &ast.Param{}
		param.Name = &ast.Leaf{Token: p.advance()}
		param.Range_ = p.previous.Range

		recoverId = -1
		return
	}

	param = &ast.Param{}
	param.Range_.Start = p.current.Range.Start
	defer func() {
		param.Range_.End = p.previous.Range.End

		if core.IsNil(param.Type) {
			param.Type = p.badType()
		}
	}()

	recoverId = -1

	if nameRequired || (p.current.Kind == lexer.Identifier && p.next.Kind == lexer.Colon) {
		// Name
		if param.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
			return
		}

		// ':'
		if recoverId = p.expect(lexer.Colon, "expected ':' before type"); recoverId >= 0 {
			return
		}
	}

	// Type
	if param.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseTypeParams() (params []*ast.TypeParam, recoverId int) {
	// '['
	if recoverId = p.expect(lexer.LeftBracket, "expected '[' before type parameters"); recoverId >= 0 {
		return
	}

	// Type Parameters
	myRecoverId := p.pushRecoverPoint(lexer.RightBracket)
	params, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightBracket, p.parseTypeParam)
	p.popRecoverPoint()

	if recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			return
		}
	}

	// ']'
	if recoverId = p.expect(lexer.RightBracket, "expected ']' after type parameters"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseTypeParam() (param *ast.TypeParam, recoverId int) {
	param = &ast.TypeParam{}
	param.Range_.Start = p.current.Range.Start
	defer func() {
		param.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// Name
	if param.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// ':' Types
	if p.current.Kind == lexer.Colon {
		// ':'
		p.advance()

		// Type
		var constraint ast.Type

		constraint, recoverId = p.parseType()
		param.Constraints = append(param.Constraints, constraint)
		if recoverId >= 0 {
			return
		}

		for p.current.Kind == lexer.Plus {
			// '+'
			p.advance()

			// Type
			constraint, recoverId = p.parseType()
			param.Constraints = append(param.Constraints, constraint)
			if recoverId >= 0 {
				return
			}
		}
	}

	return
}
