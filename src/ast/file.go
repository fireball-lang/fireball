package ast

import (
	"iter"
)

type File struct {
	baseNode

	Mod     *Mod
	Imports []*Import
	Decls   []Decl
}

func (f *File) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(f.Mod) {
			return
		}
		for _, i := range f.Imports {
			if !yield(i) {
				return
			}
		}
		for _, decl := range f.Decls {
			if !yield(decl) {
				return
			}
		}
	}
}

// Mod

type Mod struct {
	baseNode

	Path *IdentifierPath
}

func (m *Mod) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(m.Path) {
			return
		}
	}
}

// Import

type Import struct {
	baseNode

	Path    *IdentifierPath
	Symbols []*Leaf // optional
	Alias   *Leaf   // optional
}

func (i *Import) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(i.Path) {
			return
		}
		for _, symbol := range i.Symbols {
			if !yield(symbol) {
				return
			}
		}
		if i.Alias != nil && !yield(i.Alias) {
			return
		}
	}
}
