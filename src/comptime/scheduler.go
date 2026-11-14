package comptime

import (
	"fireball/ast"
	"fireball/core"
	"fireball/ir/eval"
	"fireball/sema"
	"fireball/types"
	"strings"
)

type evaluationState uint8

const (
	unseen evaluationState = iota
	evaluating
	done
	failed
	deferred
)

// Evaluator owns one comptime phase, including requests made across file and
// project boundaries. The file traversal is exhaustive, but requests are lazy.
type Evaluator struct {
	ctx         *sema.Context
	states      map[ast.Node]evaluationState
	stack       []ast.Node
	diagnostics map[string][]core.Diagnostic
}

func NewEvaluator(ctx *sema.Context) *Evaluator {
	for _, fd := range ctx.Files {
		fd.Evaluations = make(map[ast.Expr]eval.Value)
	}

	return &Evaluator{
		ctx:         ctx,
		states:      make(map[ast.Node]evaluationState),
		diagnostics: make(map[string][]core.Diagnostic),
	}
}

func (e *Evaluator) Diagnostics(file *ast.File) []core.Diagnostic {
	return e.diagnostics[file.Path]
}

func (e *Evaluator) addDiagnostic(diagnostic core.Diagnostic) {
	e.diagnostics[diagnostic.Path] = append(e.diagnostics[diagnostic.Path], diagnostic)
}

func (e *Evaluator) fileData(node ast.Node) *sema.FileData {
	return e.ctx.Files[ast.GetFile(node)]
}

func (e *Evaluator) request(node, errNode ast.Node, run func() evaluationState) bool {
	switch e.states[node] {
	case unseen:
		e.states[node] = evaluating
		e.stack = append(e.stack, node)

		state := run()

		e.stack = e.stack[:len(e.stack)-1]
		e.states[node] = state

		return state == done || state == deferred

	case done, deferred:
		return true

	case failed:
		return false

	case evaluating:
		var chain []string

		for _, active := range e.stack {
			chain = append(chain, evaluationName(active))
		}
		chain = append(chain, evaluationName(node))

		e.error(errNode, "cyclic compile-time dependency: %s", strings.Join(chain, " -> "))
		return false

	default:
		panic("comptime.Evaluator.request() - Invalid evaluation state")
	}
}

func evaluationName(node ast.Node) string {
	if enum, ok := node.(*ast.Enum); ok {
		return "enum '" + enum.Name().Token.Text + "'"
	}

	switch parent := node.Parent().(type) {
	case *ast.Const:
		return "constant '" + parent.Name().Token.Text + "'"
	case *ast.AssociatedConst:
		return "constant '" + parent.Name.Token.Text + "'"
	case *ast.GlobalVar:
		return "initializer '" + parent.Name().Token.Text + "'"
	default:
		return "expression"
	}
}

func (e *Evaluator) ensureExpr(expr ast.Expr, typ types.Type, errNode ast.Node) bool {
	if core.IsNil(expr) {
		return false
	}

	return e.request(expr, errNode, func() evaluationState {
		fd := e.fileData(expr)

		if !fd.ExprInfos[expr].CompTime || core.IsNil(typ) || typ == types.Invalid {
			return failed
		}
		if !e.ensureDependencies(expr) {
			return failed
		}

		if types.HasParam(typ) {
			return deferred
		}
		if !e.ensureType(typ, expr, make(map[types.Type]bool)) {
			return failed
		}

		value, ok := e.evalExpr(expr, typ)
		if !ok {
			return failed
		}

		fd.Evaluations[expr] = value
		return done
	})
}

