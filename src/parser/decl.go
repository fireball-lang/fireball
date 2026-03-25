package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/types"
	"slices"
)

func (p *parser) parseDecl() (ast.Decl, int) {
	switch p.current.Kind {
	case lexer.Struct:
		return p.parseStruct()
	case lexer.Func:
		return p.parseFunc()

	default:
		b := &ast.BadDecl{}
		b.Range_ = p.current.Range
		return b, p.error("expected declaration")
	}
}

func (p *parser) parseStruct() (s *ast.Struct, recoverId int) {
	s = &ast.Struct{}
	s.Range_.Start = p.current.Range.Start
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

func (p *parser) parseFunc() (f *ast.Func, recoverId int) {
	f = &ast.Func{}
	f.Range_.Start = p.current.Range.Start
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

func (p *parser) parseFuncParam() (*ast.NameType, int) {
	if p.current.Kind == lexer.DotDotDot {
		n := &ast.NameType{}
		n.Name = &ast.Leaf{Token: p.advance()}
		n.Range_ = p.previous.Range
		return n, -1
	}

	return p.parseNameType()
}
