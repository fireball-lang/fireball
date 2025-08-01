package analyzer

import (
	"fireball/ast"
	"fmt"
)

func (a *analyzer) checkIntrinsic(f *ast.Func, errorNode ast.Node, attribute *ast.Attribute) {
	name := attribute.Param

	if name == "" {
		name = f.Name()
	}

	switch name {
	// Memory

	case "memcpy":
		a.checkComplexIntrinsic(f, errorNode, false, void, pointer, pointer, unsigned|min4bytes)
	case "memmove":
		a.checkComplexIntrinsic(f, errorNode, false, void, pointer, pointer, unsigned|min4bytes)
	case "memset":
		a.checkComplexIntrinsic(f, errorNode, false, void, pointer, signed|unsigned|max1byte, unsigned|min4bytes)

	// Math

	case "abs":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating|signed)
	case "min":
		a.checkSimpleIntrinsic(f, errorNode, 2, floating|signed|unsigned)
	case "max":
		a.checkSimpleIntrinsic(f, errorNode, 2, floating|signed|unsigned)

	case "sqrt":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)

	case "pow":
		if len(f.Params) > 1 {
			if type_, ok := f.Params[1].Type.(*ast.PrimitiveType); ok && type_.Kind.IsFloating() {
				a.checkSimpleIntrinsic(f, errorNode, 2, floating)
				return
			}
		}

		a.checkComplexIntrinsic(f, errorNode, true, floating, floating, signed|min4bytes|max4bytes)

	case "sin":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "asin":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)

	case "cos":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "acos":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)

	case "tan":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "atan":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "atan2":
		a.checkSimpleIntrinsic(f, errorNode, 2, floating)

	case "exp":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "exp2":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "exp10":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)

	case "log":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "log2":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "log10":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)

	case "fma":
		a.checkSimpleIntrinsic(f, errorNode, 3, floating)

	case "floor":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "ceil":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "trunc":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)
	case "round":
		a.checkSimpleIntrinsic(f, errorNode, 1, floating)

	// Bit manipulation

	case "reverse_bits":
		a.checkSimpleIntrinsic(f, errorNode, 1, signed|unsigned)
	case "reverse_bytes":
		a.checkSimpleIntrinsic(f, errorNode, 1, signed|unsigned|min2bytes)
	case "count_ones":
		a.checkSimpleIntrinsic(f, errorNode, 1, signed|unsigned)
	case "count_leading_zeroes":
		a.checkSimpleIntrinsic(f, errorNode, 1, signed|unsigned)
	case "count_trailing_zeroes":
		a.checkSimpleIntrinsic(f, errorNode, 1, signed|unsigned)

	// Unknown

	default:
		a.error(errorNode, "Unknown '"+name+"' intrinsic.")
	}
}

// Utils

type intrinsicTypeFlags uint16

const (
	void intrinsicTypeFlags = 1 << iota
	pointer
	floating
	signed
	unsigned
	min2bytes
	min4bytes
	max1byte
	max4bytes
)

func (a *analyzer) checkSimpleIntrinsic(f *ast.Func, errorNode ast.Node, paramCount int, paramTypeFlags intrinsicTypeFlags) {
	if len(f.Params) != paramCount {
		a.error(errorNode, fmt.Sprintf("Intrinsic requires %d parameters but got %d.", paramCount, len(f.Params)))
	}

	if len(f.Params) == 0 {
		return
	}

	type_ := f.Params[0].Type

	if !ast.IsValid(type_) {
		return
	}
	if !a.checkIntrinsicType(type_, paramTypeFlags) {
		a.error(type_, "Invalid parameter type, '"+type_.String()+"' not allowed in this intrinsic.")
		return
	}

	for i := 1; i < min(paramCount, len(f.Params)); i++ {
		paramType := f.Params[i].Type

		if ast.IsValid(paramType) && !type_.Equals(paramType) {
			a.error(paramType, "Parameter types need to match.")
		}
	}

	if ast.IsValid(f.ReturnType()) && !type_.Equals(f.ReturnType()) {
		a.error(f.ReturnType(), "Parameter and return types need to match.")
	}
}

func (a *analyzer) checkComplexIntrinsic(f *ast.Func, errorNode ast.Node, returnAndFirstParamEqual bool, returnTypeFlags intrinsicTypeFlags, paramsTypeFlags ...intrinsicTypeFlags) {
	if len(f.Params) != len(paramsTypeFlags) {
		a.error(errorNode, fmt.Sprintf("Intrinsic requires %d parameters but got %d.", len(paramsTypeFlags), len(f.Params)))
	}

	for i := 1; i < min(len(paramsTypeFlags), len(f.Params)); i++ {
		paramType := f.Params[i].Type

		if ast.IsValid(paramType) {
			a.checkIntrinsicType(paramType, paramsTypeFlags[i])
		}
	}

	if ast.IsValid(f.ReturnType()) {
		a.checkIntrinsicType(f.ReturnType(), returnTypeFlags)

		if returnAndFirstParamEqual && len(f.Params) > 0 && ast.IsValid(f.Params[0].Type) && !f.Params[0].Type.Equals(f.ReturnType()) {
			a.error(f.ReturnType(), "Return type needs to be the same as the first parameter.")
		}
	}
}

func (a *analyzer) checkIntrinsicType(type_ ast.Type, flags intrinsicTypeFlags) bool {
	if type_, ok := type_.(*ast.PrimitiveType); ok {
		if type_.Kind.BitCount() < 16 && flags&min2bytes != 0 {
			return false
		}
		if type_.Kind.BitCount() < 32 && flags&min4bytes != 0 {
			return false
		}

		if type_.Kind.BitCount() > 8 && flags&max1byte != 0 {
			return false
		}
		if type_.Kind.BitCount() > 32 && flags&max4bytes != 0 {
			return false
		}

		if type_.Kind == ast.Void {
			return flags&void != 0
		}
		if type_.Kind.IsFloating() {
			return flags&floating != 0
		}
		if type_.Kind.IsSignedInteger() {
			return flags&signed != 0
		}
		if type_.Kind.IsUnsignedInteger() {
			return flags&unsigned != 0
		}
	}

	if type_, ok := type_.(*ast.PointerType); ok {
		if pointee, ok := type_.Pointee.(*ast.PrimitiveType); ok && pointee.Kind == ast.Void {
			return flags&pointer != 0
		}
	}

	return false
}
