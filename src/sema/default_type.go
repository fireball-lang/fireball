package sema

import (
	"fireball/ast"
	"fireball/types"
)

func defaultType(t types.Type) types.Type {
	switch t := t.(type) {
	case *types.Null:
		return t.Underlying()
	case *types.Integer:
		return defaultPrimitiveType(t.ToPrimitive())
	case *types.Primitive:
		return defaultPrimitiveType(t)

	default:
		return t
	}
}

func defaultPrimitiveType(t *types.Primitive) types.Type {
	switch t.Kind {
	case types.I8, types.I16:
		return types.PrimitiveI32
	case types.U8, types.U16:
		return types.PrimitiveU32
	default:
		return t
	}
}

func isLiteralExpr(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Null, *ast.Number, *ast.Bool, *ast.Character, *ast.String:
		return true

	case *ast.Prefix:
		return isLiteralExpr(e.Expr)
	case *ast.Postfix:
		return isLiteralExpr(e.Expr)
	case *ast.Binary:
		return isLiteralExpr(e.Left) && isLiteralExpr(e.Right)

	default: // Identifier, Index, Member, Call, Cast
		return false
	}
}
