package ast

import (
	"iter"
)

type Attribute interface {
	Node

	_isAttribute()
}

// Test

type Test struct {
	baseNode

	Name *String // optional
}

func (t *Test) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if t.Name != nil && !yield(t.Name) {
			return
		}
	}
}

func (t *Test) _isAttribute() {}

// Extern

type Extern struct {
	baseNode
}

func (e *Extern) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (e *Extern) _isAttribute() {}

// LinkName

type LinkName struct {
	baseNode

	Name *String
}

func (l *LinkName) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(l.Name) {
			return
		}
	}
}

func (l *LinkName) _isAttribute() {}

// Bad

type BadAttribute struct {
	baseNode
}

func (b *BadAttribute) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadAttribute) _isAttribute() {}
