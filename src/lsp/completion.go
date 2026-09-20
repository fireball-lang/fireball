package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/project"
	"fireball/sema"
	"fireball/symbols"
	"fireball/types"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/fireball-lang/protocol"
)

type completions struct {
	s *Server

	workspace *Workspace
	file      *project.File

	root symbols.Scope

	cursor core.Pos
	node   ast.Node

	exprContext bool

	seen  map[string]struct{}
	items []protocol.CompletionItem
}

func (s *Server) Completion(_ context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	// Get file
	file, locker := s.getFile(params.TextDocument.URI.Filename())
	if file == nil {
		return nil, nil
	}

	// Lock file
	locker.Lock()
	defer locker.Unlock()

	// Get node at cursor
	cursor := snapCursorLeft(file, toCorePos(params.Position))
	node := ast.GetNodeAtPos(file.Ast, cursor)

	// Construct completions
	workspace := s.getWorkspace(file)

	c := completions{
		s:         s,
		workspace: workspace,
		file:      file,
		root:      file.Proj.GetRootScope(workspace.depMap),
		cursor:    cursor,
		node:      node,
		seen:      make(map[string]struct{}),
	}

	// Cursor based context completions
	if !core.IsNil(node) {
		// Import context completions
		if imp := ast.GetClosestParent[*ast.Import](node); imp != nil {
			c.Import(imp)
			return c.Build(), nil
		}

		// Type context completions
		if c.TypeContext() {
			return c.Build(), nil
		}

		// Field list completions
		if c.FieldList() {
			return c.Build(), nil
		}

		// Name contexts
		if c.NameContext() {
			return c.Build(), nil
		}

		// Identifier path completions
		if id := ast.GetClosestParent[*ast.Identifier](node); id != nil && len(id.Path) >= 2 && cursor.GreaterThan(id.Path[len(id.Path)-2].Range().End) {
			c.exprContext = !sema.WantsFunction(c.file.NodeTypes, c.file.ExprInfos, id)
			c.Identifier(id)
			return c.Build(), nil
		}

		// Member expression completions
		if m := ast.GetClosestParent[*ast.Member](node); m != nil && !core.IsNil(m.Expr) && cursor.GreaterThan(m.Expr.Range().End) {
			c.exprContext = !sema.WantsFunction(c.file.NodeTypes, c.file.ExprInfos, m)
			c.Member(m)
			return c.Build(), nil
		}

		// Expression context inside a function body
		c.exprContext = ast.GetClosestParent[*ast.Block](node) != nil

		// Single-segment identifiers: the call/reference decision applies here too
		if id := ast.GetClosestParent[*ast.Identifier](node); id != nil {
			c.exprContext = c.exprContext && !sema.WantsFunction(c.file.NodeTypes, c.file.ExprInfos, id)
		}
	}

	// Get top level module completions
	c.RootModule()

	// Get imported completions
	for _, imp := range file.Ast.Imports {
		c.Imported(imp)
	}

	// Get file declaration completions
	for _, decl := range file.Ast.Decls {
		c.Decl(file, decl, 0)
	}

	// Cursor inside a declaration
	if !core.IsNil(node) {
		c.EnclosingDecl(ast.GetClosestParent[ast.Decl](node))
	}

	// Expression snippets
	if c.exprContext {
		c.ExprSnippets()
	}

	// Return completions
	return c.Build(), nil
}

// Import

func (c *completions) Import(imp *ast.Import) {
	// Error recovery can leave leaves with empty tokens in the path and symbol list
	path := validLeaves(imp.Path)
	imported := validLeaves(imp.Symbols)

	// Alias zone ('import path as |')
	if imp.Alias != nil && imp.Alias.Token.Text != "" && imp.Alias.Range().Contains(c.cursor) {
		return
	}

	// Symbol import zone ('import path:: { | }')
	if len(path) > 0 && len(imported) > 0 && c.cursor.GreaterThan(path[len(path)-1].Range().End) {
		c.ImportSymbol(path)
		return
	}

	// Path segment: count leaves that end before the cursor, so the resolved prefix
	// is 'path[:segment]' and the segment being typed comes right after it
	segment := 0

	for _, leaf := range path {
		if !c.cursor.GreaterThan(leaf.Range().End) {
			break
		}

		segment++
	}

	// Root modules
	if segment == 0 {
		c.RootModule()
		return
	}

	// Submodules of the resolved prefix
	scope, ok := sema.GetScope(c.root, path[:segment])
	if !ok {
		return
	}

	module, ok := scope.(*project.Module)
	if !ok {
		return
	}

	for _, child := range module.Children {
		c.Add(protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindModule,
			Label:  child.Name,
			Detail: moduleDetail(child),
		})
	}
}

