package lsp

import (
	"cmp"
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/project"
	"fireball/symbols"
	"fireball/types"
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
	hi := highlighter{file: file}

	// Imports
	for _, i := range file.Ast.Imports {
		for _, symbol := range i.Symbols {
			hi.addType(symbol, file.NodeTypes[symbol])
		}
	}

	// Declarations
	for _, decl := range file.Ast.Decls {
		hi.visit(decl)
	}

	return &protocol.SemanticTokens{Data: hi.data()}, nil
}

// Highlighter

type highlighter struct {
	file   *project.File
	tokens []semantic
}

func (hi *highlighter) addType(node ast.Node, typ types.Type) {
	switch typ.(type) {
	case *types.Struct:
		hi.add(node, classKind)
	case *types.Func:
		hi.add(node, functionKind)
	}
}

func (hi *highlighter) visit(node ast.Node) {
	switch node := node.(type) {
	case *ast.IdentifierType:
		if len(node.Path.Entries) > 0 {
			last := node.Path.Entries[len(node.Path.Entries)-1]
			hi.addType(last, hi.file.NodeTypes[node])
		}

	case *ast.Identifier:
		if len(node.Path.Entries) > 0 {
			// Entries before last one
			for _, entry := range node.Path.Entries[:len(node.Path.Entries)-1] {
				if typ, ok := hi.file.NodeTypes[entry]; ok {
					hi.addType(entry, typ)
				}
			}

			// Last entry
			if info, ok := hi.file.ExprInfos[node]; ok {
				entry := node.Path.Entries[len(node.Path.Entries)-1]

				switch info.Symbol {
				case symbols.Invalid:

				case symbols.Struct:
					hi.add(entry, classKind)

				case symbols.Func:
					hi.add(entry, functionKind)

				case symbols.Param:
					kind := parameterKind

					if len(node.Path.Entries) == 1 && entry.Token.Text == "self" {
						if f := ast.GetClosestParent[*ast.Func](node); f != nil && f.Receiver != nil {
							kind = keywordKind
						}
					}

					hi.add(entry, kind)

				case symbols.Var:
					hi.add(entry, variableKind)
				}
			}
		}

	default:
		for child := range node.Children() {
			hi.visit(child)
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

func (hi *highlighter) add(node ast.Node, kind semanticKind) {
	if !core.IsNil(node) && node.Range().Start.Column < 256 {
		range_ := node.Range()
		hi.tokens = append(hi.tokens, newSemantic(range_.Start.Line, range_.Start.Column-1, range_.End.Column-range_.Start.Column, kind))
	}
}

func (hi *highlighter) data() []uint32 {
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
