package lsp

import (
	"context"
	"fireball/ast"
	"fireball/lexer"
	"fireball/project"
	"github.com/MineGame159/protocol"
	"go.lsp.dev/uri"
)

type symbol struct {
	file *project.File
	kind protocol.SymbolKind

	name   string
	detail string

	range_         lexer.Range
	selectionRange lexer.Range
}

type symbolConsumer interface {
	add(symbol symbol) int
	addChild(parent int, child symbol)

	supportsDetail() bool
}

func (s *server) DocumentSymbol(_ context.Context, params *protocol.DocumentSymbolParams) (result []interface{}, err error) {
	defer stop(start(s, "DocumentSymbol"))

	// Get file
	file := s.getFile(params.TextDocument.URI)
	if file == nil {
		return nil, nil
	}

	// Compute
	s.astMutex.RLock()
	defer s.astMutex.RUnlock()

	symbols := documentSymbolConsumer{}
	getSymbols(&symbols, []*project.File{file})

	return symbols.symbols, nil
}

func (s *server) Symbols(_ context.Context, params *protocol.WorkspaceSymbolParams) (result []protocol.SymbolInformation, err error) {
	defer stop(start(s, "Symbols"))

	// Compute
	s.astMutex.RLock()
	defer s.astMutex.RUnlock()

	symbols := workspaceSymbolConsumer{}

	for _, proj := range s.projects {
		var files []*project.File

		for file := range proj.Files() {
			files = append(files, file)
		}

		getSymbols(&symbols, files)
	}

	return symbols.symbols, nil
}

func getSymbols(symbols symbolConsumer, files []*project.File) {
	structs := make(map[*ast.Struct]int)

	for _, file := range files {
		for _, decl := range file.Ast().Decls {
			if decl, ok := decl.(*ast.Struct); ok {
				id := symbols.add(symbol{
					file:           file,
					kind:           protocol.SymbolKindStruct,
					name:           getText(decl.NameN),
					range_:         getRange(decl),
					selectionRange: getRange(decl.NameN),
				})

				for _, field := range decl.Fields {
					symbols.addChild(id, symbol{
						file:           file,
						kind:           protocol.SymbolKindField,
						name:           getText(field.Name),
						detail:         getTypeString(field.Type),
						range_:         getRange(field),
						selectionRange: getRange(field.Name),
					})
				}

				structs[decl] = id
			}
		}
	}

	for _, file := range files {
		for _, decl := range file.Ast().Decls {
			switch decl := decl.(type) {
			case *ast.Impl:
				if decl.Struct == nil {
					continue
				}

				id, ok := structs[decl.Struct]

				if !ok {
					id = symbols.add(symbol{
						file: file,
						kind: protocol.SymbolKindStruct,
						name: getText(decl.Struct.NameN),
					})

					structs[decl.Struct] = id
				}

				for _, method := range decl.Methods {
					detail := ""

					if symbols.supportsDetail() {
						detail = method.StringWithParamNames()
					}

					symbols.addChild(id, symbol{
						file:           file,
						kind:           protocol.SymbolKindMethod,
						name:           getText(method.NameN),
						detail:         detail,
						range_:         getRange(method),
						selectionRange: getRange(method.NameN),
					})
				}

			case *ast.GlobalVar:
				symbols.add(symbol{
					file:           file,
					kind:           protocol.SymbolKindVariable,
					name:           getText(decl.NameN),
					detail:         getTypeString(decl.Type),
					range_:         getRange(decl),
					selectionRange: getRange(decl.NameN),
				})

			case *ast.Func:
				detail := ""

				if symbols.supportsDetail() {
					detail = decl.StringWithParamNames()
				}

				symbols.add(symbol{
					file:           file,
					kind:           protocol.SymbolKindFunction,
					name:           getText(decl.NameN),
					detail:         detail,
					range_:         getRange(decl),
					selectionRange: getRange(decl.NameN),
				})
			}
		}
	}
}

func getText(leaf *ast.Leaf) string {
	if leaf != nil {
		return leaf.Token.Text
	}

	return ""
}

func getTypeString(type_ ast.Type) string {
	if ast.IsValid(type_) {
		return type_.String()
	}

	return ""
}

func getRange(node ast.Node) lexer.Range {
	if ast.IsValid(node) {
		return node.Range()
	}

	var empty lexer.Range
	return empty
}

// Document symbols

type documentSymbolConsumer struct {
	symbols []any
}

func (d *documentSymbolConsumer) add(symbol symbol) int {
	if symbol.name == "" {
		return -1
	}

	d.symbols = append(d.symbols, d.convert(symbol))
	return len(d.symbols) - 1
}

func (d *documentSymbolConsumer) addChild(parent int, child symbol) {
	if parent == -1 || child.name == "" {
		return
	}

	symbol := d.symbols[parent].(protocol.DocumentSymbol)
	symbol.Children = append(symbol.Children, d.convert(child))

	d.symbols[parent] = symbol
}

func (d *documentSymbolConsumer) supportsDetail() bool {
	return true
}

func (d *documentSymbolConsumer) convert(symbol symbol) protocol.DocumentSymbol {
	return protocol.DocumentSymbol{
		Name:           symbol.name,
		Detail:         symbol.detail,
		Kind:           symbol.kind,
		Range:          rangeToProtocol(symbol.range_),
		SelectionRange: rangeToProtocol(symbol.selectionRange),
	}
}

// Workspace symbols

type workspaceSymbolConsumer struct {
	symbols []protocol.SymbolInformation
}

func (w *workspaceSymbolConsumer) add(symbol symbol) int {
	if symbol.name == "" {
		return -1
	}

	w.symbols = append(w.symbols, w.convert(symbol, -1))
	return len(w.symbols) - 1
}

func (w *workspaceSymbolConsumer) addChild(parent int, child symbol) {
	if parent == -1 || child.name == "" {
		return
	}

	w.symbols = append(w.symbols, w.convert(child, parent))
}

func (w *workspaceSymbolConsumer) supportsDetail() bool {
	return false
}

func (w *workspaceSymbolConsumer) convert(symbol symbol, parent int) protocol.SymbolInformation {
	containerName := ""

	if parent >= 0 {
		containerName = w.symbols[parent].Name
	}

	return protocol.SymbolInformation{
		Name:       symbol.name,
		Kind:       symbol.kind,
		Tags:       nil,
		Deprecated: false,
		Location: protocol.Location{
			URI:   uri.New(symbol.file.Provider().AbsolutePath()),
			Range: rangeToProtocol(symbol.range_),
		},
		ContainerName: containerName,
	}
}
