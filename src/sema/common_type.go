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
	return types.GetPrimitive(t.Kind)
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

func (c *common) VisitIdentifierType(t *ast.IdentifierType) types.Type {
	cached := c.nodeTypes[t]

	symbol, ok := c.GetSymbol(t.Path)
	if !ok {
		return types.Invalid
	}

	switch symbol.Kind {
	case symbols.Struct:
		if !core.IsNil(cached) {
			if c.checkTypeConstraints {
				s := symbol.Type.(*types.Struct)
				subs := make([]types.Substitution, len(s.TypeParams))

				for j, param := range s.TypeParams {
					argType := c.ResolveAndAnalyzeType(t.TypeArgs[j])
					if argType == types.Invalid {
						return types.Invalid
					}

					subs[j] = types.Substitution{Param: param, Type: argType}
				}

				for j, param := range s.TypeParams {
					if len(param.Constraints) > 0 {
						for _, con := range param.Constraints {
							if in, ok := c.instantiations.Substitute(con, subs).(*types.Interface); ok {
								c.CheckConstraint(subs[j].Type, in, t.TypeArgs[j])
							}
						}
					}
				}
			}

			return cached
		}

		s := symbol.Type.(*types.Struct)

		if t.Mutable {
			c.Error(t, "struct type '%s' cannot be mutable", s.Name)
		}

		// Non-generic
		if len(t.TypeArgs) == 0 {
			if len(s.TypeParams) > 0 {
				c.Error(t, "'%s' is a generic type and requires type arguments", s.Name)
				return types.Invalid
			}

			c.nodeTypes[t] = s
			return s
		}

		// Check generic parameter count
		if len(t.TypeArgs) != len(s.TypeParams) {
			c.Error(t, "'%s' expects %d type argument(s), got %d", s.Name, len(s.TypeParams), len(t.TypeArgs))
			return types.Invalid
		}

		subs := make([]types.Substitution, len(s.TypeParams))

		for j, param := range s.TypeParams {
			argType := c.ResolveAndAnalyzeType(t.TypeArgs[j])
			if argType == types.Invalid {
				return types.Invalid
			}

			subs[j] = types.Substitution{Param: param, Type: argType}
		}

		result := c.instantiations.Get(s, subs).(*types.Struct)

		c.nodeTypes[t] = result
		return result

	case symbols.Enum:
		if t.Mutable {
			c.Error(t, "enum type '%s' cannot be mutable", symbol.Name)
		}

		c.nodeTypes[t] = symbol.Type
		return symbol.Type

	case symbols.Interface:
		if !core.IsNil(cached) {
			if c.checkTypeConstraints {
				in := symbol.Type.(*types.Interface)
				subs := make([]types.Substitution, len(in.TypeParams))

				for j, param := range in.TypeParams {
					argType := c.ResolveAndAnalyzeType(t.TypeArgs[j])
					if argType == types.Invalid {
						return types.Invalid
					}

					subs[j] = types.Substitution{Param: param, Type: argType}
				}

				for j, param := range in.TypeParams {
					if len(param.Constraints) > 0 {
						for _, con := range param.Constraints {
							if in, ok := c.instantiations.Substitute(con, subs).(*types.Interface); ok {
								c.CheckConstraint(subs[j].Type, in, t.TypeArgs[j])
							}
						}
					}
				}
			}

			return cached
		}
		in := symbol.Type.(*types.Interface)

		// Non-generic
		if len(t.TypeArgs) == 0 {
			if len(in.TypeParams) > 0 {
				c.Error(t, "'%s' is a generic type and requires type arguments", in.Name)
				return types.Invalid
			}

			result := in
			if t.Mutable {
				result = in.AsMutable()
			}

			c.nodeTypes[t] = result
			return result
		}

		// Check generic parameter count
		if len(t.TypeArgs) != len(in.TypeParams) {
			c.Error(t, "'%s' expects %d type argument(s), got %d", in.Name, len(in.TypeParams), len(t.TypeArgs))
			return types.Invalid
		}

		subs := make([]types.Substitution, len(in.TypeParams))

		for j, param := range in.TypeParams {
			argType := c.ResolveAndAnalyzeType(t.TypeArgs[j])
			if argType == types.Invalid {
				return types.Invalid
			}

			subs[j] = types.Substitution{Param: param, Type: argType}
		}

		result := c.instantiations.Get(in, subs).(*types.Interface)
		if t.Mutable {
			result = result.AsMutable()
		}

		c.nodeTypes[t] = result
		return result

	case symbols.TypeParam:
		if t.Mutable {
			c.Error(t, "type parameter '%s' cannot be mutable", symbol.Name)
		}

		c.nodeTypes[t] = symbol.Type
		return symbol.Type

	case symbols.Func, symbols.Param, symbols.Var:
		c.Error(t, "'%s' cannot be used as a type", t.Path.LastName())
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

func (c *common) VisitBadType(_ *ast.BadType) types.Type {
	return types.Invalid
}

// Utils

func (c *common) ResolveAndAnalyzeType(typ ast.Type) types.Type {
	return ast.VisitType(c, typ)
}
