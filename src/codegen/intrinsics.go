package codegen

import (
	"fireball/ast"
	"fireball/ir"
	"strings"
)

func getIntrinsicFuncLinkName(f *ast.Func) string {
	name := getIntrinsicName(f)

	switch name {
	// Memory

	case "memcpy":
		return getComplexIntrinsicFuncLinkName(f, "memcpy.p0.p0", 2)
	case "memmove":
		return getComplexIntrinsicFuncLinkName(f, "memmove.p0.p0", 2)
	case "memset":
		return getComplexIntrinsicFuncLinkName(f, "memset.p0", 2)

	// Math

	case "abs":
		return getSimpleIntrinsicFuncLinkName(f, "abs", true, false)
	case "min":
		if f.Params[0].Type.(*ast.PrimitiveType).Kind.IsFloating() {
			return getSimpleIntrinsicFuncLinkName(f, "minnum", false, false)
		}
		return getSimpleIntrinsicFuncLinkName(f, "min", false, true)
	case "max":
		if f.Params[0].Type.(*ast.PrimitiveType).Kind.IsFloating() {
			return getSimpleIntrinsicFuncLinkName(f, "maxnum", false, false)
		}
		return getSimpleIntrinsicFuncLinkName(f, "max", false, true)

	case "sqrt":
		return getSimpleIntrinsicFuncLinkName(f, "sqrt", false, false)

	case "pow":
		if type_, ok := f.Params[1].Type.(*ast.PrimitiveType); ok && type_.Kind.IsFloating() {
			return getSimpleIntrinsicFuncLinkName(f, "pow", false, false)
		} else {
			return getComplexIntrinsicFuncLinkName(f, "powi", 0, 1)
		}

	case "sin":
		return getSimpleIntrinsicFuncLinkName(f, "sin", false, false)
	case "asin":
		return getSimpleIntrinsicFuncLinkName(f, "asin", false, false)

	case "cos":
		return getSimpleIntrinsicFuncLinkName(f, "cos", false, false)
	case "acos":
		return getSimpleIntrinsicFuncLinkName(f, "acos", false, false)

	case "tan":
		return getSimpleIntrinsicFuncLinkName(f, "tan", false, false)
	case "atan":
		return getSimpleIntrinsicFuncLinkName(f, "atan", false, false)
	case "atan2":
		return getSimpleIntrinsicFuncLinkName(f, "atan2", false, false)

	case "exp":
		return getSimpleIntrinsicFuncLinkName(f, "exp", false, false)
	case "exp2":
		return getSimpleIntrinsicFuncLinkName(f, "exp2", false, false)
	case "exp10":
		return getSimpleIntrinsicFuncLinkName(f, "exp10", false, false)

	case "log":
		return getSimpleIntrinsicFuncLinkName(f, "log", false, false)
	case "log2":
		return getSimpleIntrinsicFuncLinkName(f, "log2", false, false)
	case "log10":
		return getSimpleIntrinsicFuncLinkName(f, "log10", false, false)

	case "fma":
		return getSimpleIntrinsicFuncLinkName(f, "fmuladd", false, false)

	case "floor":
		return getSimpleIntrinsicFuncLinkName(f, "floor", false, false)
	case "ceil":
		return getSimpleIntrinsicFuncLinkName(f, "ceil", false, false)
	case "trunc":
		return getSimpleIntrinsicFuncLinkName(f, "trunc", false, false)
	case "round":
		return getSimpleIntrinsicFuncLinkName(f, "round", false, false)

	// Bit manipulation

	case "reverse_bits":
		return getSimpleIntrinsicFuncLinkName(f, "bitreverse", false, false)
	case "reverse_bytes":
		return getSimpleIntrinsicFuncLinkName(f, "bswap", false, false)
	case "count_ones":
		return getSimpleIntrinsicFuncLinkName(f, "ctpop", false, false)
	case "count_leading_zeroes":
		return getSimpleIntrinsicFuncLinkName(f, "ctlz", false, false)
	case "count_trailing_zeroes":
		return getSimpleIntrinsicFuncLinkName(f, "cttz", false, false)

	// Unknown

	default:
		panic("codegen.getIntrinsicFuncLinkName() - Unknown '" + name + "' intrinsic")
	}
}

