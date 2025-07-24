package ast

import (
	"fireball/utils"
	"iter"
)

type Decl interface {
	Node

	Name() string

	Visit(visitor DeclVisitor)
}

type DeclVisitor interface {
	VisitMod(m *Mod)
	VisitImport(i *Import)
	VisitStruct(f *Struct)
	VisitEnum(e *Enum)
	VisitInterface(i *Interface)
	VisitImpl(i *Impl)
	VisitGlobalVar(g *GlobalVar)
	VisitFunc(f *Func)
}

// Attribute

type Attribute struct {
	baseRangeNode

	Name  *Leaf
	Param string
}

func (a *Attribute) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if IsValid(a.Name) && !yield(a.Name) {
			return
		}
	}
}

type attributeHolder struct {
	Attributes []*Attribute
}

func (a *attributeHolder) GetAttribute(name string) *Attribute {
	for _, attribute := range a.Attributes {
		if attribute.Name != nil && attribute.Name.Token.Text == name {
			return attribute
		}
	}

	return nil
}

// Mod

type Mod struct {
	baseRangeNode
	attributeHolder

	Path *Path
}

func (m *Mod) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if m.Path != nil && !yield(m.Path) {
			return
		}
	}
}

func (m *Mod) Name() string {
	return "<mod>"
}

func (m *Mod) Visit(visitor DeclVisitor) {
	visitor.VisitMod(m)
}

// Import

type Import struct {
	baseRangeNode
	attributeHolder

	Path    *Path
	Symbols []*Leaf

	ResolvedSymbols []Decl
}

func (i *Import) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if i.Path != nil && !yield(i.Path) {
			return
		}

		for _, symbol := range i.Symbols {
			if !yield(symbol) {
				return
			}
		}
	}
}

func (i *Import) Name() string {
	return "<import>"
}

func (i *Import) Visit(visitor DeclVisitor) {
	visitor.VisitImport(i)
}

// Struct

type Struct struct {
	baseRangeNode
	attributeHolder

	NameN  *Leaf
	Fields []*Field
}

func (s *Struct) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range s.Attributes {
			if !yield(attribute) {
				return
			}
		}

		if s.NameN != nil && !yield(s.NameN) {
			return
		}

		for _, field := range s.Fields {
			if !yield(field) {
				return
			}
		}
	}
}

func (s *Struct) Name() string {
	if s.NameN != nil {
		return s.NameN.Token.Text
	}

	return ""
}

func (s *Struct) Visit(visitor DeclVisitor) {
	visitor.VisitStruct(s)
}

func (s *Struct) GetField(name string) (*Field, int) {
	for i, field := range s.Fields {
		if field.Name != nil && field.Name.Token.Text == name {
			return field, i
		}
	}

	return nil, -1
}

// Field

type Field struct {
	baseRangeNode

	Name *Leaf
	Type Type
}

func (f *Field) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if f.Name != nil && !yield(f.Name) {
			return
		}
		if IsValid(f.Type) && !yield(f.Type) {
			return
		}
	}
}

// Enum

type Enum struct {
	baseRangeNode
	attributeHolder

	NameN *Leaf
	Type  Type
	Cases []*EnumCase

	ActualType Type
}

func (e *Enum) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if e.NameN != nil && !yield(e.NameN) {
			return
		}
		if IsValid(e.Type) && !yield(e.Type) {
			return
		}
		for _, enumCase := range e.Cases {
			if !yield(enumCase) {
				return
			}
		}
	}
}

func (e *Enum) Name() string {
	if e.NameN != nil {
		return e.NameN.Token.Text
	}

	return ""
}

func (e *Enum) Visit(visitor DeclVisitor) {
	visitor.VisitEnum(e)
}

// EnumCase

type EnumCase struct {
	baseRangeNode

	Name  *Leaf
	Value *Leaf

	ActualValue utils.Integer
}

func (e *EnumCase) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if e.Name != nil && !yield(e.Name) {
			return
		}
		if e.Value != nil && !yield(e.Value) {
			return
		}
	}
}

// Interface

