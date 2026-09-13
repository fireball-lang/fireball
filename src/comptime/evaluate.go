package comptime

import (
	"errors"
	"fireball/abi"
	"fireball/ast"
	"fireball/codegen"
	"fireball/core"
	"fireball/fb-core"
	"fireball/ir"
	"fireball/ir/eval"
	"fireball/lexer"
	"fireball/sema"
	"fireball/types"
	"fmt"
)

type compTime struct {
	file           *ast.File
	instantiations *types.InstantiationCache
	typeEnv        *sema.TypeEnvironment
	fileDataMap    map[*ast.File]codegen.FileData
	builtins       fb_core.Builtins

	diagnostics []core.Diagnostic
	evaluations map[ast.Expr]eval.Value
}

func Evaluate(file *ast.File, instantiations *types.InstantiationCache, typeEnv *sema.TypeEnvironment, fileDataMap map[*ast.File]codegen.FileData, builtins fb_core.Builtins) (map[ast.Expr]eval.Value, []core.Diagnostic) {
	defer core.Scope()()

	ct := compTime{
		file:           file,
		instantiations: instantiations,
		typeEnv:        typeEnv,
		fileDataMap:    fileDataMap,
		builtins:       builtins,

		diagnostics: nil,
		evaluations: make(map[ast.Expr]eval.Value),
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Const:
			if !fileDataMap[file].ExprInfos[decl.Value].CompTime {
				continue
			}

			ok := ct.CheckCompTimeExprRoot(decl.Value, decl.Name())
			if !ok {
				continue
			}

			value, ok := ct.EvalExpr(decl.Value, decl.Type)

			if ok {
				ct.evaluations[decl.Value] = value
			}

		case *ast.GlobalVar:
			if core.IsNil(decl.Initializer) || !fileDataMap[file].ExprInfos[decl.Initializer].CompTime {
				continue
			}

			ok := ct.CheckCompTimeExprRoot(decl.Initializer, decl.Name())
			if !ok {
				continue
			}

			initializer, ok := ct.EvalExpr(decl.Initializer, decl.Type)

			if ok {
				ct.evaluations[decl.Initializer] = initializer
			}
		}
	}

	return ct.evaluations, ct.diagnostics
}

func (ct *compTime) EvalExpr(expr ast.Expr, typ ast.Type) (eval.Value, bool) {
	// Get literal value directly
	value := ct.GetLiteralEvalValue(expr)
	if value != nil {
		return value, true
	}

	// Generate IR module
	module := ir.NewModule()
	module.Path = "__comptime__"

	c := codegen.New(module, ct.file, abi.AMD64, abi.SystemV, ct.instantiations, ct.typeEnv, ct.fileDataMap, ct.builtins, true)
	fun := generateModule(c, expr, typ)

	// Evaluate IR module
	e := eval.NewEvaluator()
	e.Load(module)

	reg, err := e.Run(fun)

	if err != nil {
		if err, ok := errors.AsType[eval.Error](err); ok && err.File != "" {
			ct.diagnostics = append(ct.diagnostics, getDiagnostic(err))
			return nil, false
		}

		panic("comptime.Evaluate() - " + err.Error())
	}

	return e.GetValue(reg, c.Types.Get(c.NodeType(typ))), true
}

func (ct *compTime) CheckCompTimeExprRoot(expr ast.Expr, errNode ast.Node) bool {
	seen := make(map[ast.Expr]any)
	seen[expr] = nil

	return ct.CheckCompTimeExpr(expr, seen, errNode)
}

func (ct *compTime) CheckCompTimeExpr(expr ast.Expr, seen map[ast.Expr]any, errNode ast.Node) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		switch node := ct.fileDataMap[ast.GetFile(expr)].ExprInfos[expr].Node.(type) {
		case *ast.Const:
			if _, ok := seen[node.Value]; ok {
				ct.diagnostics = append(ct.diagnostics, core.Diagnostic{
					Kind:    core.Error,
					Path:    ast.GetFile(expr).Path,
					Range:   errNode.Range(),
					Message: fmt.Sprintf("cyclic reference of constant '%s'", node.Name().Token.Text),
				})

				return false
			}

			seen[node.Value] = nil
			ct.CheckCompTimeExpr(node.Value, seen, errNode)
			delete(seen, node.Value)
		}

	default:
		for child := range expr.Children() {
			if child, ok := child.(ast.Expr); ok {
				if !ct.CheckCompTimeExpr(child, seen, errNode) {
					return false
				}
			}
		}
	}

	return true
}

func (ct *compTime) GetLiteralEvalValue(expr ast.Expr) eval.Value {
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
			value, err := lexer.ParseDecimal(expr.Token)
			if err != nil {
				panic("comptime.getLiteralEvalValue() - Failed to parse float '" + expr.Token.Text + "'")
			}

			return &eval.FloatValue{Value: value}
		}

		// Double
		if expr.Token.Kind == lexer.Decimal {
			value, err := lexer.ParseDecimal(expr.Token)
			if err != nil {
				panic("comptime.getLiteralEvalValue() - Failed to parse double '" + expr.Token.Text + "'")
			}

			return &eval.FloatValue{Value: value}
		}

		// Unknown
		panic("comptime.getLiteralEvalValue() - Invalid token kind")

	case *ast.Character:
		return &eval.IntValue{Value: uint64(expr.Rune)}

	case *ast.String:
		fields := abi.AMD64.Info(ct.builtins.StringView).Fields

		ptrI := 0
		if ct.builtins.StringView.Fields[fields[0].Index].Name == "size" {
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

func generateModule(c *codegen.Codegen, expr ast.Expr, typ ast.Type) *ir.Function {
	evalFunc := &ast.Func{
		Name_:   &ast.Leaf{Token: lexer.Token{Kind: lexer.Identifier, Text: "__comptime__"}},
		Returns: typ,
	}

	evalFile := &ast.File{
		Mod: &ast.Mod{Path: []*ast.Leaf{
			{Token: lexer.Token{Kind: lexer.Identifier, Text: "__comptime__"}},
		}},
		Decls: []ast.Decl{evalFunc},
	}

	evalFunc.SetParent(evalFile)

	funTyp := &types.Func{Returns: c.NodeType(typ)}

	fun := c.CreateFunction(
		evalFunc,
		funTyp,
		false,
		nil,
	)

	c.BeginFunc(evalFunc, funTyp, fun)

	val := c.LoadImplicitCast(expr, c.UnderlyingNodeType(typ))
	c.Emitter.Ret(val)

	c.EndFunc()

	return fun
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