func (c *completions) ImportSymbol(path []*ast.Leaf) {
	// Resolve module scope
	scope, ok := sema.GetScope(c.root, path)
	if !ok {
		return
	}

	module, ok := scope.(*project.Module)
	if !ok {
		return
	}

	// Private symbols can only be imported from the file's own module
	sameModule := slices.EqualFunc(path, c.file.Ast.Mod.Path, func(a, b *ast.Leaf) bool {
		return a.Token.Text == b.Token.Text
	})

	// Skip already imported symbols
	c.ModuleSymbol(module, sameModule, false)
}

// Imported

func (c *completions) Imported(imp *ast.Import) {
	// Module
	if len(imp.Symbols) == 0 {
		c.ImportedModule(imp)
		return
	}

	// Symbols
	c.ImportedSymbols(imp)
}

func (c *completions) ImportedModule(imp *ast.Import) {
	name := imp.Path[len(imp.Path)-1].Token.Text

	if imp.Alias != nil {
		name = imp.Alias.Token.Text
	}

	var detail strings.Builder

	for i, name := range validLeaves(imp.Path) {
		if i > 0 {
			detail.WriteString("::")
		}

		detail.WriteString(name.Token.Text)
	}

	c.Add(protocol.CompletionItem{
		Kind:       protocol.CompletionItemKindModule,
		Label:      name,
		InsertText: name + "::",
		Detail:     detail.String(),
	})
}

func (c *completions) ImportedSymbols(imp *ast.Import) {
	importScope, ok := sema.GetScope(c.root, imp.Path)
	if !ok {
		return
	}

	for _, name := range imp.Symbols {
		symbol, ok := importScope.GetSymbol(symbols.Type|symbols.Variable|symbols.Function, name.Token.Text)
		if !ok {
			continue
		}

		if decl, ok := symbol.Node.(ast.Decl); ok {
			file, locker := c.s.getFile(ast.GetFile(decl).Path)
			if file == nil {
				continue
			}

			locker.Lock()
			c.Decl(file, decl, 0)
			locker.Unlock()
		}
	}
}

// Module

func (c *completions) RootModule() {
	// Current project
	c.Add(protocol.CompletionItem{
		Kind:   protocol.CompletionItemKindModule,
		Label:  c.file.Proj.Config.Name,
		Detail: c.file.Proj.Config.Name,
	})

	// Dependencies
	coreDep := project.Dependency{Path: "core"}

	for _, dep := range c.file.Proj.Config.Dependencies {
		if dep == coreDep {
			continue
		}

		proj, ok := c.workspace.depMap[dep]
		if !ok {
			continue
		}

		c.Add(protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindModule,
			Label:  proj.Config.Name,
			Detail: proj.Config.Name,
		})
	}

	if proj, ok := c.workspace.depMap[coreDep]; ok {
		c.Add(protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindModule,
			Label:  proj.Config.Name,
			Detail: proj.Config.Name,
		})
	}
}

func (c *completions) ModuleSymbol(module *project.Module, sameModule, typesOnly bool) {
	for _, moduleFile := range module.Files {
		for _, symbol := range moduleFile.Symbols {
			// Visibility
			if !symbol.Public && !sameModule {
				continue
			}

			// Types only
			if typesOnly {
				switch symbol.Kind {
				case symbols.TypeAlias, symbols.Struct, symbols.Enum, symbols.Interface:
				default:
					continue
				}
			}

			// Declaration
			decl, ok := symbol.Node.(ast.Decl)
			if !ok {
				continue
			}

			declFile, declLocker := c.s.getFile(ast.GetFile(decl).Path)
			if declFile == nil {
				continue
			}

			declLocker.Lock()
			c.Decl(declFile, decl, 0)
			declLocker.Unlock()
		}
	}
}

// Identifier expression

func (c *completions) Identifier(id *ast.Identifier) {
	// Prefix path
	entries := id.Path[:len(id.Path)-1]
	prefix := make([]*ast.Leaf, 0, len(entries))

	for _, entry := range entries {
		prefix = append(prefix, entry.Name)
	}

	// Module scope
	if scope, ok := sema.GetScope(c.root, prefix); ok {
		if module, ok := scope.(*project.Module); ok {
			c.NamespaceModule(module, prefix, false)
			return
		}
	}

	// Type scope
	if typ, ok := c.file.NodeTypes[entries[len(entries)-1]]; ok {
		c.TypeMember(typ)
	}
}

