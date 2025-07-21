package analyzer

import (
	"fireball/ast"
	"fireball/utils"
)

type CastKind uint8

const (
	Nop CastKind = iota
	Extend
	Truncate
	IntegerToFloating
	FloatingToInteger
	IntegerToPointer
	PointerToInteger
	PointerToInterface
	InterfaceToPointer
)

func GetCastKind(ctx Context, from, to ast.Type, allowExtended bool) (CastKind, bool) {
	switch from := from.(type) {
	case *ast.PrimitiveType:
		if to, ok := to.(*ast.PrimitiveType); ok {
			if (from.Kind.IsInteger() && to.Kind.IsInteger()) || (from.Kind.IsFloating() && to.Kind.IsFloating()) {
				if from.Kind.BitCount() == to.Kind.BitCount() {
					return Nop, true
				}
				if from.Kind.BitCount() > to.Kind.BitCount() {
					return Truncate, true
				}
				return Extend, true
			}

			if from.Kind.IsInteger() && to.Kind.IsFloating() {
				return IntegerToFloating, true
			}

			if from.Kind.IsFloating() && to.Kind.IsInteger() {
				return FloatingToInteger, true
			}

			if allowExtended {
				if from.Kind == ast.Bool && to.Kind.IsInteger() {
					return Extend, true
				}

				if from.Kind.IsInteger() && to.Kind == ast.Bool {
					return Truncate, true
				}
			}
		}

		if from.Kind.IsInteger() {
			if to, ok := to.(*ast.DeclType); ok {
				if decl, ok := to.Decl.(*ast.Enum); ok {
					return GetCastKind(ctx, from, decl.ActualType, false)
				}
			}
		}

		if allowExtended {
			_, toIsPointer := to.(*ast.PointerType)
			_, toIsFunc := to.(ast.FuncType)

			if from.Kind.IsInteger() && (toIsPointer || toIsFunc) {
				return IntegerToPointer, true
			}
		}

	case *ast.PointerType:
		if _, ok := to.(*ast.PointerType); ok {
			return Nop, true
		}

		if _, ok := to.(ast.FuncType); ok {
			return Nop, true
		}

		if to, ok := to.(*ast.PrimitiveType); allowExtended && ok && to.Kind.IsInteger() {
			return PointerToInteger, true
		}

		if from, ok := from.Pointee.(*ast.DeclType); ok {
			if in, ok := ast.GetDeclFromDeclType[*ast.Interface](to); ok && declImplementsInterface(ctx, from.Decl, in) {
				return PointerToInterface, true
			}
		}

	case *ast.DeclType:
		if decl, ok := from.Decl.(*ast.Enum); ok {
			if p, ok := to.(*ast.PrimitiveType); ok && p.Kind.IsInteger() {
				return GetCastKind(ctx, decl.ActualType, p, false)
			}
		}

		if _, ok := from.Decl.(*ast.Interface); ok {
			if to, ok := to.(*ast.PointerType); ok {
				if _, ok := to.Pointee.(*ast.DeclType); ok {
					return InterfaceToPointer, true
				}
			}
		}

	case ast.FuncType:
		if to, ok := to.(*ast.PrimitiveType); allowExtended && ok && to.Kind.IsInteger() {
			return PointerToInteger, true
		}
	}

	return Nop, false
}

func GetImplicitCastKind(ctx Context, from, to ast.Type) (CastKind, bool) {
	switch from := from.(type) {
	case *ast.PrimitiveType:
		if to, ok := to.(*ast.PrimitiveType); ok {
			if (from.Kind.IsSignedInteger() && to.Kind.IsSignedInteger()) || (from.Kind.IsUnsignedInteger() && to.Kind.IsUnsignedInteger()) || (from.Kind.IsFloating() && to.Kind.IsFloating()) {
				if from.Kind.BitCount() < to.Kind.BitCount() {
					return Extend, true
				}
			}
		}

	case *ast.PointerType:
		if to, ok := to.(*ast.PointerType); ok && to.Pointee.Equals(ast.VoidType) {
			return Nop, true
		}

		if from, ok := from.Pointee.(*ast.DeclType); ok {
			if in, ok := ast.GetDeclFromDeclType[*ast.Interface](to); ok && declImplementsInterface(ctx, from.Decl, in) {
				return PointerToInterface, true
			}
		}
	}

	return Nop, false
}

func declImplementsInterface(ctx Context, decl ast.Decl, in *ast.Interface) bool {
	if utils.IsNil(ctx) {
		return true
	}

	modPath := ast.Root(decl).ModulePath()
	mod := ctx.GetAbsoluteModule(modPath)

	return mod.DeclImplementsInterface(decl, in)
}
