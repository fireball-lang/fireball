package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"math"
)

// Visitor

func (c *common) VisitPrimitiveType(t *ast.PrimitiveType) types.Type {
	typ := types.GetPrimitive(t.Kind)

	c.nodeTypes[t] = typ
	return typ
}

func (c *common) VisitArrayType(t *ast.ArrayType) types.Type {
	if typ := c.nodeTypes[t]; !core.IsNil(typ) {
		return typ
	}

	element := c.ResolveAndAnalyzeType(t.Type)
	if element == types.Invalid {
		return types.Invalid
	}
	if element == types.PrimitiveVoid {
		return c.Error(t, "arrays of voids are not allowed").Type
	}

	size := lexer.ParseInteger(t.Size)
	if size.Negative() {
		return c.Error(t, "array size cannot be negative").Type
	}
	if size.Raw() == 0 {
		return c.Error(t, "zero-sized arrays are not allowed").Type
	}
	if size.Raw() > math.MaxUint32 {
		return c.Error(t, "array size cannot be bigger than an unsigned 32-bit integer").Type
	}

	typ := &types.Array{
		Size:    uint32(size.Raw()),
		Element: element,
	}

	c.nodeTypes[t] = typ
	return typ
}

func (c *common) VisitPointerType(t *ast.PointerType) types.Type {
	if typ := c.nodeTypes[t]; !core.IsNil(typ) {
		return typ
	}

	pointee := c.ResolveAndAnalyzeType(t.Pointee)
	if pointee == types.Invalid {
		return types.Invalid
	}

	typ := &types.Pointer{
		Mutable: t.Mutable,
		Pointee: pointee,
	}

	c.nodeTypes[t] = typ
	return typ
}

func (c *common) VisitFuncType(t *ast.FuncType) types.Type {
	if typ := c.nodeTypes[t]; !core.IsNil(typ) {
		return typ
	}

	params := make([]types.Type, len(t.Params))

	for i, param := range t.Params {
		params[i] = c.ResolveAndAnalyzeType(param.Type)
	}

	typ := &types.Func{
		Params:  params,
		VarArgs: t.VarArgs,
		Returns: c.ResolveAndAnalyzeType(t.Returns),
	}

	c.nodeTypes[t] = typ
	return typ
}

func (c *common) VisitIdentifierType(t *ast.IdentifierType) types.Type {
	// Get symbol for path
	symbol, ok := c.GetSymbol(symbols.Type, t.Path)
	if !ok {
		return types.Invalid
	}

	// Check nodeTypes for an existing type
	cached := c.nodeTypes[t]
	if !core.IsNil(cached) {
		return cached
	}

	switch symbol.Kind {
	case symbols.Struct:
		s := symbol.Type.(*types.Struct)

		if t.Mutable {
			c.Error(t, "struct type '%s' cannot be mutable", s)
		}

		c.nodeTypes[t] = s
		return s

	case symbols.Enum:
		if t.Mutable {
			c.Error(t, "enum type '%s' cannot be mutable", symbol.Name)
		}

		c.nodeTypes[t] = symbol.Type
		return symbol.Type

	case symbols.Interface:
		in := symbol.Type.(*types.Interface)

		if t.Mutable {
			in = in.AsMutable()
		}

		c.nodeTypes[t] = in
		return in

	case symbols.TypeParam:
		if t.Mutable {
			c.Error(t, "type parameter '%s' cannot be mutable", symbol.Name)
		}

		c.nodeTypes[t] = symbol.Type
		return symbol.Type

	case symbols.Func, symbols.Param, symbols.Var:
		c.Error(t, "'%s' cannot be used as a type", t.Path[len(t.Path)-1].Name.Token.Text)
		return types.Invalid

	default:
		panic("sema.analyzer.VisitIdentifierType() - Invalid kind")
	}
}

func (c *common) VisitSelfType(t *ast.SelfType) types.Type {
	if typ := c.nodeTypes[t]; !core.IsNil(typ) {
		return typ
	}

	if c.selfType == nil {
		return c.Error(t, "'Self' can only be used inside an impl or interface block").Type
	}

	c.nodeTypes[t] = c.selfType
	return c.selfType
}

func (c *common) VisitOptionType(t *ast.OptionType) types.Type {
	if typ := c.nodeTypes[t]; !core.IsNil(typ) {
		return typ
	}

	path := []*ast.IdentifierEntry{
		{
			Name: &ast.Leaf{Token: lexer.Token{Text: "core"}},
		},
		{
			Name:     &ast.Leaf{Token: lexer.Token{Text: "Option"}},
			TypeArgs: []ast.Type{t.Type},
		},
	}

	symbol, ok := c.GetSymbol(symbols.Type, path)
	if !ok {
		panic("sema.common.VisitOptionType() - Failed to find 'core::Option'")
	}

	return symbol.Type
}

func (c *common) VisitSliceType(t *ast.SliceType) types.Type {
	if typ := c.nodeTypes[t]; !core.IsNil(typ) {
		return typ
	}

	name := "Slice"
	if t.Mutable {
		name = "MutSlice"
	}

	path := []*ast.IdentifierEntry{
		{
			Name: &ast.Leaf{Token: lexer.Token{Text: "core"}},
		},
		{
			Name:     &ast.Leaf{Token: lexer.Token{Text: name}},
			TypeArgs: []ast.Type{t.Type},
		},
	}

	symbol, ok := c.GetSymbol(symbols.Type, path)
	if !ok {
		panic("sema.common.VisitSliceType() - Failed to find 'core::" + name + "'")
	}

	return symbol.Type
}

func (c *common) VisitBadType(_ *ast.BadType) types.Type {
	return types.Invalid
}

// Utils

func (c *common) ResolveAndAnalyzeType(typ ast.Type) types.Type {
	return ast.VisitType(c, typ)
}