func (c *completions) NamespaceModule(module *project.Module, prefix []*ast.Leaf, typesOnly bool) {
	// Private symbols can only be accessed from the file's own module
	sameModule := slices.EqualFunc(prefix, c.file.Ast.Mod.Path, func(a, b *ast.Leaf) bool {
		return a.Token.Text == b.Token.Text
	})

	// Submodules
	for _, child := range module.Children {
		c.Add(protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindModule,
			Label:  child.Name,
			Detail: moduleDetail(child),
		})
	}

	// Symbols
	c.ModuleSymbol(module, sameModule, typesOnly)
}

// Types ('var x: Poi|')

func (c *completions) TypeContext() bool {
	for n := c.node; !core.IsNil(n); n = n.Parent() {
		// Type position
		if typ, ok := n.(ast.Type); ok {
			c.Types(typ)
			return true
		}

		// Expression position
		if _, ok := n.(ast.Expr); ok {
			return false
		}

		// Empty type slots on declarations
		if typ, name := typeSlot(n); typ != nil && c.emptyTypeSlot(typ, name, n.Range().Start) {
			c.Types(nil)
			return true
		}
	}

	return false
}

// Name contexts

func (c *completions) NameContext() bool {
	// Declaration keyword before the cursor ('var |', 'func fo|o', 'mod |')
	if c.afterDeclKeyword() {
		return true
	}

	for n := c.node; !core.IsNil(n); n = n.Parent() {
		// Type / expression positions are handled by their own contexts
		if _, ok := n.(ast.Type); ok {
			return false
		}

		if _, ok := n.(ast.Expr); ok {
			return false
		}

		if fn, ok := n.(*ast.Func); ok && c.funcName(fn) {
			// Declaration name: no completions
			return true
		}
	}

	return false
}

func (c *completions) afterDeclKeyword() bool {
	source, ok := c.file.Source.(*Source)
	if !ok {
		return false
	}

	line, col, ok := sourcePos(source, c.cursor)
	if !ok {
		return false
	}

	text := source.lines[line]

	isIdent := func(b byte) bool {
		return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
	}

	isSpace := func(b byte) bool {
		return b == ' ' || b == '\t' || b == '\r' || b == '\n'
	}

	// Word being completed
	for col > 0 && isIdent(text[col-1]) {
		col--
	}

	// Whitespace before the word, crossing lines upward
	for {
		for col > 0 && isSpace(text[col-1]) {
			col--
		}

		if col > 0 || line == 0 {
			break
		}

		line--
		text = source.lines[line]
		col = len(text)
	}

	if col == 0 {
		return false
	}

	// The identifier before the whitespace decides
	end := col

	for col > 0 && isIdent(text[col-1]) {
		col--
	}

	switch string(text[col:end]) {
	case "var", "const", "global", "func", "struct", "enum", "interface", "impl", "type", "mod", "pub":
		return true
	}

	return false
}

func (c *completions) funcName(fn *ast.Func) bool {
	name := fn.Name_

	// Untyped name: anywhere after the keyword / receiver
	if name == nil || name.Token.Text == "" {
		bound := fn.Range().Start

		if fn.Receiver != nil {
			bound = fn.Receiver.Range().End
		}

		return c.cursor.GreaterThan(bound)
	}

	// Formed name: cursor within the name
	return !c.cursor.GreaterThan(name.Range().End)
}

func (c *completions) Types(typ ast.Type) {
	// Type path ('module::Type')
	if id, ok := typ.(*ast.IdentifierType); ok && len(id.Path) >= 2 && c.cursor.GreaterThan(id.Path[len(id.Path)-2].Range().End) {
		entries := id.Path[:len(id.Path)-1]

		prefix := make([]*ast.Leaf, 0, len(entries))
		for _, entry := range entries {
			prefix = append(prefix, entry.Name)
		}

		c.TypePath(prefix)
		return
	}

	// File type declarations
	for _, decl := range c.file.Ast.Decls {
		switch decl.(type) {
		case *ast.TypeAlias, *ast.Struct, *ast.Enum, *ast.Interface:
			c.Decl(c.file, decl, 0)
		}
	}

	// Imported types
	for _, imp := range c.file.Ast.Imports {
		path := validLeaves(imp.Path)

		scope, ok := sema.GetScope(c.root, path)
		if !ok {
			continue
		}

		module, ok := scope.(*project.Module)
		if !ok {
			continue
		}

		sameModule := slices.EqualFunc(path, c.file.Ast.Mod.Path, func(a, b *ast.Leaf) bool {
			return a.Token.Text == b.Token.Text
		})

		c.ModuleSymbol(module, sameModule, true)
	}

	// Primitives
	for i := range types.PrimitiveKindCount {
		kind := types.PrimitiveKind(i)

		if kind != types.Void {
			c.Add(protocol.CompletionItem{
				Kind:  protocol.CompletionItemKindClass,
				Label: kind.String(),
			})
		}
	}

	// Types in scope (type parameters, 'Self')
	c.scopeTypes(ast.GetClosestParent[ast.Decl](c.node))
}

