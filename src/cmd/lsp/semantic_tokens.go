package lsp

import (
	"cmp"
	"context"
	"fireball/analyzer"
	"fireball/ast"
	"github.com/MineGame159/protocol"
	"slices"
)

func (s *server) SemanticTokensFull(_ context.Context, params *protocol.SemanticTokensParams) (result *protocol.SemanticTokens, err error) {
	defer stop(start(s, "SemanticTokensFull"))

	// Get file
	file := s.getFile(params.TextDocument.URI)
	if file == nil {
		return nil, nil
	}

	// Compute
	s.astMutex.RLock()
	defer s.astMutex.RUnlock()

	h := highlighter{}
	h.visit(file.Ast())

	return &protocol.SemanticTokens{
		Data: h.data(),
	}, nil
}

func (s *server) SemanticTokensRefresh(_ context.Context) (err error) {
	return nil
}

// Highlighter

type highlighter struct {
	variables analyzer.VariableTracker[bool]
	tokens    []semantic
}

// Declarations

func (h *highlighter) VisitStruct(f *ast.Struct) {
	h.add(f.NameN, classKind)

	for _, field := range f.Fields {
		h.add(field.Name, propertyKind)
		h.visit(field.Type)
	}
}

func (h *highlighter) VisitFunc(f *ast.Func) {
	h.variables.PushScope()

	h.add(f.NameN, functionKind)

	for _, param := range f.Params {
		h.variables.Add(param.Name.Token.Text, param.Type, true)

		h.add(param.Name, parameterKind)
	}

	h.visitChildren(f)

	h.variables.PopScope()
}

// Expressions

func (h *highlighter) VisitBlock(b *ast.Block) {
	h.variables.PushScope()
	h.visitChildren(b)
	h.variables.PopScope()
}

func (h *highlighter) VisitVar(v *ast.Var) {
	h.variables.Add(v.Name.Token.Text, v.ActualType(), false)

	h.add(v.Name, variableKind)

	h.visitChildren(v)
}

func (h *highlighter) VisitIf(i *ast.If) {
	h.visitChildren(i)
}

func (h *highlighter) VisitWhile(w *ast.While) {
	h.visitChildren(w)
}

func (h *highlighter) VisitBreak(b *ast.Break) {
	h.visitChildren(b)
}

func (h *highlighter) VisitContinue(c *ast.Continue) {
	h.visitChildren(c)
}

func (h *highlighter) VisitReturn(r *ast.Return) {
	h.visitChildren(r)
}

func (h *highlighter) VisitLiteral(l *ast.Literal) {
}

func (h *highlighter) VisitParen(p *ast.Paren) {
	h.visitChildren(p)
}

func (h *highlighter) VisitIdentifier(i *ast.Identifier) {
	kind := variableKind

	switch i.Result().Type.(type) {
	case *ast.Func:
		kind = functionKind

	default:
		if v, param := h.variables.Find(i.Name.Token.Text); v != nil && param {
			kind = parameterKind
		}
	}

	h.add(i.Name, kind)
}

func (h *highlighter) VisitCall(c *ast.Call) {
	h.visitChildren(c)
}

func (h *highlighter) VisitIndex(i *ast.Index) {
	h.visitChildren(i)
}

func (h *highlighter) VisitMember(m *ast.Member) {
	h.visit(m.Value)
	h.add(m.Name, propertyKind)
}

func (h *highlighter) VisitUnary(u *ast.Unary) {
	h.visitChildren(u)
}

func (h *highlighter) VisitBinary(b *ast.Binary) {
	h.visitChildren(b)
}

func (h *highlighter) VisitCast(c *ast.Cast) {
	h.visitChildren(c)
}

// Visit

func (h *highlighter) visitChildren(node ast.Node) {
	for child := range node.Children() {
		h.visit(child)
	}
}

func (h *highlighter) visit(node ast.Node) {
	switch node := node.(type) {
	case ast.Decl:
		node.Visit(h)
	case ast.Expr:
		node.Visit(h)

	case *ast.PrimitiveType:
		h.add(node, typeKind)
	case *ast.DeclType:
		h.add(node.Name, classKind)

	default:
		for child := range node.Children() {
			h.visit(child)
		}
	}
}

// Tokens

type semanticKind uint8

const (
	functionKind semanticKind = iota
	parameterKind
	variableKind
	typeKind
	classKind
	enumKind
	propertyKind
	enumMemberKind
	namespaceKind
	interfaceKind
	genericKind
)

type semantic struct {
	line   uint16
	column uint8

	length uint8
	kind   semanticKind
}

func newSemantic(line, column, length uint32, kind semanticKind) semantic {
	return semantic{
		line:   uint16(line) - 1,
		column: uint8(column),
		length: uint8(length),
		kind:   kind,
	}
}

func (h *highlighter) add(node ast.Node, kind semanticKind) {
	if ast.IsValid(node) && node.Range().Start.Column < 256 {
		range_ := node.Range()
		h.tokens = append(h.tokens, newSemantic(range_.Start.Line, range_.Start.Column, range_.End.Column-range_.Start.Column, kind))
	}
}

func (h *highlighter) data() []uint32 {
	// Sort tokens
	slices.SortFunc(h.tokens, func(a, b semantic) int {
		if a.line == b.line {
			return cmp.Compare(a.column, b.column)
		}

		if a.line < b.line {
			return -1
		}

		return 1
	})

	// Get data
	data := make([]uint32, len(h.tokens)*5)

	lastLine := uint16(0)
	lastColumn := uint8(0)

	for i, token := range h.tokens {
		if lastLine != token.line {
			lastColumn = 0
		}

		j := i * 5

		data[j+0] = uint32(token.line - lastLine)
		data[j+1] = uint32(token.column - lastColumn)
		data[j+2] = uint32(token.length)
		data[j+3] = uint32(token.kind)
		data[j+4] = uint32(0)

		lastLine = token.line
		lastColumn = token.column
	}

	return data
}
