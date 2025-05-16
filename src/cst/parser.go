package cst

import (
	"fireball/lexer"
	"fireball/utils"
)

type parser struct {
	l *lexer.Lexer

	current lexer.Token

	diagnostics []utils.Diagnostic
}

func Parse(source string) (Node, []utils.Diagnostic) {
	p := parser{
		l: lexer.NewLexer(source),
	}

	p.advance()
	node := p.file()

	calculateRange(&node)

	return node, p.diagnostics
}

func calculateRange(node *Node) {
	for i := range node.Children {
		calculateRange(&node.Children[i])
	}

	node.calculateRange()
}

func (p *parser) file() Node {
	node := Node{Kind: File}

	lastErr := false

	for {
		if p.current.Kind == lexer.Eof {
			break
		}

		if p.current.Text == "func" {
			child, err := p.funcNode()
			node.Children = append(node.Children, child)

			if err {
				p.advance()
				lastErr = true
			} else {
				lastErr = false
			}
		} else {
			if !lastErr {
				p.error("Invalid '" + p.current.Text + "'")
			} else {
				p.advance()
			}

			lastErr = true
		}
	}

	return node
}

// Utils

func (p *parser) appendAdvance(node *Node, kind lexer.TokenKind, errorMsg string) bool {
	if p.current.Kind != kind {
		p.error(errorMsg)
		return true
	}

	node.append(p.advance())
	return false
}

func (p *parser) error(message string) {
	p.diagnostics = append(p.diagnostics, utils.Diagnostic{
		Kind:    utils.Error,
		Message: message,
		Range:   p.current.Range,
	})

	p.advance()
}

func (p *parser) advance() Node {
	leaf := Node{
		Kind:  Leaf,
		Token: p.current,
		Range: p.current.Range,
	}

	p.current = p.l.Next()

	return leaf
}