func (c *completions) TypePath(prefix []*ast.Leaf) {
	scope, ok := sema.GetScope(c.root, prefix)
	if !ok {
		return
	}

	module, ok := scope.(*project.Module)
	if !ok {
		return
	}

	c.NamespaceModule(module, prefix, true)
}

func (c *completions) scopeTypes(decl ast.Decl) {
	if core.IsNil(decl) {
		return
	}

	switch decl := decl.(type) {
	case *ast.Struct:
		c.TypeParams(decl.TypeParams)

	case *ast.Interface:
		c.TypeParams(decl.TypeParams)
		c.Self(nil)

	case *ast.Impl:
		c.TypeParams(decl.TypeParams)
		c.Self(decl.Type)

	case *ast.Func:
		c.TypeParams(decl.TypeParams)
		c.scopeTypes(ast.GetClosestParent[*ast.Impl](decl))
	}
}

func typeSlot(n ast.Node) (typ ast.Type, name *ast.Leaf) {
	switch n := n.(type) {
	case *ast.Var:
		return n.Type, n.Name
	case *ast.Const:
		return n.Type, n.Name_
	case *ast.GlobalVar:
		return n.Type, n.Name_
	case *ast.Param:
		return n.Type, n.Name
	case *ast.Field:
		return n.Type, n.Name
	case *ast.AssociatedConst:
		return n.Type, n.Name
	case *ast.AssociatedType:
		return n.Type, n.Name
	case *ast.TypeAlias:
		return n.Type, n.Name_
	case *ast.Impl:
		return n.Type, nil
	}

	return nil, nil
}

func (c *completions) emptyTypeSlot(typ ast.Type, name *ast.Leaf, declStart core.Pos) bool {
	bad, ok := typ.(*ast.BadType)
	if !ok {
		return false
	}

	// Nameless holders ('impl'): the slot is only attempted when the BadType
	// lies past the declaration keyword. Otherwise the BadType is the recovery
	// for a failed name ('const |'), which is a name position instead.
	if name == nil || name.Token.Text == "" {
		return bad.Range().Start.GreaterThan(declStart)
	}

	// The slot must have been attempted: the BadType sits after the name
	if !bad.Range().Start.GreaterThan(name.Range().End) {
		return false
	}

	return c.cursor.GreaterThan(name.Range().End)
}

// Member expression

func (c *completions) Member(m *ast.Member) {
	// Base expression type
	info, ok := c.file.ExprInfos[m.Expr]
	if !ok || info.Invalid() {
		return
	}

	// Type environment
	if c.workspace.typeEnv == nil {
		return
	}

	// Reference / Pointer
	typ := info.Type

	if r, ok := typ.(*types.Reference); ok {
		typ = r.Pointee
	} else if p, ok := typ.(*types.Pointer); ok {
		typ = p.Pointee
	}

	// Members
	modPath := modulePath(c.file)

	switch t := typ.(type) {
	case *types.Struct:
		c.StructMember(t, modPath)
	case *types.Enum:
		c.InstanceMember(t, t.ModulePath, modPath)
	case *types.Interface:
		c.InterfaceMember(t)
	case *types.Param:
		c.ParamMember(t)
	}
}

func (c *completions) StructMember(t *types.Struct, modPath []string) {
	// Fields
	c.Fields(t, modPath)

	// Methods
	c.InstanceMember(t, t.ModulePath, modPath)
}

