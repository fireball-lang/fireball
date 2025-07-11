package analyzer

import (
	"fireball/ast"
	"fireball/utils"
	"math"
)

type typeResolver struct {
	ctx   Context
	scope Scope

	diagnostics []utils.Diagnostic
}

func ResolveTypes(file *ast.File, ctx Context, scope Scope) []utils.Diagnostic {
	resolver := &typeResolver{ctx: ctx}
	resolver.scope = getFileScope(file, ctx, scope, resolver)

	resolver.visit(file)

	return resolver.diagnostics
}

func (t *typeResolver) resolveEnum(e *ast.Enum) {
	// Case values
	value := utils.Signed(-1)

	minValue := utils.Unsigned(false, math.MaxUint64)
	maxValue := utils.Unsigned(true, math.MaxUint64)

	for _, c := range e.Cases {
		if c.Value != nil {
			v, err := parseInteger(c.Value.Token)

			if err != "" {
				addError(t, c.Value, err)
				value = value.AddOne()
			} else {
				value = v
			}
		} else {
			value = value.AddOne()
		}

		c.ActualValue = value

		minValue = minValue.Min(value)
		maxValue = maxValue.Max(value)
	}

	// Type
	e.ActualType = nil

	if ast.IsValid(e.Type) {
		var typeMinValue, typeMaxValue utils.Integer

		if p, ok := e.Type.(*ast.PrimitiveType); ok && p.Kind.IsInteger() {
			e.ActualType = p
			typeMinValue, typeMaxValue = p.Kind.IntegerBounds()
		}

		if !ast.IsValid(e.ActualType) {
			addError(t, e.Type, "Enum backing type needs be an integer.")
		} else {
			// Verify case values
			for _, c := range e.Cases {
				if c.ActualValue.LessThan(typeMinValue) || c.ActualValue.GreaterThan(typeMaxValue) {
					addError(t, e.Type, "Enum case value is outside the range of backing type.")
				}
			}
		}
	}

	if !ast.IsValid(e.ActualType) {
		// Infer type from case values
		if minValue.Negative() || maxValue.Negative() {
			if tryEnumType(e, minValue, maxValue, ast.I8Type) {
				return
			}
			if tryEnumType(e, minValue, maxValue, ast.I16Type) {
				return
			}
			if tryEnumType(e, minValue, maxValue, ast.I32Type) {
				return
			}
			e.ActualType = ast.I64Type
		}

		if tryEnumType(e, minValue, maxValue, ast.U8Type) {
			return
		}
		if tryEnumType(e, minValue, maxValue, ast.U16Type) {
			return
		}
		if tryEnumType(e, minValue, maxValue, ast.U32Type) {
			return
		}
		e.ActualType = ast.U64Type
	}
}

func tryEnumType(e *ast.Enum, minValue, maxValue utils.Integer, type_ *ast.PrimitiveType) bool {
	typeMinValue, typeMaxValue := type_.Kind.IntegerBounds()

	if minValue.GreaterThanEqual(typeMinValue) && maxValue.LessThanEqual(typeMaxValue) {
		e.ActualType = type_
		return true
	}

	return false
}

func (t *typeResolver) visit(node ast.Node) {
	switch node := node.(type) {
	case *ast.Enum:
		t.resolveEnum(node)

	case *ast.Impl:
		if node.NameN == nil {
			node.Struct = nil
		} else {
			decl := t.scope.GetTypeDecl(node.Name())

			if s, ok := decl.(*ast.Struct); ok {
				node.Struct = s
			} else {
				addError(t, node.NameN, "Type with the name '"+node.Name()+"' is not a struct.")
			}
		}

	case *ast.StructInitializer:
		decl := t.scope.GetTypeDecl(node.Name.Token.Text)

		if s, ok := decl.(*ast.Struct); ok {
			node.Struct = s
		} else {
			addError(t, node.Name, "Type with the name '"+node.Name.Token.Text+"' is not a struct.")
		}

	case *ast.DeclType:
		node.Decl = nil

		lookup := getSymbolLookup(t.ctx, t.scope, node.Path)

		if !utils.IsNil(lookup) {
			node.Decl = lookup.GetTypeDecl(node.Path.SegmentAt(node.Path.SegmentCount() - 1))
		}

		if utils.IsNil(node.Decl) {
			addError(t, node.Path, "Type with the path '"+ast.PathString(node.Path)+"' doesn't exist in the current scope.")
		}
	}

	for child := range node.Children() {
		t.visit(child)
	}
}

func (t *typeResolver) Add(diag utils.Diagnostic) {
	t.diagnostics = append(t.diagnostics, diag)
}
