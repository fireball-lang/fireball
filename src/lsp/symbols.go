package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/project"
	"fireball/types"

	"github.com/fireball-lang/protocol"
	"go.lsp.dev/uri"
)

type symbol struct {
	file *project.File
	kind protocol.SymbolKind

	name   string
	detail string

	range_         core.Range
	selectionRange core.Range
}

type symbolConsumer interface {
	add(symbol symbol) int
	addChild(parent int, child symbol)

	supportsDetail() bool
}

func (s *Server) DocumentSymbol(_ context.Context, params *protocol.DocumentSymbolParams) (result []interface{}, err error) {
	// Get file
	file, locker := s.getFile(params.TextDocument.URI.Filename())
	if file == nil {
		return nil, nil
	}

	// Lock file
	locker.Lock()
	defer locker.Unlock()

	// Get symbols
	symbols := documentSymbolConsumer{}
	getSymbols(&symbols, []*project.File{file})

	return symbols.symbols, nil
}

func (s *Server) Symbols(_ context.Context, _ *protocol.WorkspaceSymbolParams) (result []protocol.SymbolInformation, err error) {
	symbols := workspaceSymbolConsumer{}

	for _, workspace := range s.workspaces {
		var files []*project.File

		workspace.mutex.RLock()

		for _, proj := range workspace.projMap {
			files = append(files, proj.Files...)
		}

		workspace.mutex.RUnlock()

		getSymbols(&symbols, files)
	}

	return symbols.symbols, nil
}

func getSymbols(symbols symbolConsumer, files []*project.File) {
	decls := make(map[types.Type]int)

	for _, file := range files {
		for _, decl := range file.Ast.Decls {
			switch decl := decl.(type) {
			case *ast.Struct:
				typ, ok := file.NodeTypes[decl]
				if !ok {
					continue
				}

				id := symbols.add(symbol{
					file:           file,
					kind:           protocol.SymbolKindStruct,
					name:           getText(decl.Name_),
					range_:         getRange(decl),
					selectionRange: getRange(decl.Name_),
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

				decls[typ] = id

			case *ast.Enum:
				typ, ok := file.NodeTypes[decl]
				if !ok {
					continue
				}

				t := typ.(*types.Enum)

				id := symbols.add(symbol{
					file:           file,
					kind:           protocol.SymbolKindEnum,
					name:           getText(decl.Name_),
					detail:         typ.String(),
					range_:         getRange(decl),
					selectionRange: getRange(decl.Name_),
				})

				for i, c := range decl.Cases {
					symbols.addChild(id, symbol{
						file:           file,
						kind:           protocol.SymbolKindEnumMember,
						name:           c.Name.Token.Text,
						detail:         t.Cases[i].Value.String(),
						range_:         getRange(c),
						selectionRange: getRange(c.Name),
					})
				}

				decls[typ] = id

			case *ast.Interface:
				typ, ok := file.NodeTypes[decl]
				if !ok {
					continue
				}

				id := symbols.add(symbol{
					file:           file,
					kind:           protocol.SymbolKindInterface,
					name:           getText(decl.Name_),
					range_:         getRange(decl),
					selectionRange: getRange(decl.Name_),
				})

				for _, associatedType := range decl.AssociatedTypes {
					symbols.addChild(id, symbol{
						file:           file,
						kind:           protocol.SymbolKindTypeParameter,
						name:           getText(associatedType.Name),
						range_:         getRange(associatedType),
						selectionRange: getRange(associatedType.Name),
					})
				}

				for _, method := range decl.Methods {
					addFuncSymbol(symbols, file, method, id)
				}

				decls[typ] = id
			}
		}
	}

	for _, file := range files {
		for _, decl := range file.Ast.Decls {
			switch decl := decl.(type) {
			case *ast.GlobalVar:
				symbols.add(symbol{
					file:           file,
					kind:           protocol.SymbolKindVariable,
					name:           getText(decl.Name()),
					detail:         getTypeString(decl.Type),
					range_:         getRange(decl),
					selectionRange: getRange(decl.Name()),
				})

			case *ast.Impl:
				if core.IsNil(decl.Type) {
					continue
				}

				typ, ok := file.NodeTypes[decl]
				if !ok {
					continue
				}

				if s, ok := typ.(*types.Struct); ok && s.Generic != nil {
					typ = s.Generic
				}

				id, ok := decls[typ]

				if !ok {
					id = symbols.add(symbol{
						file: file,
						kind: protocol.SymbolKindStruct,
						name: typ.String(),
					})

					decls[typ] = id
				}

				for _, associatedType := range decl.AssociatedTypes {
					symbols.addChild(id, symbol{
						file:           file,
						kind:           protocol.SymbolKindTypeParameter,
						name:           getText(associatedType.Name),
						detail:         getTypeString(associatedType.Type),
						range_:         getRange(associatedType),
						selectionRange: getRange(associatedType.Name),
					})
				}

				for _, method := range decl.Methods {
					addFuncSymbol(symbols, file, method, id)
				}

			case *ast.Func:
				addFuncSymbol(symbols, file, decl, -1)
			}
		}
	}
}

func addFuncSymbol(symbols symbolConsumer, file *project.File, f *ast.Func, parent int) {
	detail := ""

	if symbols.supportsDetail() {
		detail = f.String(true)
	}

	sym := symbol{
		file:           file,
		kind:           protocol.SymbolKindFunction,
		name:           getText(f.Name_),
		detail:         detail,
		range_:         getRange(f),
		selectionRange: getRange(f.Name_),
	}

	if parent != -1 {
		sym.kind = protocol.SymbolKindMethod
		symbols.addChild(parent, sym)
	} else {
		symbols.add(sym)
	}
}

func getText(leaf *ast.Leaf) string {
	if leaf != nil {
		return leaf.Token.Text
	}

	return ""
}

func getTypeString(type_ ast.Type) string {
	if !core.IsNil(type_) {
		return type_.String()
	}

	return ""
}

func getRange(node ast.Node) core.Range {
	if !core.IsNil(node) {
		return node.Range()
	}

	return core.Range{}
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
		Range:          toLspRange(symbol.range_),
		SelectionRange: toLspRange(symbol.selectionRange),
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
			URI:   uri.New(symbol.file.Path),
			Range: toLspRange(symbol.range_),
		},
		ContainerName: containerName,
	}
}