func (c *completions) Fields(t *types.Struct, modPath []string) {
	for i := range t.Fields {
		field := &t.Fields[i]

		// Visibility
		if !field.Public && !slices.Equal(t.ModulePath, modPath) {
			continue
		}

		item := protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindField,
			Label:  field.Name,
			Detail: field.Type.String(),
		}

		// Documentation
		lookup := t
		if lookup.Generic != nil {
			lookup = lookup.Generic
		}

		if structNode := c.workspace.typeEnv.GetStructNode(lookup); structNode != nil {
			for _, fieldNode := range structNode.Fields {
				if fieldNode.Name.Token.Text == field.Name {
					item.Documentation = c.s.markup(fieldNode.Documentation)
					break
				}
			}
		}

		c.Add(item)
	}
}

func (c *completions) InstanceMember(typ types.Type, declModPath, modPath []string) {
	for member := range c.workspace.typeEnv.InstanceMembers(typ) {
		if !member.Symbol.Public && !slices.Equal(declModPath, modPath) {
			continue
		}

		c.Func(member.Symbol.Node, protocol.CompletionItemKindMethod)
	}
}

func (c *completions) InterfaceMember(t *types.Interface) {
	// Interface node
	inNode := c.workspace.typeEnv.GetInterfaceNode(t)
	if inNode == nil {
		return
	}

	for _, method := range t.InstanceMethods {
		// Method AST node
		var f *ast.Func

		for _, mf := range inNode.Methods {
			if mf.Name().Token.Text == method.Name {
				f = mf
				break
			}
		}

		if f == nil {
			continue
		}

		c.Func(f, protocol.CompletionItemKindMethod)
	}
}

func (c *completions) ParamMember(t *types.Param) {
	for member := range c.workspace.typeEnv.ParamInstanceMembers(t) {
		c.Func(member.Symbol.Node, protocol.CompletionItemKindMethod)
	}
}

// Field lists

func (c *completions) FieldList() bool {
	// Owning field list
	var fields []*ast.FieldInitializer
	var typ types.Type

	if fi := ast.GetClosestParent[*ast.FieldInitializer](c.node); fi != nil {
		// Field name position (a real name with the cursor past it is a value position)
		if fi.Name != nil && fi.Name.Token.Text != "" && c.cursor.GreaterThan(fi.Name.Range().End) {
			return false
		}

		switch owner := fi.Parent().(type) {
		case *ast.StructInitializer:
			fields = owner.Fields
			typ = c.InitializerType(owner.Type)

		case *ast.With:
			fields = owner.Fields
			typ = c.WithType(owner)

		default:
			return false
		}
	} else {
		// Empty list zone: the node is the anchor itself
		switch anchor := c.node.(type) {
		case *ast.StructInitializer:
			fields = anchor.Fields
			typ = c.InitializerType(anchor.Type)

		case *ast.With:
			fields = anchor.Fields
			typ = c.WithType(anchor)

		default:
			return false
		}
	}

	// Alias
	if a, ok := typ.(*types.Alias); ok {
		typ = a.Type
	}

	// Struct type
	t, ok := typ.(*types.Struct)
	if !ok {
		return false
	}

	// Hide fields that are already initialized
	for _, field := range fields {
		if field.Name != nil && field.Name.Token.Text != "" {
			c.seen[field.Name.Token.Text] = struct{}{}
		}
	}

	// Fields
	c.Fields(t, modulePath(c.file))
	return true
}

func (c *completions) InitializerType(typeAst ast.Type) types.Type {
	switch t := typeAst.(type) {
	case *ast.IdentifierType:
		if typ, ok := resolveIdentifierType(c.file, t); ok {
			return typ
		}

		return nil

	case *ast.SelfType:
		return c.s.resolveSelf(c.file, t)
	}

	if core.IsNil(typeAst) {
		return nil
	}

	return c.file.NodeTypes[typeAst]
}

func (c *completions) WithType(w *ast.With) types.Type {
	info, ok := c.file.ExprInfos[w.Expr]
	if !ok || info.Invalid() {
		return nil
	}

	// Reference / Pointer (sema requires a plain struct value, completion stays forgiving)
	typ := info.Type

	if r, ok := typ.(*types.Reference); ok {
		typ = r.Pointee
	} else if p, ok := typ.(*types.Pointer); ok {
		typ = p.Pointee
	}

	return typ
}

// Enclosing declarations

