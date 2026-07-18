package parser

import (
	"fireball/ast"
	"fireball/lexer"
)

func (p *parser) parseFile() (f *ast.File) {
	f = &ast.File{}
	f.Path = p.path
	f.Range_.Start = p.current.Range.Start
	defer func() {
		f.Range_.End = p.previous.Range.End
	}()

	recoverId := -1
	p.pushRecoverPoint(lexer.Hashtag, lexer.Mod, lexer.Import, lexer.Pub, lexer.Struct, lexer.Enum, lexer.Interface, lexer.Impl, lexer.Var, lexer.Func)

	// Attributes

	if p.current.Kind == lexer.Hashtag {
		f.Attributes_, _ = p.parseAttributes()
	}

	// Mod

	if f.Mod, recoverId = p.parseMod(); recoverId >= 0 && p.current.Kind == lexer.EOF {
		return
	}

	// Imports / Declarations

	for p.current.Kind != lexer.EOF {
		var attributes []ast.Attribute

		if p.current.Kind == lexer.Hashtag {
			attributes, _ = p.parseAttributes()
		}

		if p.current.Kind == lexer.Import {
			i, _ := p.parseImport()
			i.Attributes_ = attributes

			f.Imports = append(f.Imports, i)
		} else {
			decl, _ := p.parseDecl(attributes)
			f.Decls = append(f.Decls, decl)
		}
	}

	return
}

func (p *parser) parseMod() (m *ast.Mod, recoverId int) {
	m = &ast.Mod{}
	m.Range_.Start = p.current.Range.Start
	defer func() {
		m.Range_.End = p.previous.Range.End
	}()

	// 'mod'
	if recoverId = p.expect(lexer.Mod, "expected 'mod'"); recoverId >= 0 {
		m.Path = &ast.IdentifierPath{}
		m.Path.Range_ = p.current.Range
		return
	}

	// Path
	if m.Path, recoverId = p.parseIdentifierPath(false); recoverId >= 0 {
		return
	}

	// ';'
	if recoverId = p.expect(lexer.Semicolon, "expected ';' after module"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseImport() (i *ast.Import, recoverId int) {
	i = &ast.Import{}
	i.Range_.Start = p.current.Range.Start
	defer func() {
		i.Range_.End = p.previous.Range.End
	}()

	// 'import'
	if recoverId = p.expect(lexer.Import, "expected 'import'"); recoverId >= 0 {
		return
	}

	// Path
	if i.Path, recoverId = p.parseIdentifierPath(true); recoverId >= 0 {
		return
	}

	// '::' '{' Symbols '}'
	symbols := false

	if p.current.Kind == lexer.ColonColon {
		symbols = true

		// '::'
		if recoverId = p.expect(lexer.ColonColon, "expected '::' before import symbols"); recoverId >= 0 {
			return
		}

		// '{'
		if recoverId = p.expect(lexer.LeftBrace, "expected '{' before import symbols"); recoverId >= 0 {
			return
		}

		// Symbols
		myRecoverId := p.pushRecoverPoint(lexer.RightBrace)
		i.Symbols, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightBrace, p.parseLeaf)
		p.popRecoverPoint()

		if recoverId >= 0 {
			if recoverId == myRecoverId {
				recoverId = -1
			} else {
				return
			}
		}

		// '}'
		if recoverId = p.expect(lexer.RightBrace, "expected '}' before import symbols"); recoverId >= 0 {
			return
		}
	}

	// 'as' Alias
	if !symbols && p.current.Kind == lexer.As {
		// 'as'
		if recoverId = p.expect(lexer.As, "expected 'as' before module alias"); recoverId >= 0 {
			return
		}

		// Alias
		if i.Alias, recoverId = p.parseLeaf(); recoverId >= 0 {
			return
		}
	}

	// ';'
	if recoverId = p.expect(lexer.Semicolon, "expected ';' after import"); recoverId >= 0 {
		return
	}

	return
}
