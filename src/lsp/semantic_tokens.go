package lsp

import (
	"cmp"
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/project"
	"slices"

	"github.com/fireball-lang/protocol"
)

func (s *Server) SemanticTokensFull(_ context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	// Get file
	file, locker := s.getFile(uriPath(params.TextDocument.URI))
	if file == nil {
		return nil, nil
	}

	// Lock file
	locker.Lock()
	defer locker.Unlock()

	// Highlighter
	hi := highlighter{
		fullSemanticTokens: s.fullSemanticTokens,
		file:               file,
	}

	// Mod
	for _, entry := range file.Ast.Mod.Path.Entries {
		hi.AddFull(entry, namespaceKind)
	}

	// Imports
	for _, im := range file.Ast.Imports {
		for _, entry := range im.Path.Entries {
			hi.AddFull(entry, namespaceKind)
		}

		for _, symbol := range im.Symbols {
			hi.AddType(symbol, file.NodeTypes[symbol])
		}
	}

	// Declarations
	for _, decl := range file.Ast.Decls {
		ast.VisitDecl(&hi, decl)
	}

	return &protocol.SemanticTokens{Data: hi.Data()}, nil
}

// Highlighter

type highlighter struct {
	fullSemanticTokens bool
	file               *project.File

	tokens []semantic
}

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
	keywordKind
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

func (hi *highlighter) AddFull(node ast.Node, kind semanticKind) {
	if hi.fullSemanticTokens {
		hi.Add(node, kind)
	}
}

func (hi *highlighter) Add(node ast.Node, kind semanticKind) {
	if !core.IsNil(node) && node.Range().Start.Column < 256 {
		range_ := node.Range()
		hi.tokens = append(hi.tokens, newSemantic(range_.Start.Line, range_.Start.Column-1, range_.End.Column-range_.Start.Column, kind))
	}
}

func (hi *highlighter) Data() []uint32 {
	// Sort tokens
	slices.SortFunc(hi.tokens, func(a, b semantic) int {
		if a.line == b.line {
			return cmp.Compare(a.column, b.column)
		}

		if a.line < b.line {
			return -1
		}

		return 1
	})

	// Get data
	data := make([]uint32, len(hi.tokens)*5)

	lastLine := uint16(0)
	lastColumn := uint8(0)

	for i, token := range hi.tokens {
		if lastLine != token.line {
			lastColumn = 0
		}

		j := i * 5

		data[j+0] = uint32(token.line - lastLine)
		data[j+1] = uint32(token.column - lastColumn)
		data[j+2] = uint32(token.length)
		data[j+3] = uint32(token.kind)
		data[j+4] = 0

		lastLine = token.line
		lastColumn = token.column
	}

	return data
}