func (c *completions) EnclosingDecl(decl ast.Decl) {
	if core.IsNil(decl) {
		return
	}

	switch decl := decl.(type) {
	case *ast.Struct:
		c.TypeParams(decl.TypeParams)

	case *ast.Interface:
		c.TypeParams(decl.TypeParams)
		c.Self(nil)
		c.EnclosingAssociatedType(decl.AssociatedTypes)
		c.EnclosingAssociatedConst(decl.AssociatedConsts)

	case *ast.Impl:
		c.TypeParams(decl.TypeParams)
		c.Self(decl.Type)
		c.EnclosingAssociatedType(decl.AssociatedTypes)
		c.EnclosingAssociatedConst(decl.AssociatedConsts)

	case *ast.Func:
		c.EnclosingDecl(ast.GetClosestParent[*ast.Impl](decl))
		c.TypeParams(decl.TypeParams)
		c.EnclosingReceiver(decl.Receiver)

		if block := ast.GetClosestParent[*ast.Block](c.node); block != nil {
			c.EnclosingLocalVar(block)
		}

		c.EnclosingParam(decl.Params)
	}
}

func (c *completions) EnclosingAssociatedType(assocTypes []*ast.AssociatedType) {
	for _, assocType := range assocTypes {
		c.AssociatedType(c.file, assocType)
	}
}

func (c *completions) EnclosingAssociatedConst(assocConsts []*ast.AssociatedConst) {
	for _, assocConst := range assocConsts {
		c.AssociatedConst(c.file, assocConst)
	}
}

func (c *completions) EnclosingLocalVar(block *ast.Block) {
	for _, stmt := range block.Stmts {
		if stmt.Range().Start.GreaterThan(c.cursor) {
			break
		}

		if local, ok := stmt.(*ast.Var); ok && c.cursor.GreaterThan(local.Name.Range().End) {
			c.Add(protocol.CompletionItem{
				Kind:   protocol.CompletionItemKindVariable,
				Label:  local.Name.Token.Text,
				Detail: typeString(c.file, local, local.Type),
			})
		}
	}

	if parent := block.Parent(); !core.IsNil(parent) {
		if block := ast.GetClosestParent[*ast.Block](parent); block != nil {
			c.EnclosingLocalVar(block)
		}
	}
}

func (c *completions) EnclosingParam(params []*ast.Param) {
	for _, param := range params {
		c.Add(protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindTypeParameter,
			Label:  param.Name.Token.Text,
			Detail: typeString(c.file, param, param.Type),
		})
	}
}

func (c *completions) EnclosingReceiver(receiver *ast.Receiver) {
	if receiver == nil {
		return
	}

	impl := ast.GetClosestParent[*ast.Impl](receiver)
	prefix := "&"

	if receiver.Mutable {
		prefix = "mut &"
	}

	c.Add(protocol.CompletionItem{
		Kind:   protocol.CompletionItemKindTypeParameter,
		Label:  "self",
		Detail: prefix + typeString(c.file, impl, impl.Type),
	})
}

// Utils

func (c *completions) ExprSnippets() {
	c.Add(protocol.CompletionItem{
		Kind:             protocol.CompletionItemKindSnippet,
		Label:            "sizeof",
		InsertTextFormat: protocol.InsertTextFormatSnippet,
		InsertText:       "sizeof(${1:type})",
	})

	c.Add(protocol.CompletionItem{
		Kind:             protocol.CompletionItemKindSnippet,
		Label:            "alignof",
		InsertTextFormat: protocol.InsertTextFormatSnippet,
		InsertText:       "alignof(${1:type})",
	})

	c.Add(protocol.CompletionItem{
		Kind:             protocol.CompletionItemKindSnippet,
		Label:            "offsetof",
		InsertTextFormat: protocol.InsertTextFormatSnippet,
		InsertText:       "offsetof(${1:type}, ${2:field})",
	})

	c.Add(protocol.CompletionItem{
		Kind:             protocol.CompletionItemKindSnippet,
		Label:            "typeof",
		InsertTextFormat: protocol.InsertTextFormatSnippet,
		InsertText:       "typeof(${1:type})",
	})
}

func (c *completions) AssociatedConst(file *project.File, assoc *ast.AssociatedConst) {
	c.Add(protocol.CompletionItem{
		Kind:          protocol.CompletionItemKindConstant,
		Label:         assoc.Name.Token.Text,
		Detail:        constHoverLabel(file, "", "", assoc.Type, assoc.Value),
		Documentation: c.s.markup(assoc.Documentation),
	})
}

func (c *completions) AssociatedType(file *project.File, assoc *ast.AssociatedType) {
	item := protocol.CompletionItem{
		Kind:  protocol.CompletionItemKindTypeParameter,
		Label: assoc.Name.Token.Text,
	}

	if !core.IsNil(assoc.Type) {
		item.Kind = getAstTypeCompletionKind(file, assoc.Type)
		item.Detail = typeString(file, assoc, assoc.Type)
	}

	c.Add(item)
}