func getOverrideIntrinsicFuncTyp(t *TypeCache, f *ast.Func) *ir.FunctionType {
	switch getIntrinsicName(f) {
	case "memcpy", "memmove":
		typ := t.Get(f.Params[2].Type)

		return &ir.FunctionType{
			Returns: ir.Void,
			Params:  []ir.Type{ir.Pointer, ir.Pointer, typ, ir.I1},
			VarArgs: false,
		}

	case "memset":
		typ := t.Get(f.Params[2].Type)

		return &ir.FunctionType{
			Returns: ir.Void,
			Params:  []ir.Type{ir.Pointer, ir.I8, typ, ir.I1},
			VarArgs: false,
		}

	case "abs":
		if f.Params[0].Type.(*ast.PrimitiveType).Kind.IsFloating() {
			return nil
		}

		typ := t.Get(f.Params[0].Type)

		return &ir.FunctionType{
			Returns: typ,
			Params:  []ir.Type{typ, ir.I1},
			VarArgs: false,
		}

	case "count_leading_zeroes", "count_trailing_zeroes":
		typ := t.Get(f.Params[0].Type)

		return &ir.FunctionType{
			Returns: typ,
			Params:  []ir.Type{typ, ir.I1},
			VarArgs: false,
		}

	default:
		return nil
	}
}

func appendIntrinsicFuncParamNames(f *ast.Func, paramNames []string) []string {
	switch getIntrinsicName(f) {
	case "memcpy", "memmove", "memset":
		return append(paramNames, "is_volatile")

	case "abs":
		if f.Params[0].Type.(*ast.PrimitiveType).Kind.IsFloating() {
			return paramNames
		}

		return append(paramNames, "is_int_min_poison")

	case "count_leading_zeroes", "count_trailing_zeroes":
		return append(paramNames, "is_zero_poison")

	default:
		return paramNames
	}
}

func appendIntrinsicCallArgs(f *ast.Func, args []ir.Value) []ir.Value {
	switch getIntrinsicName(f) {
	case "memcpy", "memmove", "memset":
		return append(args, ir.False)

	case "abs":
		if f.Params[0].Type.(*ast.PrimitiveType).Kind.IsFloating() {
			return args
		}

		return append(args, ir.False)

	case "count_leading_zeroes", "count_trailing_zeroes":
		return append(args, ir.False)

	default:
		return args
	}
}

// Utils

func getIntrinsicName(f *ast.Func) string {
	attribute := f.GetAttribute("intrinsic")

	//goland:noinspection GoMaybeNil
	if attribute.Param != "" {
		return attribute.Param
	}

	return f.Name()
}

func getSimpleIntrinsicFuncLinkName(f *ast.Func, name string, floatingPrefix bool, signedPrefix bool) string {
	var sb strings.Builder
	sb.WriteString("llvm.")

	kind := f.Params[0].Type.(*ast.PrimitiveType).Kind

	if floatingPrefix {
		if kind.IsFloating() {
			sb.WriteRune('f')
		}
	}

	if signedPrefix {
		if kind.IsSignedInteger() {
			sb.WriteRune('s')
		} else if kind.IsUnsignedInteger() {
			sb.WriteRune('u')
		}
	}

	sb.WriteString(name)
	sb.WriteRune('.')
	sb.WriteString(kind.String())

	return sb.String()
}

func getComplexIntrinsicFuncLinkName(f *ast.Func, name string, paramIndices ...int) string {
	var sb strings.Builder

	sb.WriteString("llvm.")
	sb.WriteString(name)

	for _, index := range paramIndices {
		sb.WriteRune('.')
		sb.WriteString(f.Params[index].Type.(*ast.PrimitiveType).Kind.String())
	}

	return sb.String()
}
