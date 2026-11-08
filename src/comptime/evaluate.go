package comptime

import (
	"errors"
	"fireball/abi"
	"fireball/ast"
	"fireball/codegen"
	"fireball/core"
	"fireball/ir"
	"fireball/ir/eval"
	"fireball/lexer"
	"fireball/sema"
	"fireball/types"
	"fmt"
)

var Arch = abi.AMD64
var CallConv = abi.SystemV

var valueTrue eval.Value = &eval.IntValue{Value: 1}
var valueFalse eval.Value = &eval.IntValue{Value: 0}

func (e *Evaluator) EvaluateFile(file *ast.File) {
	defer core.Scope()()

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Enum:
			e.ensureEnum(decl, decl.Name())

		case *ast.Impl:
			for _, assocConst := range decl.AssociatedConsts {
				e.evaluateInitializer(assocConst.Value, assocConst.Type, assocConst.Name)
			}

			for _, method := range decl.Methods {
				if !core.IsNil(method.Body) {
					e.evaluateStmt(method.Body)
				}
			}

		case *ast.Const:
			e.evaluateInitializer(decl.Value, decl.Type, decl.Name())

		case *ast.GlobalVar:
			if core.IsNil(decl.Initializer) {
				continue
			}

			e.evaluateInitializer(decl.Initializer, decl.Type, decl.Name())

		case *ast.Func:
			if !core.IsNil(decl.Body) {
				e.evaluateStmt(decl.Body)
			}
		}
	}
}

// Stmt

func (e *Evaluator) evaluateStmt(stmt ast.Stmt) {
	switch stmt := stmt.(type) {
	case *ast.Match:
		fd := e.fileData(stmt)
		typ := fd.ExprInfos[stmt.Value].Type
		if typ == types.Invalid {
			return
		}

		// Evaluate patterns
		count := 0
		patternsOK := true
		fallbackExpr := ast.Expr(nil)

		for _, arm := range stmt.Arms {
			// Evaluate
			for i, pattern := range fd.ArmPatterns[arm] {
				switch pattern := pattern.(type) {
				case *sema.ExprPattern:
					if e.ensureExpr(pattern.Expr, typ, pattern.Expr) {
						count++
					} else {
						patternsOK = false
					}

				case *sema.FallbackPattern:
					fallbackExpr = arm.Patterns[i]
				}
			}
		}

		// Check for duplicates
		seen := make([]eval.Value, 0, count)

		for _, arm := range stmt.Arms {
		pattern:
			for i, pattern := range fd.ArmPatterns[arm] {
				if pattern, ok := pattern.(*sema.ExprPattern); ok {
					if val, ok := fd.Evaluations[pattern.Expr]; ok {
						for _, value := range seen {
							if value.Equals(val) {
								e.error(arm.Patterns[i], "duplicate expression pattern")
								continue pattern
							}
						}

						seen = append(seen, val)
					}
				}
			}
		}

		// Check exhaustiveness
		exhaustive := false

		if typ == types.PrimitiveBool {
			if len(seen) == 2 && ((seen[0].Equals(valueTrue) && seen[1].Equals(valueFalse)) || (seen[0].Equals(valueFalse) && seen[1].Equals(valueTrue))) {
				exhaustive = true
			}
		}

		if patternsOK && fallbackExpr == nil && !exhaustive {
			e.error(stmt.Value, "missing fallback arm, match is not exhaustive")
		} else if patternsOK && fallbackExpr != nil && exhaustive {
			e.error(fallbackExpr, "fallback arm is not needed, match is fully exhaustive")
		}

		// Arm statements
		for _, arm := range stmt.Arms {
			e.evaluateStmt(arm.Body)
		}

	default:
		for child := range stmt.Children() {
			if child, ok := child.(ast.Stmt); ok {
				e.evaluateStmt(child)
			}
		}
	}
}

// Utils

func (e *Evaluator) evaluateInitializer(value ast.Expr, typ ast.Type, errNode ast.Node) {
	e.ensureExpr(value, e.fileData(errNode).NodeTypes[typ], errNode)
}