func (c *completions) Self(parent ast.Type) {
	detail := ""

	if !core.IsNil(parent) {
		detail = parent.String()
	}

	c.Add(protocol.CompletionItem{
		Kind:   protocol.CompletionItemKindKeyword,
		Label:  "Self",
		Detail: detail,
	})
}

func (c *completions) TypeParams(params []*ast.TypeParam) {
	for _, param := range params {
		c.Add(protocol.CompletionItem{
			Kind:  protocol.CompletionItemKindTypeParameter,
			Label: param.Name.Token.Text,
		})
	}
}

func (c *completions) Func(node ast.Node, kind protocol.CompletionItemKind) {
	f, ok := node.(*ast.Func)
	if !ok {
		return
	}

	// Declaration file
	declFile, locker := c.s.getFile(ast.GetFile(f).Path)
	if declFile == nil {
		return
	}

	locker.Lock()
	defer locker.Unlock()

	// Completion
	c.Decl(declFile, f, kind)
}

func (c *completions) TypeMember(typ types.Type) {
	// Alias
	if a, ok := typ.(*types.Alias); ok {
		typ = a.Type
	}

	// Type environment
	if c.workspace.typeEnv == nil {
		return
	}

	// Constrained type parameter: merged scope of all constraints
	if tp, ok := typ.(*types.Param); ok {
		if scope, ok := c.workspace.typeEnv.GetTypeScope(tp).(symbols.SymbolScope); ok {
			for _, symbol := range scope {
				c.StaticMember(sema.MemberSymbol{Symbol: symbol, Type: symbol.Type})
			}
		}

		return
	}

	// Interface statics are registered under the canonical variant
	if in, ok := typ.(*types.Interface); ok {
		typ = in.AsImmutable()
	}

	// Registered static members
	for member := range c.workspace.typeEnv.StaticMembers(typ) {
		c.StaticMember(member)
	}
}

func (c *completions) StaticMember(member sema.MemberSymbol) {
	switch member.Symbol.Kind {
	// Static method
	case symbols.Func:
		c.Func(member.Symbol.Node, protocol.CompletionItemKindFunction)

	// Enum case
	case symbols.Case:
		item := protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindEnumMember,
			Label:  member.Symbol.Name,
			Detail: member.Type.String(),
		}

		if cas, ok := member.Symbol.Node.(*ast.Case); ok {
			item.Label = cas.Name.Token.Text
			item.Documentation = c.s.markup(cas.Documentation)
		}

		c.Add(item)

	// Associated constant
	case symbols.AssociatedConst:
		if assoc, ok := member.Symbol.Node.(*ast.AssociatedConst); ok {
			if declFile, locker := c.s.getFile(ast.GetFile(assoc).Path); declFile != nil {
				locker.Lock()
				c.AssociatedConst(declFile, assoc)
				locker.Unlock()

				return
			}
		}

		c.Add(protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindConstant,
			Label:  member.Symbol.Name,
			Detail: member.Type.String(),
		})

	// Associated type
	case symbols.TypeParam:
		if assoc, ok := member.Symbol.Node.(*ast.AssociatedType); ok {
			if declFile, locker := c.s.getFile(ast.GetFile(assoc).Path); declFile != nil {
				locker.Lock()
				c.AssociatedType(declFile, assoc)
				locker.Unlock()

				return
			}
		}

		c.Add(protocol.CompletionItem{
			Kind:  protocol.CompletionItemKindTypeParameter,
			Label: member.Symbol.Name,
		})

	default:
	}
}

func (c *completions) Add(item protocol.CompletionItem) {
	if _, ok := c.seen[item.Label]; ok {
		return
	}

	c.seen[item.Label] = struct{}{}
	c.items = append(c.items, item)
}

func (c *completions) Build() *protocol.CompletionList {
	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        c.items,
	}
}

func validLeaves(leaves []*ast.Leaf) []*ast.Leaf {
	valid := make([]*ast.Leaf, 0, len(leaves))

	for _, leaf := range leaves {
		if leaf.Token.Text != "" {
			valid = append(valid, leaf)
		}
	}

	return valid
}

func modulePath(file *project.File) []string {
	path := make([]string, 0, len(file.Ast.Mod.Path))

	for _, entry := range file.Ast.Mod.Path {
		path = append(path, entry.Token.Text)
	}

	return path
}

