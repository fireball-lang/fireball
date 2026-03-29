package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"io"
)

type parser struct {
	lexer *lexer.Lexer

	path        string
	diagnostics []core.Diagnostic

	previous lexer.Token
	current  lexer.Token
	next     lexer.Token

	recoverTokenKinds []lexer.TokenKind
	recoverFrames     []int
}

func Parse(reader io.Reader, path string) ([]ast.Decl, []core.Diagnostic) {
	p := parser{
		lexer: lexer.New(reader),
		path:  path,
	}

	p.advance()
	p.advance()

	var decls []ast.Decl

	p.pushRecoverPoint(lexer.Struct, lexer.Func)

	for p.current.Kind != lexer.EOF {
		decl, _ := p.parseDecl()
		decls = append(decls, decl)
	}

	return decls, p.diagnostics
}

// Utils

func (p *parser) expectFunc(predicate func(kind lexer.TokenKind) bool, msg string) int {
	if !predicate(p.current.Kind) {
		return p.error(msg)
	}

	p.advance()
	return -1
}

func (p *parser) expect(kind lexer.TokenKind, msg string) int {
	if p.current.Kind != kind {
		return p.error(msg)
	}

	p.advance()
	return -1
}

func (p *parser) advance() lexer.Token {
	p.previous = p.current
	p.current = p.next
	p.next = p.lexer.Next()

	return p.previous
}

func (p *parser) pushRecoverPoint(kinds ...lexer.TokenKind) int {
	frameId := len(p.recoverFrames)

	p.recoverFrames = append(p.recoverFrames, len(p.recoverTokenKinds))
	p.recoverTokenKinds = append(p.recoverTokenKinds, kinds...)

	return frameId
}

func (p *parser) popRecoverPoint() {
	lastIndex := len(p.recoverFrames) - 1
	frameStart := p.recoverFrames[lastIndex]

	p.recoverTokenKinds = p.recoverTokenKinds[:frameStart]
	p.recoverFrames = p.recoverFrames[:lastIndex]
}

func (p *parser) error(msg string) int {
	// Report error
	p.reportError(p.current.Range, msg)

	// Synchronize
	for p.current.Kind != lexer.EOF {
		for i := len(p.recoverFrames) - 1; i >= 0; i-- {
			start := p.recoverFrames[i]
			end := len(p.recoverTokenKinds)

			if i < len(p.recoverFrames)-1 {
				end = p.recoverFrames[i+1]
			}

			for j := start; j < end; j++ {
				if p.current.Kind == p.recoverTokenKinds[j] {
					return i
				}
			}
		}

		p.advance()
	}

	return 0
}

func (p *parser) reportError(range_ core.Range, msg string) {
	p.diagnostics = append(p.diagnostics, core.Diagnostic{
		Kind:    core.Error,
		Path:    p.path,
		Range:   range_,
		Message: msg,
	})
}
