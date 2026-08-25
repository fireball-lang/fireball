package sema

import (
	"fireball/ast"
	"fireball/types"
)

func (a *analyzer) CheckIntrinsic(f *ast.Func) {
	intrinsic := ast.GetAttribute[*ast.Intrinsic](f)
	kind := intrinsic.Kind

	if f.VarArgs {
		a.Error(intrinsic, "intrinsic '%s' does not take variable arguments", kind)
	}

	switch kind {
	case ast.Syscall:
		a.IntrinsicExpectPrimitiveClass(kind, f.Returns, "64-bit integer", is64Bit)

		if len(f.Params) == 0 {
			a.Error(intrinsic, "intrinsic 'syscall' needs to have at least 1 parameter")
		}
		if len(f.Params) > 7 {
			a.Error(intrinsic, "intrinsic 'syscall' can have at most 7 parameters")
		}

		for _, param := range f.Params {
			a.IntrinsicExpectPrimitiveClass(kind, param.Type, "64-bit integer", is64Bit)
		}

	case ast.Memcpy, ast.Memmove:
		a.IntrinsicExpectPrimitiveKind(kind, f.Returns, types.Void)

		if a.IntrinsicExpectParamCount(f, 3) {
			return
		}

		a.IntrinsicExpectPointer(kind, f.Params[0].Type)
		a.IntrinsicExpectPointer(kind, f.Params[1].Type)
		a.IntrinsicExpectPrimitiveKind(kind, f.Params[2].Type, types.U64)

	case ast.Memset:
		a.IntrinsicExpectPrimitiveKind(kind, f.Returns, types.Void)

		if a.IntrinsicExpectParamCount(f, 3) {
			return
		}

		a.IntrinsicExpectPointer(kind, f.Params[0].Type)
		a.IntrinsicExpectPrimitiveKind(kind, f.Params[1].Type, types.U8)
		a.IntrinsicExpectPrimitiveKind(kind, f.Params[2].Type, types.U64)

	default:
		panic("sema.analyzer.CheckIntrinsic() - Invalid intrinsic")
	}
}

func is64Bit(p types.PrimitiveKind) bool {
	return p == types.U64 || p == types.I64
}

func (a *analyzer) IntrinsicExpectParamCount(f *ast.Func, expected int) bool {
	if len(f.Params) != expected {
		intrinsic := ast.GetAttribute[*ast.Intrinsic](f)
		a.Error(intrinsic, "intrinsic '%s' expected %d parameters, got %d", intrinsic.Kind, expected, len(f.Params))

		return true
	}

	return false
}

func (a *analyzer) IntrinsicExpectPrimitiveKind(kind ast.IntrinsicKind, t ast.Type, primKind types.PrimitiveKind) {
	typ := a.nodeTypes[t]

	if typ, ok := typ.(*types.Primitive); ok && typ.Kind == primKind {
		return
	}

	a.Error(t, "intrinsic '%s' expected '%s', got '%s'", kind, primKind, typ)
}

func (a *analyzer) IntrinsicExpectPrimitiveClass(kind ast.IntrinsicKind, t ast.Type, name string, predicate func(types.PrimitiveKind) bool) {
	typ := a.nodeTypes[t]

	if typ, ok := typ.(*types.Primitive); ok && predicate(typ.Kind) {
		return
	}

	a.Error(t, "intrinsic '%s' expected %s type, got '%s'", kind, name, typ)
}

func (a *analyzer) IntrinsicExpectPointer(kind ast.IntrinsicKind, t ast.Type) {
	typ := a.nodeTypes[t]

	if _, ok := typ.(*types.Pointer); ok {
		return
	}

	a.Error(t, "intrinsic '%s' expected pointer type, got '%s'", kind, typ)
}
