package sema

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/symbols"
	"fireball/types"
)

// Visitor

func (a *analyzer) VisitPrimitiveType(p *ast.PrimitiveType) {
	a.typ = types.GetPrimitive(p.Kind)
}

func (a *analyzer) VisitArrayType(t *ast.ArrayType) {
	element := a.AnalyzeType(t.Type)
	if element == types.Invalid {
		a.typ = types.Invalid
		return
	}

	if element == types.PrimitiveVoid {
		a.Error(t, "arrays of voids are not allowed")
		a.typ = types.Invalid
		return
	}

	size := lexer.ParseInteger(t.Size)
	if size == 0 {
		a.Error(t, "zero-sized arrays are not allowed")
		a.typ = types.Invalid
		return
	}

	a.typ = &types.Array{
		Size:    size,
		Element: element,
	}
}

func (a *analyzer) VisitPointerType(p *ast.PointerType) {
	pointee := a.AnalyzeType(p.Pointee)
	if pointee == types.Invalid {
		a.typ = types.Invalid
		return
	}

	a.typ = &types.Pointer{
		Pointee: pointee,
	}
}

func (a *analyzer) VisitIdentifierType(i *ast.IdentifierType) {
	symbol, ok := a.scope.Get(i.Token.Text)
	if !ok {
		a.Error(i, "unknown type '%s'", i.Token.Text)
		a.typ = types.Invalid
		return
	}

	switch symbol.Kind {
	case symbols.Struct:
		a.typ = symbol.Type

	case symbols.Func, symbols.Param, symbols.Var:
		a.Error(i, "'%s' cannot be used as a type", i.Token.Text)
		a.typ = types.Invalid

	default:
		panic("sema.analyzer.VisitIdentifierType() - Invalid kind")
	}
}

func (a *analyzer) VisitBadType(_ *ast.BadType) {
	a.typ = types.Invalid
}

// Utils

func (a *analyzer) AnalyzeType(type_ ast.Type) types.Type {
	type_.VisitType(a)

	typ := a.typ
	a.typ = nil

	return typ
}
