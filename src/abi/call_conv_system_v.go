package abi

import (
	"fireball/ast"
)

type callConvSystemV struct {
}

var SystemV CallConv = &callConvSystemV{}

func (c *callConvSystemV) Classify(type_ ast.Type) []Reg {
	switch type_ := type_.(type) {
	case *ast.PrimitiveType:
		if type_.Kind == ast.Void {
			return nil
		}

		size, _ := TypeInfo(AMD64, type_)
		class := SSE

		if type_.Kind == ast.Bool || type_.Kind.IsInteger() {
			class = Integer
		}

		return []Reg{{class, size}}

	case *ast.PointerType, ast.FuncType:
		return []Reg{{Integer, AMD64.WordSize}}

	case *ast.DeclType, *ast.ArrayType:
		fields := flatten(AMD64, c, type_, 0, nil)
		size := fields[len(fields)-1].offset + fields[len(fields)-1].size

		if size > 4*8 {
			return []Reg{{Memory, size}}
		}

		eightbytes := make([]Reg, alignTo(size, 8)/8)

		for i := range eightbytes {
			eightbytes[i].Size = 8
		}

		eightbytes[len(eightbytes)-1].Size = size - (uint32(len(eightbytes))-1)*8

		for _, field := range fields {
			startEightbyteIndex := field.offset / 8
			endEightbyteIndex := (field.offset + field.size - 1) / 8

			for i := startEightbyteIndex; i <= endEightbyteIndex; i++ {
				eightbytes[i].Class = mergeClasses(eightbytes[i].Class, field.class)
			}
		}

		for _, eightbyte := range eightbytes {
			if eightbyte.Class == Memory {
				return []Reg{{Memory, size}}
			}
		}

		if size > 2*8 && eightbytes[0].Class != SSE {
			return []Reg{{Memory, size}}
		}

		return eightbytes

	default:
		panic("abi.SystemV.Classify() - Invalid type")
	}
}

func mergeClasses(class1, class2 Class) Class {
	if class1 == class2 {
		return class1
	}

	if class1 == None {
		return class2
	}
	if class2 == None {
		return class1
	}

	if class1 == Memory || class2 == Memory {
		return Memory
	}

	if class1 == Integer || class2 == Integer {
		return Integer
	}

	return SSE
}