func (e *Evaluator) evalExpr(expr ast.Expr, typ types.Type) (eval.Value, bool) {
	// Get literal value directly
	value := e.getLiteralEvalValue(expr, typ)
	if value != nil {
		return value, true
	}

	// Generate IR module
	file := ast.GetFile(expr)
	module := ir.NewModule()
	module.Path = file.Path

	c := codegen.New(e.ctx, module, file, Arch, CallConv, true)
	c.Types.ShallowMeta = true

	val, err, ok := c.GenerateComptimeValue(expr, typ)
	if !ok {
		if evalErr, isEvalErr := errors.AsType[eval.Error](err); isEvalErr && evalErr.File != "" {
			e.addDiagnostic(getDiagnostic(evalErr))
			return nil, false
		}

		panic("comptime.Evaluator.evalExpr() - " + err.Error())
	}

	return val, true
}

func (e *Evaluator) getLiteralEvalValue(expr ast.Expr, typ types.Type) eval.Value {
	switch expr.(type) {
	case *ast.Bool, *ast.Number, *ast.Character, *ast.String:
	default:
		return nil
	}

	kind, ok := sema.GetImplicitCast(e.ctx, e.fileData(expr).ExprInfos[expr], typ)
	if !ok {
		return nil
	}

	switch kind {
	case sema.Noop, sema.ZeroExtend, sema.SignExtend, sema.Truncate, sema.FloatExtend:
	default:
		return nil
	}

	switch expr := expr.(type) {
	case *ast.Bool:
		if expr.Value {
			return &eval.IntValue{Value: 1}
		}

		return &eval.IntValue{Value: 0}

	case *ast.Number:
		// Integer
		if lexer.IsInteger(expr.Token.Kind) {
			return &eval.IntValue{Value: lexer.ParseInteger(expr.Token).TwosComplement()}
		}

		// Float
		if expr.Token.Kind == lexer.Decimal32bit {
			value, _ := lexer.ParseDecimal(expr.Token)
			return &eval.FloatValue{Value: value}
		}

		// Double
		if expr.Token.Kind == lexer.Decimal {
			value, _ := lexer.ParseDecimal(expr.Token)
			return &eval.FloatValue{Value: value}
		}

		// Unknown
		panic("comptime.getLiteralEvalValue() - Invalid token kind")

	case *ast.Character:
		return &eval.IntValue{Value: uint64(expr.Rune)}

	case *ast.String:
		fields := abi.AMD64.Info(e.ctx.Builtins.StringView).Fields

		ptrI := 0
		if e.ctx.Builtins.StringView.Fields[fields[0].Index].Name == "size" {
			ptrI = 1
		}

		init := ir.NewString(expr.Runes, true)

		values := make([]eval.Value, 2)

		values[ptrI] = &eval.StringValue{Value: init}
		values[1-ptrI] = &eval.IntValue{Value: uint64(init.Size)}

		return &eval.AggregateValue{Values: values}

	default:
		return nil
	}
}

func (e *Evaluator) error(node ast.Node, format string, args ...any) {
	e.addDiagnostic(core.Diagnostic{
		Kind:    core.Error,
		Path:    ast.GetFile(node).Path,
		Range:   node.Range(),
		Message: fmt.Sprintf(format, args...),
	})
}

func integerFitsInKind(value core.Integer, kind types.PrimitiveKind) bool {
	kMin, kMax := kind.IntegerRange()
	return value.GreaterThanEqual(kMin) && value.LessThanEqual(kMax)
}

func getDiagnostic(err eval.Error) core.Diagnostic {
	return core.Diagnostic{
		Kind: core.Error,
		Path: err.File,
		Range: core.Range{
			Start: core.Pos{
				Line:   err.Line,
				Column: err.Column,
			},
			End: core.Pos{
				Line:   err.Line,
				Column: err.Column + 1,
			},
		},
		Message: err.Msg,
	}
}