func moduleDetail(module *project.Module) string {
	var modules []*project.Module

	modules = append(modules, module)

	for module.Parent != nil {
		module = module.Parent
		modules = append(modules, module)
	}

	var detail strings.Builder

	for _, module := range slices.Backward(modules) {
		if detail.Len() > 0 {
			detail.WriteString("::")
		}

		detail.WriteString(module.Name)
	}

	return detail.String()
}

func snapCursorLeft(file *project.File, cursor core.Pos) core.Pos {
	source, ok := file.Source.(*Source)
	if !ok {
		return cursor
	}

	lineIdx, byteOffset, ok := sourcePos(source, cursor)
	if !ok {
		return cursor
	}

	line := source.lines[lineIdx]

	// Scan back over spaces and tabs
	whitespace := 0

	for byteOffset-whitespace > 0 {
		if ch := line[byteOffset-whitespace-1]; ch != ' ' && ch != '\t' {
			break
		}

		whitespace++
	}

	cursor.Column -= uint32(whitespace)
	return cursor
}

// sourcePos converts a cursor position to a (line index, byte offset) pair.
func sourcePos(source *Source, cursor core.Pos) (int, int, bool) {
	if cursor.Line < 1 || cursor.Line > uint32(len(source.lines)) {
		return 0, 0, false
	}

	line := source.lines[cursor.Line-1]

	byteOffset := 0

	for column := uint32(1); column < cursor.Column; column++ {
		_, size := utf8.DecodeRune(line[byteOffset:])
		if size == 0 {
			return 0, 0, false
		}

		byteOffset += size
	}

	return int(cursor.Line) - 1, byteOffset, true
}

func (c *completions) Decl(file *project.File, decl ast.Decl, overrideKind protocol.CompletionItemKind) {
	if decl.Name() == nil {
		return
	}

	item := protocol.CompletionItem{
		Label:         decl.Name().Token.Text,
		Documentation: c.s.markup(decl.Documentation()),
	}

	switch decl := decl.(type) {
	case *ast.TypeAlias:
		item.Kind = getAstTypeCompletionKind(file, decl.Type)
		item.Detail = decl.Type.String()

	case *ast.Struct:
		item.Kind = protocol.CompletionItemKindStruct

	case *ast.Enum:
		item.Kind = protocol.CompletionItemKindEnum

		if typ, ok := file.NodeTypes[decl]; ok {
			if enum, ok := typ.(*types.Enum); ok {
				item.Detail = enum.CaseType.String()
			}
		}

	case *ast.Interface:
		item.Kind = protocol.CompletionItemKindInterface

	case *ast.Const:
		item.Kind = protocol.CompletionItemKindConstant
		item.Detail = constHoverLabel(file, "", "", decl.Type, decl.Value)
		item.CommitCharacters = []string{"."}

	case *ast.GlobalVar:
		item.Kind = protocol.CompletionItemKindVariable
		item.Detail = decl.Type.String()
		item.CommitCharacters = []string{"."}

	case *ast.Func:
		item.Kind = protocol.CompletionItemKindFunction
		item.Detail = decl.String(true)

		if c.exprContext {
			if len(decl.Params) == 0 && !decl.VarArgs {
				item.InsertText = item.Label + "()"
			} else {
				item.InsertTextFormat = protocol.InsertTextFormatSnippet
				item.InsertText = item.Label + "($1)"
			}
		}

	default:
		return
	}

	if overrideKind != 0 {
		item.Kind = overrideKind
	}

	c.Add(item)
}

func getAstTypeCompletionKind(file *project.File, typ ast.Type) protocol.CompletionItemKind {
	switch typ.(type) {
	case *ast.PrimitiveType:
		return protocol.CompletionItemKindClass
	case *ast.FuncType:
		return protocol.CompletionItemKindFunction
	case *ast.IdentifierType:
		return getTypeCompletionKind(file.NodeTypes[typ])
	case *ast.OptionType, *ast.SliceType:
		return protocol.CompletionItemKindStruct
	default:
		return protocol.CompletionItemKindText
	}
}

func getTypeCompletionKind(typ types.Type) protocol.CompletionItemKind {
	switch typ.(type) {
	case *types.Primitive:
		return protocol.CompletionItemKindClass
	case *types.Struct:
		return protocol.CompletionItemKindStruct
	case *types.Enum:
		return protocol.CompletionItemKindEnum
	case *types.Interface:
		return protocol.CompletionItemKindInterface
	case *types.Func:
		return protocol.CompletionItemKindFunction
	default:
		return protocol.CompletionItemKindText
	}
}
