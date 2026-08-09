package lsp

import (
	"context"
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/project"
	"fmt"
	"slices"
	"strings"

	"github.com/fireball-lang/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) PrepareRename(ctx context.Context, params *protocol.PrepareRenameParams) (*protocol.Range, error) {
	// Get file
	file, locker := s.getFile(params.TextDocument.URI.Filename())
	if file == nil {
		return nil, nil
	}

	// Lock file
	locker.Lock()
	defer locker.Unlock()

	// Deepest node at the cursor position
	node := ast.GetNodeAtPos(file.Ast, toCorePos(params.Position))

	if core.IsNil(node) {
		s.warn(ctx, "failed to get leaf node at position")
		return nil, nil
	}

	// Only support renaming when the cursor is positioned on an identifier
	// token, so that empty space doesn't resolve to an enclosing declaration.
	leaf, ok := node.(*ast.Leaf)
	if !ok || leaf.Token.Kind != lexer.Identifier {
		return nil, nil
	}

	// Resolve the target, either a referenced definition or the declaration itself
	defNode := s.resolveDefinition(file, node)
	if core.IsNil(defNode) {
		defNode = s.declarationAt(node)
	}

	if core.IsNil(defNode) {
		return nil, nil
	}

	rng := toLspRange(leaf.Range())

	return &rng, nil
}

func (s *Server) Rename(ctx context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	// Get file
	file, locker := s.getFile(params.TextDocument.URI.Filename())
	if file == nil {
		return nil, nil
	}

	// Lock file
	locker.Lock()
	defer locker.Unlock()

	// Deepest node at the cursor position
	node := ast.GetNodeAtPos(file.Ast, toCorePos(params.Position))

	if core.IsNil(node) {
		s.warn(ctx, "failed to get leaf node at position")
		return nil, nil
	}

	// Only rename when the cursor is positioned on an identifier token, so
	// that empty space doesn't resolve to an enclosing declaration.
	if leaf, ok := node.(*ast.Leaf); !ok || leaf.Token.Kind != lexer.Identifier {
		return nil, nil
	}

	// Validate new name
	if !isValidIdentifier(params.NewName) {
		return nil, fmt.Errorf("invalid identifier: '%s'", params.NewName)
	}

	// Resolve the target, either a referenced definition or the declaration itself
	defNode := s.resolveDefinition(file, node)
	if core.IsNil(defNode) {
		defNode = s.declarationAt(node)
	}

	if core.IsNil(defNode) {
		return nil, nil
	}

	// Collect all references and the declaration name, grouped per file
	type edit struct {
		rng  core.Range
		file *project.File
	}

	var edits []edit
	seen := make(map[referenceKey]any)

	add := func(nodeFile *project.File, rng core.Range) {
		key := referenceKey{path: nodeFile.Path, range_: rng}
		if _, ok := seen[key]; ok {
			return
		}

		seen[key] = nil
		edits = append(edits, edit{rng: rng, file: nodeFile})
	}

	s.collectReferences(defNode, false, add)

	if rng := declNameRange(defNode); rng != (core.Range{}) {
		if nodeFile := ast.GetFile(defNode); nodeFile != nil {
			if projectFile, _ := s.getFile(nodeFile.Path); projectFile != nil {
				add(projectFile, rng)
			}
		}
	}

	// Sort edits per file so the client can apply them in order
	slices.SortFunc(edits, func(a, b edit) int {
		if a.file != b.file {
			return strings.Compare(a.file.Path, b.file.Path)
		}

		if a.rng.Start.Line != b.rng.Start.Line {
			if a.rng.Start.Line < b.rng.Start.Line {
				return -1
			}
			return 1
		}

		if a.rng.Start.Column != b.rng.Start.Column {
			if a.rng.Start.Column < b.rng.Start.Column {
				return -1
			}
			return 1
		}

		return 0
	})

	changes := make(map[protocol.DocumentURI][]protocol.TextEdit)

	for _, e := range edits {
		uri_ := uri.File(e.file.Path)
		changes[uri_] = append(changes[uri_], protocol.TextEdit{
			Range:   toLspRange(e.rng),
			NewText: params.NewName,
		})
	}

	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

func isValidIdentifier(name string) bool {
	if name == "" {
		return false
	}

	l := lexer.New(strings.NewReader(name))

	token := l.Next()
	if token.Kind != lexer.Identifier {
		return false
	}

	return l.Next().Kind == lexer.EOF
}
