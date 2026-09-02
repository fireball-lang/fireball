package ast

import (
	"fireball/core"
	"fireball/types"
	"iter"
	"strings"
)

type Decl interface {
	AttributeHolder

	Documentation() []*Leaf
	Name() *Leaf

	_isDecl()
}

type AttributeHolder interface {
	Node

	Attributes() []Attribute
}

func GetAttribute[A Attribute, N AttributeHolder](node N) A {
	for _, attribute := range node.Attributes() {
		if attr, ok := attribute.(A); ok {
			return attr
		}
	}

	var empty A
	return empty
}

// TypeAlias

type TypeAlias struct {
	baseNode

	Documentation_ []*Leaf
	Attributes_    []Attribute
	Public         bool

	Name_      *Leaf
	TypeParams []*TypeParam

	Type Type
}

func (t *TypeAlias) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range t.Documentation_ {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range t.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		if !yield(t.Name_) {
			return
		}
		for _, param := range t.TypeParams {
			if !yield(param) {
				return
			}
		}
		if !yield(t.Type) {
			return
		}
	}
}

func (t *TypeAlias) Documentation() []*Leaf {
	return t.Documentation_
}

func (t *TypeAlias) Attributes() []Attribute {
	return t.Attributes_
}

func (t *TypeAlias) Name() *Leaf {
	return t.Name_
}

func (t *TypeAlias) _isDecl() {}

// Struct

type Struct struct {
	baseNode

	Documentation_ []*Leaf
	Attributes_    []Attribute
	Public         bool

	Name_      *Leaf
	TypeParams []*TypeParam
	Fields     []*Field
}

func (s *Struct) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range s.Documentation_ {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range s.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		if !yield(s.Name_) {
			return
		}
		for _, param := range s.TypeParams {
			if !yield(param) {
				return
			}
		}
		for _, field := range s.Fields {
			if !yield(field) {
				return
			}
		}
	}
}

func (s *Struct) Documentation() []*Leaf {
	return s.Documentation_
}

func (s *Struct) Attributes() []Attribute {
	return s.Attributes_
}

func (s *Struct) Name() *Leaf {
	return s.Name_
}

func (s *Struct) _isDecl() {}

func (s *Struct) GetLayout() types.Layout {
	if repr := GetAttribute[*Repr](s); repr != nil {
		return repr.Layout
	}

	return types.Fireball
}

// Field

type Field struct {
	baseNode

	Documentation []*Leaf
	Attributes_   []Attribute
	Public        bool

	Name *Leaf
	Type Type
}

func (f *Field) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range f.Documentation {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range f.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		if !yield(f.Name) {
			return
		}
		yield(f.Type)
	}
}

func (f *Field) Attributes() []Attribute {
	return f.Attributes_
}

// Enum

type Enum struct {
	baseNode

	Documentation_ []*Leaf
	Attributes_    []Attribute
	Public         bool

	Name_ *Leaf
	Type  Type // optional

	Cases []*Case
}

func (e *Enum) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range e.Documentation_ {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range e.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		if !yield(e.Name_) {
			return
		}
		if !core.IsNil(e.Type) && !yield(e.Type) {
			return
		}
		for _, c := range e.Cases {
			if !yield(c) {
				return
			}
		}
	}
}

func (e *Enum) Documentation() []*Leaf {
	return e.Documentation_
}

func (e *Enum) Attributes() []Attribute {
	return e.Attributes_
}

func (e *Enum) Name() *Leaf {
	return e.Name_
}

func (e *Enum) _isDecl() {}

// Case

type Case struct {
	baseNode

	Documentation []*Leaf

	Name  *Leaf
	Value *Leaf // optional
}

func (c *Case) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range c.Documentation {
			if !yield(doc) {
				return
			}
		}
		if !yield(c.Name) {
			return
		}
		if c.Value != nil && !yield(c.Value) {
			return
		}
	}
}

// Interface

type Interface struct {
	baseNode

	Documentation_ []*Leaf
	Attributes_    []Attribute
	Public         bool

	Name_      *Leaf
	TypeParams []*TypeParam

	AssociatedTypes []*AssociatedType
	Methods         []*Func
}

func (i *Interface) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range i.Documentation_ {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range i.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		if !yield(i.Name_) {
			return
		}
		for _, param := range i.TypeParams {
			if !yield(param) {
				return
			}
		}
		for _, assocType := range i.AssociatedTypes {
			if !yield(assocType) {
				return
			}
		}
		for _, method := range i.Methods {
			if !yield(method) {
				return
			}
		}
	}
}

func (i *Interface) Documentation() []*Leaf {
	return i.Documentation_
}

func (i *Interface) Attributes() []Attribute {
	return i.Attributes_
}

func (i *Interface) Name() *Leaf {
	return i.Name_
}

func (i *Interface) _isDecl() {}

// Impl

type Impl struct {
	baseNode

	Documentation_ []*Leaf
	Attributes_    []Attribute

	TypeParams []*TypeParam

	Type      Type
	Interface *IdentifierType // optional

	AssociatedTypes []*AssociatedType
	Methods         []*Func
}

func (i *Impl) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range i.Documentation_ {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range i.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		for _, param := range i.TypeParams {
			if !yield(param) {
				return
			}
		}
		if !yield(i.Type) {
			return
		}
		if i.Interface != nil && !yield(i.Interface) {
			return
		}
		for _, assocType := range i.AssociatedTypes {
			if !yield(assocType) {
				return
			}
		}
		for _, method := range i.Methods {
			if !yield(method) {
				return
			}
		}
	}
}

