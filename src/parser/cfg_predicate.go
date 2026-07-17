package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
)

func (p *parser) parseCfgPredicate() (ast.CfgPredicate, int) {
	if p.current.Kind != lexer.Identifier {
		b := &ast.BadCfg{}
		b.Range_ = p.current.Range
		return b, p.error("expected cfg predicate")
	}

	switch p.current.Text {
	case "target_os":
		return p.parseTargetOsCfg()
	case "target_family":
		return p.parseTargetFamilyCfg()

	case "not":
		return p.parseNotCfg()
	case "all":
		return p.parseAllCfg()
	case "any":
		return p.parseAnyCfg()

	default:
		b := &ast.BadCfg{}
		b.Range_ = p.current.Range

		p.reportError(p.current.Range, "unknown cfg predicate '"+p.current.Text+"'")
		p.advance()

		return b, p.error("")
	}
}

func (p *parser) parseTargetOsCfg() (t *ast.TargetOsCfg, recoverId int) {
	t = &ast.TargetOsCfg{}
	t.Range_.Start = p.current.Range.Start
	defer func() {
		t.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'target_os'
	if recoverId = p.expect(lexer.Identifier, "expected 'target_os'"); recoverId >= 0 {
		return
	}

	// '!='
	if p.current.Kind == lexer.BangEqual {
		p.advance()
		t.Not = true
	} else {
		// '='
		if recoverId = p.expect(lexer.Equal, "expected '='"); recoverId >= 0 {
			return
		}
	}

	// Kind
	if recoverId = p.expect(lexer.String, "expected a string"); recoverId >= 0 {
		return
	}

	switch p.previous.Text[1 : len(p.previous.Text)-1] {
	case "windows":
		t.Kind = ast.WindowsOs
	case "linux":
		t.Kind = ast.Linux
	case "macos":
		t.Kind = ast.MacOS

	default:
		p.reportError(p.previous.Range, "invalid 'target_os' value, expected 'windows', 'linux' or 'macos'")
	}

	return
}

func (p *parser) parseTargetFamilyCfg() (t *ast.TargetFamilyCfg, recoverId int) {
	t = &ast.TargetFamilyCfg{}
	t.Range_.Start = p.current.Range.Start
	defer func() {
		t.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'target_family'
	if recoverId = p.expect(lexer.Identifier, "expected 'target_family'"); recoverId >= 0 {
		return
	}

	// '!='
	if p.current.Kind == lexer.BangEqual {
		p.advance()
		t.Not = true
	} else {
		// '='
		if recoverId = p.expect(lexer.Equal, "expected '='"); recoverId >= 0 {
			return
		}
	}

	// Kind
	if recoverId = p.expect(lexer.String, "expected a string"); recoverId >= 0 {
		return
	}

	switch p.previous.Text[1 : len(p.previous.Text)-1] {
	case "windows":
		t.Kind = ast.WindowsFamily
	case "unix":
		t.Kind = ast.Unix

	default:
		p.reportError(p.previous.Range, "invalid 'target_family' value, expected 'windows' or 'unix'")
	}

	return
}

func (p *parser) parseNotCfg() (n *ast.NotCfg, recoverId int) {
	n = &ast.NotCfg{}
	n.Range_.Start = p.current.Range.Start
	defer func() {
		n.Range_.End = p.previous.Range.End

		if core.IsNil(n.Predicate) {
			n.Predicate = &ast.BadCfg{}
		}
	}()

	recoverId = -1

	// 'not'
	if recoverId = p.expect(lexer.Identifier, "expected 'not'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before cfg predicate"); recoverId >= 0 {
		return
	}

	// Predicate
	if n.Predicate, recoverId = p.parseCfgPredicate(); recoverId >= 0 {
		return
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after cfg predicate"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseAllCfg() (a *ast.AllCfg, recoverId int) {
	a = &ast.AllCfg{}
	a.Range_.Start = p.current.Range.Start
	defer func() {
		a.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'all'
	if recoverId = p.expect(lexer.Identifier, "expected 'all'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before cfg predicates"); recoverId >= 0 {
		return
	}

	// Predicates
	myRecoverId := p.pushRecoverPoint(lexer.RightParen)
	a.Predicates, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightParen, p.parseCfgPredicate)
	p.popRecoverPoint()

	if recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			return
		}
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after cfg predicates"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseAnyCfg() (a *ast.AnyCfg, recoverId int) {
	a = &ast.AnyCfg{}
	a.Range_.Start = p.current.Range.Start
	defer func() {
		a.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'any'
	if recoverId = p.expect(lexer.Identifier, "expected 'any'"); recoverId >= 0 {
		return
	}

	// '('
	if recoverId = p.expect(lexer.LeftParen, "expected '(' before cfg predicates"); recoverId >= 0 {
		return
	}

	// Predicates
	myRecoverId := p.pushRecoverPoint(lexer.RightParen)
	a.Predicates, recoverId = parseCommaList(p, lexer.Identifier, lexer.RightParen, p.parseCfgPredicate)
	p.popRecoverPoint()

	if recoverId >= 0 {
		if recoverId == myRecoverId {
			recoverId = -1
		} else {
			return
		}
	}

	// ')'
	if recoverId = p.expect(lexer.RightParen, "expected ')' after cfg predicates"); recoverId >= 0 {
		return
	}

	return
}
