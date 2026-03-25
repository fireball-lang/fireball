package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
)

func (p *parser) parseStmt() (ast.Stmt, int) {
	switch p.current.Kind {
	case lexer.LeftBrace:
		return p.parseBlock()

	case lexer.Var:
		return p.parseVar()
	case lexer.If:
		return p.parseIf()
	case lexer.While:
		return p.parseWhile()
	case lexer.For:
		return p.parseFor()

	case lexer.Return:
		return p.parseReturn()
	case lexer.Break:
		return p.parseBreak()
	case lexer.Continue:
		return p.parseContinue()

	default:
		return p.parseExpression()
	}
}

func (p *parser) parseBlock() (b *ast.Block, recoverId int) {
	b = &ast.Block{}
	b.Range_.Start = p.current.Range.Start
	defer func() {
		b.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// '{'
	if recoverId = p.expect(lexer.LeftBrace, "expected '{' before block"); recoverId >= 0 {
		return
	}

	// Statements
	for p.current.Kind != lexer.RightBrace && p.current.Kind != lexer.EOF {
		myRecoverId := p.pushRecoverPoint(lexer.RightBrace, lexer.Semicolon, lexer.Var, lexer.If, lexer.While, lexer.For, lexer.Return, lexer.Break, lexer.Continue)

		var stmt ast.Stmt
		stmt, recoverId = p.parseStmt()
		b.Stmts = append(b.Stmts, stmt)

		if recoverId == -1 {
			if needsSemicolon(stmt) {
				recoverId = p.expect(lexer.Semicolon, "expected ';' after statement")
			}
		} else if p.current.Kind == lexer.Semicolon {
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

	// '}'
	if recoverId = p.expect(lexer.RightBrace, "expected '}' after block"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseExpression() (e *ast.Expression, recoverId int) {
	e = &ast.Expression{}
	e.Range_.Start = p.current.Range.Start
	defer func() {
		e.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// Expression
	if e.Expr, recoverId = p.parseExpr(); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseVar() (v *ast.Var, recoverId int) {
	v = &ast.Var{}
	v.Range_.Start = p.current.Range.Start
	v.Name = emptyLeaf
	defer func() {
		v.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'var'
	if recoverId = p.expect(lexer.Var, "expected 'var' before variable name"); recoverId >= 0 {
		return
	}

	// Name
	if v.Name, recoverId = p.parseLeaf(); recoverId >= 0 {
		return
	}

	// (':' Type)?
	if p.current.Kind == lexer.Colon {
		p.advance()

		if v.Type, recoverId = p.parseType(); recoverId >= 0 {
			return
		}
	}

	// '=' Initializer
	if p.current.Kind != lexer.Semicolon {
		if recoverId = p.expect(lexer.Equal, "expected '=' before variable initializer"); recoverId >= 0 {
			return
		}

		if v.Initializer, recoverId = p.parseExpr(); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseIf() (i *ast.If, recoverId int) {
	i = &ast.If{}
	i.Range_.Start = p.current.Range.Start
	defer func() {
		i.Range_.End = p.previous.Range.End

		if core.IsNil(i.Condition) {
			i.Condition = p.badExpr()
		}
		if core.IsNil(i.BranchTrue) {
			i.BranchTrue = p.badStmt()
		}
	}()

	recoverId = -1

	// 'if'
	if recoverId = p.expect(lexer.If, "expected 'if' before condition"); recoverId >= 0 {
		return
	}

	// '(' Condition ')'
	if i.Condition, recoverId = parseParenWrapped(p, p.parseExpr); recoverId >= 0 {
		return
	}

	// Branch true
	if i.BranchTrue, recoverId = p.parseStmt(); recoverId >= 0 {
		return
	}

	// 'else' Branch false
	if p.current.Kind == lexer.Else {
		p.advance()

		if i.BranchFalse, recoverId = p.parseStmt(); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseWhile() (w *ast.While, recoverId int) {
	w = &ast.While{}
	w.Range_.Start = p.current.Range.Start
	defer func() {
		w.Range_.End = p.previous.Range.End

		if core.IsNil(w.Condition) {
			w.Condition = p.badExpr()
		}
		if core.IsNil(w.Body) {
			w.Body = p.badStmt()
		}
	}()

	recoverId = -1

	// 'while'
	if recoverId = p.expect(lexer.While, "expected 'while' before condition"); recoverId >= 0 {
		return
	}

	// '(' Condition ')'
	if w.Condition, recoverId = parseParenWrapped(p, p.parseExpr); recoverId >= 0 {
		return
	}

	// Body
	if w.Body, recoverId = p.parseStmt(); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseFor() (f *ast.For, recoverId int) {
	f = &ast.For{}
	f.Range_.Start = p.current.Range.Start
	defer func() {
		f.Range_.End = p.previous.Range.End

		if core.IsNil(f.Body) {
			f.Body = p.badStmt()
		}
	}()

	recoverId = -1

	// 'for'
	if recoverId = p.expect(lexer.For, "expected 'for' before clauses"); recoverId >= 0 {
		return
	}

	// '(' Clauses ')'
	var clauses forClauses
	clauses, recoverId = parseParenWrapped(p, p.parseForClauses)

	f.Initializer = clauses.initializer
	f.Condition = clauses.condition
	f.Increment = clauses.increment

	if recoverId >= 0 {
		return
	}

	// Body
	if f.Body, recoverId = p.parseStmt(); recoverId >= 0 {
		return
	}

	return
}

type forClauses struct {
	initializer ast.Stmt
	condition   ast.Expr
	increment   ast.Expr
}

func (p *parser) parseForClauses() (c forClauses, recoverId int) {
	recoverId = -1

	// Initializer? ';'
	if p.current.Kind != lexer.Semicolon {
		if c.initializer, recoverId = p.parseStmt(); recoverId >= 0 {
			return
		}
	}

	if recoverId = p.expect(lexer.Semicolon, "expected ';' after initializer"); recoverId >= 0 {
		return
	}

	// Condition? ';'
	if p.current.Kind != lexer.Semicolon {
		if c.condition, recoverId = p.parseExpr(); recoverId >= 0 {
			return
		}
	}

	if recoverId = p.expect(lexer.Semicolon, "expected ';' after condition"); recoverId >= 0 {
		return
	}

	// Increment?
	if p.current.Kind != lexer.RightParen {
		if c.increment, recoverId = p.parseExpr(); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseReturn() (r *ast.Return, recoverId int) {
	r = &ast.Return{}
	r.Range_.Start = p.current.Range.Start
	defer func() {
		r.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'return'
	if recoverId = p.expect(lexer.Return, "expected 'return'"); recoverId >= 0 {
		return
	}

	// Value?
	if p.current.Kind != lexer.Semicolon {
		if r.Value, recoverId = p.parseExpr(); recoverId >= 0 {
			return
		}
	}

	return
}

func (p *parser) parseBreak() (b *ast.Break, recoverId int) {
	b = &ast.Break{}
	b.Range_.Start = p.current.Range.Start
	defer func() {
		b.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'break'
	if recoverId = p.expect(lexer.Break, "expected 'break'"); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseContinue() (c *ast.Continue, recoverId int) {
	c = &ast.Continue{}
	c.Range_.Start = p.current.Range.Start
	defer func() {
		c.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'continue'
	if recoverId = p.expect(lexer.Continue, "expected 'continue'"); recoverId >= 0 {
		return
	}

	return
}

// Helpers

func needsSemicolon(stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.Block:
		return false
	case *ast.Expression:
		return true

	case *ast.Var:
		return true
	case *ast.If:
		return false
	case *ast.While:
		return false
	case *ast.For:
		return false

	case *ast.Return:
		return true
	case *ast.Break:
		return true
	case *ast.Continue:
		return true

	case *ast.BadStmt:
		return false

	default:
		panic("parser.needsSemicolon() - Invalid type")
	}
}
