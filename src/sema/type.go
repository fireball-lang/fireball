package sema

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
	"math"
)

// Visitor

func (a *analyzer) VisitPrimitiveType(p *ast.PrimitiveType) types.Type {
	return types.GetPrimitive(p.Kind)
}

func (a *analyzer) VisitArrayType(t *ast.ArrayType) types.Type {
	element := a.AnalyzeType(t.Type)
	if element == types.Invalid {
		return types.Invalid
	}
	if element == types.PrimitiveVoid {
		return a.Error(t, "arrays of voids are not allowed").Type
	}

	size := lexer.ParseInteger(t.Size)
	if size == 0 {
		return a.Error(t, "zero-sized arrays are not allowed").Type
	}
	if size > math.MaxUint32 {
		return a.Error(t, "array size cannot be bigger than an unsigned 32-bit integer").Type
	}

	return &types.Array{
		Size:    uint32(size),
		Element: element,
	}
}

func (a *analyzer) VisitPointerType(p *ast.PointerType) types.Type {
	pointee := a.AnalyzeType(p.Pointee)
	if pointee == types.Invalid {
		return types.Invalid
	}

	return &types.Pointer{
		Mutable: p.Mutable,
		Pointee: pointee,
	}
}

func (a *analyzer) VisitIdentifierType(i *ast.IdentifierType) types.Type {
	symbol, ok := a.GetSymbol(i.Path)
	if !ok {
		return types.Invalid
	}

	switch symbol.Kind {
	case symbols.Struct:
		a.nodeTypes[i] = symbol.Type
		return symbol.Type

	case symbols.Func, symbols.Param, symbols.Var:
		a.Error(i, "'%s' cannot be used as a type", i.Path.LastName())
		return types.Invalid

	default:
		panic("sema.analyzer.VisitIdentifierType() - Invalid kind")
	}
}

func (a *analyzer) VisitBadType(_ *ast.BadType) types.Type {
	return types.Invalid
}

// Utils

func (a *analyzer) AnalyzeType(typ ast.Type) types.Type {
	return ast.VisitType(a, typ)
}
