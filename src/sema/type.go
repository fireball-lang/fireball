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
		s := symbol.Type.(*types.Struct)

		if i.Mutable {
			a.Error(i, "struct type '%s' cannot be mutable", s.Name)
		}

		// Non-generic
		if len(i.TypeArgs) == 0 {
			if len(s.TypeParams) > 0 {
				a.Error(i, "'%s' is a generic type and requires type arguments", s.Name)
				return types.Invalid
			}

			a.nodeTypes[i] = s
			return s
		}

		// Check generic parameter count
		if len(i.TypeArgs) != len(s.TypeParams) {
			a.Error(i, "'%s' expects %d type argument(s), got %d", s.Name, len(s.TypeParams), len(i.TypeArgs))
			return types.Invalid
		}

		subs := make([]types.Substitution, len(s.TypeParams))

		for j, param := range s.TypeParams {
			argType := a.AnalyzeType(i.TypeArgs[j])
			if argType == types.Invalid {
				return types.Invalid
			}

			subs[j] = types.Substitution{Param: param, Type: argType}
		}

		for j, param := range s.TypeParams {
			if len(param.Constraints) > 0 {
				for _, c := range param.Constraints {
					if in, ok := a.instantiations.Substitute(c, subs).(*types.Interface); ok {
						a.CheckConstraint(subs[j].Type, in, i.TypeArgs[j])
					}
				}
			}
		}

		result := a.instantiations.Get(s, subs).(*types.Struct)

		a.nodeTypes[i] = result
		return result

	case symbols.Interface:
		in := symbol.Type.(*types.Interface)

		// Non-generic
		if len(i.TypeArgs) == 0 {
			if len(in.TypeParams) > 0 {
				a.Error(i, "'%s' is a generic type and requires type arguments", in.Name)
				return types.Invalid
			}

			result := in
			if i.Mutable {
				result = in.AsMutable()
			}

			a.nodeTypes[i] = result
			return result
		}

		// Check generic parameter count
		if len(i.TypeArgs) != len(in.TypeParams) {
			a.Error(i, "'%s' expects %d type argument(s), got %d", in.Name, len(in.TypeParams), len(i.TypeArgs))
			return types.Invalid
		}

		subs := make([]types.Substitution, len(in.TypeParams))

		for j, param := range in.TypeParams {
			argType := a.AnalyzeType(i.TypeArgs[j])
			if argType == types.Invalid {
				return types.Invalid
			}

			subs[j] = types.Substitution{Param: param, Type: argType}
		}

		for j, param := range in.TypeParams {
			if len(param.Constraints) > 0 {
				for _, c := range param.Constraints {
					if in, ok := a.instantiations.Substitute(c, subs).(*types.Interface); ok {
						a.CheckConstraint(subs[j].Type, in, i.TypeArgs[j])
					}
				}
			}
		}

		result := a.instantiations.Get(in, subs).(*types.Interface)
		if i.Mutable {
			result = result.AsMutable()
		}

		a.nodeTypes[i] = result
		return result

	case symbols.TypeParam:
		if i.Mutable {
			a.Error(i, "type parameter '%s' cannot be mutable", symbol.Name)
		}

		a.nodeTypes[i] = symbol.Type
		return symbol.Type

	case symbols.Func, symbols.Param, symbols.Var:
		a.Error(i, "'%s' cannot be used as a type", i.Path.LastName())
		return types.Invalid

	default:
		panic("sema.analyzer.VisitIdentifierType() - Invalid kind")
	}
}

func (a *analyzer) VisitSelfType(s *ast.SelfType) types.Type {
	if a.selfType == nil {
		return a.Error(s, "'Self' can only be used inside an impl or interface block").Type
	}

	a.nodeTypes[s] = a.selfType
	return a.selfType
}

func (a *analyzer) VisitBadType(_ *ast.BadType) types.Type {
	return types.Invalid
}

// Utils

func (a *analyzer) AnalyzeType(typ ast.Type) types.Type {
	return ast.VisitType(a, typ)
}
