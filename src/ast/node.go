package ast

import (
	"fireball/lexer"
	"iter"
	"reflect"
)

type Node interface {
	Parent() Node
	setParent(parent Node)

	Children() iter.Seq[Node]

	Range() lexer.Range
}

func IsValid(n Node) bool {
	if n == nil {
		return false
	}

	switch reflect.TypeOf(n).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Slice, reflect.Func:
		return !reflect.ValueOf(n).IsNil()
	default:
		return false
	}
}

// baseNode

type baseNode struct {
	parent Node
}

func (b *baseNode) Parent() Node {
	return b.parent
}

func (b *baseNode) setParent(parent Node) {
	b.parent = parent
}

// baseRangeNode

type baseRangeNode struct {
	baseNode

	range_ lexer.Range
}

func (b *baseRangeNode) Range() lexer.Range {
	return b.range_
}

// Leaf

type Leaf struct {
	baseNode

	Token lexer.Token
}

func (l *Leaf) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (l *Leaf) Range() lexer.Range {
	return l.Token.Range
}

// Path

type Path struct {
	baseRangeNode

	Segments []*Leaf
}

func (p *Path) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, segment := range p.Segments {
			if !yield(segment) {
				return
			}
		}
	}
}

func (p *Path) SegmentCount() int {
	return len(p.Segments)
}

func (p *Path) SegmentAt(index int) string {
	return p.Segments[index].Token.Text
}

// File

type File struct {
	baseRangeNode

	Decls []Decl
}

func (f *File) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, decl := range f.Decls {
			if !yield(decl) {
				return
			}
		}
	}
}

func (f *File) ModulePath() *Path {
	for _, decl := range f.Decls {
		if mod, ok := decl.(*Mod); ok && mod.Path != nil {
			return mod.Path
		}
	}

	return &Path{}
}
