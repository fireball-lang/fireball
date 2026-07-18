package lsp

import (
	"cmp"
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/project"
	"math"
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
		hi.VisitDecl(decl)
	}

	// Stripped
	for _, range_ := range file.Ast.Stripped {
		hi.AddRange(range_, commentKind)
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
	commentKind
)

type semantic struct {
	line   uint16
	column uint16

	length uint16
	kind   semanticKind
}

func newSemantic(line, column, length uint32, kind semanticKind) semantic {
	return semantic{
		line:   uint16(line) - 1,
		column: uint16(column),
		length: uint16(length),
		kind:   kind,
	}
}

func (hi *highlighter) AddFull(node ast.Node, kind semanticKind) {
	if hi.fullSemanticTokens {
		hi.Add(node, kind)
	}
}

func (hi *highlighter) Add(node ast.Node, kind semanticKind) {
	if !core.IsNil(node) {
		hi.AddRange(node.Range(), kind)
	}
}

func (hi *highlighter) AddRange(range_ core.Range, kind semanticKind) {
	startL := range_.Start.Line
	endL := range_.End.Line

	for line := startL; line <= endL; line++ {
		var column uint32
		var length uint32

		if line == startL {
			column = range_.Start.Column - 1

			if startL == endL {
				length = range_.End.Column - range_.Start.Column
			} else {
				length = hi.file.LineTable[startL-1] - (range_.Start.Column - 1)
			}
		} else if line == endL {
			column = 0
			length = range_.End.Column
		} else {
			column = 0
			length = hi.file.LineTable[line-1]
		}

		length = min(length, math.MaxUint16)
		hi.tokens = append(hi.tokens, newSemantic(line, column, length, kind))
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
	lastColumn := uint16(0)

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