func (i *Impl) Documentation() []*Leaf {
	return i.Documentation_
}

func (i *Impl) Attributes() []Attribute {
	return i.Attributes_
}

func (i *Impl) Name() *Leaf {
	return nil
}

func (i *Impl) _isDecl() {}

// AssociatedType

type AssociatedType struct {
	baseNode

	Documentation []*Leaf
	Attributes_   []Attribute

	Name *Leaf
	Type Type // nil for interface, non-nil for implementation block
}

func (a *AssociatedType) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range a.Documentation {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range a.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		if !yield(a.Name) {
			return
		}
		if !core.IsNil(a.Type) && !yield(a.Type) {
			return
		}
	}
}

func (a *AssociatedType) Attributes() []Attribute {
	return a.Attributes_
}

// GlobalVar

type GlobalVar struct {
	baseNode

	Documentation_ []*Leaf
	Attributes_    []Attribute
	Public         bool

	Name_ *Leaf
	Type  Type
}

func (g *GlobalVar) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range g.Documentation_ {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range g.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		if !yield(g.Name_) {
			return
		}
		if !yield(g.Type) {
			return
		}
	}
}

func (g *GlobalVar) Documentation() []*Leaf {
	return g.Documentation_
}

func (g *GlobalVar) Attributes() []Attribute {
	return g.Attributes_
}

func (g *GlobalVar) Name() *Leaf {
	return g.Name_
}

func (g *GlobalVar) _isDecl() {}

func (g *GlobalVar) IsExtern() bool {
	return GetAttribute[*Extern](g) != nil
}

func (g *GlobalVar) GetLinkName() string {
	if link := GetAttribute[*LinkName](g); link != nil {
		return string(link.Name.Runes)
	}

	return ""
}

// Func

type Func struct {
	baseNode

	Documentation_ []*Leaf
	Attributes_    []Attribute
	Public         bool

	Name_      *Leaf
	TypeParams []*TypeParam

	Receiver *Receiver // optional
	Params   []*Param
	VarArgs  bool

	Returns Type

	Body Stmt // optional
}

func (f *Func) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, doc := range f.Documentation_ {
			if !yield(doc) {
				return
			}
		}
		for _, attribute := range f.Attributes_ {
			if !yield(attribute) {
				return
			}
		}
		if !yield(f.Name_) {
			return
		}
		for _, param := range f.TypeParams {
			if !yield(param) {
				return
			}
		}
		if f.Receiver != nil && !yield(f.Receiver) {
			return
		}
		for _, param := range f.Params {
			if !yield(param) {
				return
			}
		}
		if !yield(f.Returns) {
			return
		}
		if !core.IsNil(f.Body) {
			yield(f.Body)
		}
	}
}

func (f *Func) Documentation() []*Leaf {
	return f.Documentation_
}

func (f *Func) Attributes() []Attribute {
	return f.Attributes_
}

func (f *Func) Name() *Leaf {
	return f.Name_
}

func (f *Func) _isDecl() {}

func (f *Func) IsMethod() bool {
	_, ok := f.Parent().(*Impl)
	return ok
}

func (f *Func) IsInterfaceMethod() bool {
	if impl, ok := f.Parent().(*Impl); ok {
		return impl.Interface != nil
	}

	return false
}

func (f *Func) GetTestName() string {
	if test := GetAttribute[*Test](f); test != nil {
		name := ""

		if test.Name != nil {
			name = string(test.Name.Runes)
		}

		if name == "" {
			name = f.Name_.Token.Text
		}

		return name
	}

	return ""
}

func (f *Func) IsExtern() bool {
	return GetAttribute[*Extern](f) != nil
}

func (f *Func) GetLinkName() string {
	if link := GetAttribute[*LinkName](f); link != nil {
		return string(link.Name.Runes)
	}

	return ""
}

func (f *Func) String(paramNames bool) string {
	var sb strings.Builder

	sb.WriteString("func ")
	sb.WriteString(f.Name_.Token.Text)
	sb.WriteString("(")

	hasParams := false

	if f.Receiver != nil {
		if f.Receiver.Mutable {
			sb.WriteString("mut self")
		} else {
			sb.WriteString("self")
		}

		hasParams = true
	}

	for _, param := range f.Params {
		if hasParams {
			sb.WriteString(", ")
		}

		if paramNames {
			sb.WriteString(param.Name.Token.Text)
			sb.WriteString(": ")
		}

		sb.WriteString(param.Type.String())

		hasParams = true
	}

	sb.WriteString(") ")
	sb.WriteString(f.Returns.String())

	return sb.String()
}

// Receiver

type Receiver struct {
	baseNode

	Mutable bool
}

func (r *Receiver) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

// Param

type Param struct {
	baseNode

	Name *Leaf // optional when used inside FuncType
	Type Type
}

func (p *Param) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if p.Name != nil && !yield(p.Name) {
			return
		}
		yield(p.Type)
	}
}

// TypeParam

type TypeParam struct {
	baseNode

	Name        *Leaf
	Constraints []Type // optional
}

func (t *TypeParam) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(t.Name) {
			return
		}
		for _, c := range t.Constraints {
			if !yield(c) {
				return
			}
		}
	}
}

// Bad

type BadDecl struct {
	baseNode
}

func (b *BadDecl) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadDecl) Documentation() []*Leaf {
	return nil
}

func (b *BadDecl) Attributes() []Attribute {
	return nil
}

func (b *BadDecl) Name() *Leaf {
	return nil
}

func (b *BadDecl) _isDecl() {}
