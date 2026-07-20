package ast

import (
	"fireball/core"
	"iter"
)

type File struct {
	baseNode

	Path string

	Attributes_ []Attribute

	Mod     *Mod
	Imports []*Import
	Decls   []Decl

	Stripped []core.Range
}

func (f *File) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range f.Attributes_ {
			for !yield(attribute) {
				return
			}
		}
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

func (f *File) Attributes() []Attribute {
	return f.Attributes_
}

// Mod

type Mod struct {
	baseNode

	Path []*Leaf
}

func (m *Mod) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, entry := range m.Path {
			if !yield(entry) {
				return
			}
		}
	}
}

// Import

type Import struct {
	baseNode

	Attributes_ []Attribute

	Path    []*Leaf
	Symbols []*Leaf // optional
	Alias   *Leaf   // optional
}

func (i *Import) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range i.Attributes_ {
			for !yield(attribute) {
				return
			}
		}
		for _, entry := range i.Path {
			if !yield(entry) {
				return
			}
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

func (i *Import) Attributes() []Attribute {
	return i.Attributes_
}