func (e *Evaluator) ensureDependencies(node ast.Node) bool {
	fd := e.fileData(node)

	if reflect, ok := node.(*ast.TypeOf); ok {
		return e.ensureReflectedType(fd.NodeTypes[reflect.Type], reflect, make(map[types.Type]bool))
	}

	if identifier, ok := node.(*ast.Identifier); ok {
		switch decl := fd.ExprInfos[identifier].Node.(type) {
		case *ast.Const:
			fd := e.fileData(decl)
			if !e.ensureExpr(decl.Value, fd.NodeTypes[decl.Type], identifier) {
				return false
			}

		case *ast.AssociatedConst:
			if !core.IsNil(decl.Value) {
				fd := e.fileData(decl)
				if !e.ensureExpr(decl.Value, fd.NodeTypes[decl.Type], identifier) {
					return false
				}
			}

		case *ast.EnumCase:
			if !e.ensureEnum(decl.Parent().(*ast.Enum), identifier) {
				return false
			}

		case *ast.Func:
			typ := fd.ExprInfos[identifier].Type.(*types.Func)

			for _, param := range typ.Params {
				if !e.ensureType(param, identifier, make(map[types.Type]bool)) {
					return false
				}
			}

			if !e.ensureType(typ.Returns, identifier, make(map[types.Type]bool)) {
				return false
			}
		}
	}

	// Type dependencies
	if info, ok := fd.ExprInfos[node]; ok {
		if !e.ensureType(info.Type, node, make(map[types.Type]bool)) {
			return false
		}
	}

	if typ, ok := fd.NodeTypes[node]; ok {
		if !e.ensureType(typ, node, make(map[types.Type]bool)) {
			return false
		}
	}

	if _, ok := node.(ast.Type); ok {
		return true
	}

	// Children expressions
	for child := range node.Children() {
		if !e.ensureDependencies(child) {
			return false
		}
	}

	return true
}

func (e *Evaluator) ensureReflectedType(typ types.Type, errNode ast.Node, seen map[types.Type]bool) bool {
	if core.IsNil(typ) || seen[typ] {
		return true
	}

	seen[typ] = true

	if !e.ensureType(typ, errNode, make(map[types.Type]bool)) {
		return false
	}

	var children []types.Type

	switch typ := typ.(type) {
	case *types.Pointer:
		children = append(children, typ.Pointee)

	case *types.Reference:
		children = append(children, typ.Pointee)

	case *types.Func:
		children = append(children, typ.Params...)
		children = append(children, typ.Returns)

	case *types.Array:
		children = append(children, typ.Element)

	case *types.Tuple:
		children = append(children, typ.Types...)

	case *types.Struct:
		for _, field := range typ.Fields {
			children = append(children, field.Type)
		}

	case *types.Union:
		for _, cas := range typ.Cases {
			for _, field := range cas.Fields {
				children = append(children, field.Type)
			}
		}

	case *types.Alias:
		children = append(children, typ.Type)
	}

	for _, child := range children {
		if !e.ensureReflectedType(child, errNode, seen) {
			return false
		}
	}

	return true
}

func (e *Evaluator) ensureType(typ types.Type, errNode ast.Node, seen map[types.Type]bool) bool {
	if core.IsNil(typ) || seen[typ] {
		return true
	}

	if typ == types.Invalid {
		return false
	}

	seen[typ] = true

	switch typ := typ.(type) {
	case *types.Enum:
		decl := e.ctx.TypeEnv.GetEnumNode(typ)
		return decl != nil && e.ensureEnum(decl, errNode)

	case *types.Array:
		return e.ensureType(typ.Element, errNode, seen)

	case *types.Struct:
		for _, field := range typ.Fields {
			if !e.ensureType(field.Type, errNode, seen) {
				return false
			}
		}

	case *types.Union:
		for _, cas := range typ.Cases {
			for _, field := range cas.Fields {
				if !e.ensureType(field.Type, errNode, seen) {
					return false
				}
			}
		}

	case *types.Tuple:
		for _, element := range typ.Types {
			if !e.ensureType(element, errNode, seen) {
				return false
			}
		}

	case *types.Alias:
		return e.ensureType(typ.Type, errNode, seen)
	}

	// pointers / references do not need to ensure pointee type
	return true
}