type Interface struct {
	baseRangeNode
	attributeHolder

	NameN   *Leaf
	Methods []*Func
}

func (i *Interface) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if i.NameN != nil && !yield(i.NameN) {
			return
		}
		for _, method := range i.Methods {
			if !yield(method) {
				return
			}
		}
	}
}

func (i *Interface) Name() string {
	if i.NameN != nil {
		return i.NameN.Token.Text
	}

	return ""
}

func (i *Interface) Visit(visitor DeclVisitor) {
	visitor.VisitInterface(i)
}

// Impl

type Impl struct {
	baseRangeNode
	attributeHolder

	DeclName *Leaf
	Decl     Decl

	InterfaceName *Leaf
	Interface     *Interface

	StaticMethods []*Func
	Methods       []*Func
}

func (i *Impl) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range i.Attributes {
			if !yield(attribute) {
				return
			}
		}

		if i.DeclName != nil && !yield(i.DeclName) {
			return
		}
		if i.InterfaceName != nil && !yield(i.InterfaceName) {
			return
		}

		for _, method := range i.StaticMethods {
			if !yield(method) {
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

func (i *Impl) Name() string {
	if i.DeclName != nil {
		return i.DeclName.Token.Text
	}

	return ""
}

func (i *Impl) Visit(visitor DeclVisitor) {
	visitor.VisitImpl(i)
}

// GlobalVar

type GlobalVar struct {
	baseRangeNode
	attributeHolder

	NameN *Leaf
	Type  Type
}

func (g *GlobalVar) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if g.NameN != nil && !yield(g.NameN) {
			return
		}
		if IsValid(g.Type) && !yield(g.Type) {
			return
		}
	}
}

func (g *GlobalVar) Name() string {
	if g.NameN != nil {
		return g.NameN.Token.Text
	}

	return ""
}

func (g *GlobalVar) Visit(visitor DeclVisitor) {
	visitor.VisitGlobalVar(g)
}

// Func

type Func struct {
	baseRangeNode
	attributeHolder

	NameN   *Leaf
	Params  []*Param
	varArgs bool
	Returns Type
	Body    *Block
}

func (f *Func) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, attribute := range f.Attributes {
			if !yield(attribute) {
				return
			}
		}

		if f.NameN != nil && !yield(f.NameN) {
			return
		}

		for _, param := range f.Params {
			if !yield(param) {
				return
			}
		}

		if IsValid(f.Returns) && !yield(f.Returns) {
			return
		}
		if f.Body != nil && !yield(f.Body) {
			return
		}
	}
}

func (f *Func) Name() string {
	if f.NameN != nil {
		return f.NameN.Token.Text
	}

	return ""
}

func (f *Func) Visit(visitor DeclVisitor) {
	visitor.VisitFunc(f)
}

func (f *Func) Equals(other Type) bool {
	return funcTypeEquals(f, other)
}

func (f *Func) String() string {
	return funcTypeString(f, false)
}

func (f *Func) StringWithParamNames() string {
	return funcTypeString(f, true)
}

func (f *Func) ParamTypeCount() int {
	return len(f.Params)
}

func (f *Func) ParamTypeAt(index int) Type {
	return f.Params[index].Type
}

func (f *Func) ParamTypes() iter.Seq[Type] {
	return func(yield func(Type) bool) {
		for _, param := range f.Params {
			if IsValid(param.Type) && !yield(param.Type) {
				return
			}
		}
	}
}

func (f *Func) VarArgs() bool {
	return f.varArgs
}

func (f *Func) ReturnType() Type {
	if IsValid(f.Returns) {
		return f.Returns
	}

	return VoidType
}

func (f *Func) IsMethod() bool {
	switch f.Parent().(type) {
	case *Impl, *Interface:
		return true
	default:
		return false
	}
}

// Param

type Param struct {
	baseRangeNode

	Name *Leaf
	Type Type
}

func (p *Param) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if p.Name != nil && !yield(p.Name) {
			return
		}
		if IsValid(p.Type) && !yield(p.Type) {
			return
		}
	}
}
