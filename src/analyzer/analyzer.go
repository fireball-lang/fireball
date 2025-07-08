package analyzer

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/utils"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

type analyzer struct {
	ctx   Context
	scope Scope

	fun       *ast.Func
	variables VariableTracker[any]

	isLoopBody bool

	diagnostics []utils.Diagnostic
}

func Analyze(file *ast.File, ctx Context, scope Scope) []utils.Diagnostic {
	a := analyzer{
		ctx:   ctx,
		scope: getFileScope(file, ctx, scope, utils.DiagnosticConsumer(nil)),
	}

	a.accept(file)

	return a.diagnostics
}

// Declarations

func (a *analyzer) VisitMod(m *ast.Mod) {
	if m.Parent().(*ast.File).Decls[0] != m {
		a.error(m, "Module declaration needs to be at the top of the file and there can only be one.")
	}
}

func (a *analyzer) VisitImport(i *ast.Import) {
	if slices.ContainsFunc(i.Symbols, isLeafStar) && len(i.Symbols) != 1 {
		errorSlice(a, i.Symbols, "When importing all symbols using '*', it needs to be the only symbol in  the list.")
	}
}

func (a *analyzer) VisitStruct(s *ast.Struct) {
	a.acceptChildren(s)

	names := make(map[string]bool)

	for _, field := range s.Fields {
		if ast.VoidType.Equals(field.Type) {
			a.error(field.Type, "Type 'void' cannot be used for fields.")
		}

		if field.Name != nil {
			name := field.Name.Token.Text

			if _, ok := names[name]; ok {
				a.error(field.Name, "Field with the name '"+name+"' already exists.")
			}

			names[name] = true
		}
	}
}

func (a *analyzer) VisitImpl(i *ast.Impl) {
	a.acceptChildren(i)

	if i.Struct != nil {
		structModulePath := ast.Root(i.Struct).ModulePath()
		implModulePath := ast.Root(i).ModulePath()

		if !ast.PathEquals(structModulePath, implModulePath) {
			a.error(firstNonNil(i.NameN, i), "Can only implement methods for structs in the same module.")
		}
	}
}

func (a *analyzer) VisitFunc(f *ast.Func) {
	a.variables.PushScope()

	if impl, ok := f.Parent().(*ast.Impl); ok && impl.Struct != nil {
		type_ := ast.GetStructPointerType(impl.Struct)
		a.variables.Add("this", type_, nil)
	}

	for _, param := range f.Params {
		if param.Name != nil && ast.IsValid(param.Type) {
			name := param.Name.Token.Text

			if !a.variables.Add(name, param.Type, nil) {
				a.error(param.Name, "Parameter with the name '"+name+"' already exists in this scope.")
			}
		}
	}

	var errorNode ast.Node = f.NameN
	if !ast.IsValid(errorNode) {
		errorNode = f
	}

	if ast.IsValid(f.Body) {
		a.fun = f
		a.acceptChildren(f)
		a.fun = nil

		if !f.ReturnType().Equals(ast.VoidType) {
			expr := ast.GetLastExpr(f.Body)

			if _, ok := expr.(*ast.Return); !ok {
				a.error(errorNode, "Function '"+f.Name()+"' is missing a return statement.")
			}
		}
	} else {
		a.acceptChildren(f)

		attr := f.GetAttribute("extern")

		if attr == nil {
			a.error(errorNode, "Functions without the extern attribute need to have a body.")
		}
	}

	for _, param := range f.Params {
		if ast.VoidType.Equals(param.Type) {
			a.error(param.Type, "Type 'void' cannot be used for parameters.")
		}
	}

	if attr := f.GetAttribute("test"); attr != nil {
		if f.IsMethod() {
			a.error(errorNode, "Methods cannot be marked with the Test attribute, only global functions.")
		} else {
			if len(f.Params) > 0 {
				a.error(errorNode, "Test functions cannot have any parameters.")
			}

			if !f.ReturnType().Equals(ast.BoolType) {
				a.error(errorNode, "Test functions need to return a 'bool'.")
			}
		}
	}

	a.variables.PopScope()
}

// Expressions

func (a *analyzer) VisitBlock(b *ast.Block) {
	a.variables.PushScope()

	a.acceptChildren(b)
	b.Result().Set(ast.Value, ast.VoidType)

	a.variables.PopScope()
}

func (a *analyzer) VisitVar(v *ast.Var) {
	a.acceptChildren(v)

	if a.checkType(v.Value, v.Type, true) {
		if t := v.ActualType(); ast.IsValid(t) {
			if t.Equals(ast.VoidType) {
				var node ast.Node = v.Type

				if !ast.IsValid(node) {
					node = v.Value
				}

				a.error(node, "Type 'void' cannot be used for variables.")
			} else {
				if !a.variables.Add(v.Name.Token.Text, t, nil) {
					a.error(v.Name, "Variable with the name '"+v.Name.Token.Text+"' already exists in this scope.")
				}
			}
		}
	}

	v.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitIf(i *ast.If) {
	a.acceptChildren(i)

	a.checkType(i.Condition, ast.BoolType, true)

	i.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitWhile(w *ast.While) {
	prevIsLoopBody := a.isLoopBody
	a.isLoopBody = true

	a.acceptChildren(w)

	a.isLoopBody = prevIsLoopBody

	a.checkType(w.Condition, ast.BoolType, true)

	w.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitBreak(b *ast.Break) {
	if !a.isLoopBody {
		a.error(b, "Break can only be inside a loop body.")
	}

	b.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitContinue(c *ast.Continue) {
	if !a.isLoopBody {
		a.error(c, "Continue can only be inside a loop body.")
	}

	c.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitReturn(r *ast.Return) {
	a.acceptChildren(r)

	// TODO: allow implicit casts
	a.checkType(r.Value, a.fun.ReturnType(), false)

	r.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitLiteral(l *ast.Literal) {
	str := l.Value.Token.Text

	switch l.Value.Token.Kind {
	case lexer.Identifier:
		if l.Value.Token.Text == "nil" {
			l.Result().Set(ast.Value, &ast.PointerType{Pointee: ast.VoidType})
		} else {
			l.Result().Set(ast.Value, ast.BoolType)
		}

	case lexer.Integer:
		if strings.ContainsAny(str, "uU") {
			a.parseUint(l, 10, "Invalid unsigned integer.")
		} else {
			v, err := strconv.ParseInt(str, 10, 64)

			if err != nil {
				a.error(l, "Invalid signed integer.")
				l.Result().SetInvalid()
			} else {
				type_ := ast.I64Type
				if v >= math.MinInt32 && v <= math.MaxInt32 {
					type_ = ast.I32Type
				}

				l.Result().Set(ast.Value, type_)
			}
		}

	case lexer.Floating:
		if strings.ContainsAny(str, "fF") {
			if _, err := strconv.ParseFloat(str[:len(str)-1], 32); err != nil {
				a.error(l, "Invalid 32-bit floating number.")
			}

			l.Result().Set(ast.Value, ast.F32Type)
		} else {
			if _, err := strconv.ParseFloat(str, 64); err != nil {
				a.error(l, "Invalid 64-bit floating number.")
			}

			l.Result().Set(ast.Value, ast.F64Type)
		}

	case lexer.Hexadecimal:
		a.parseUint(l, 16, "Invalid hexadecimal integer.")

	case lexer.Binary:
		a.parseUint(l, 2, "Invalid binary integer.")

	case lexer.Character:
		l.Result().Set(ast.Value, ast.U8Type)

	case lexer.String:
		l.Result().Set(ast.Value, &ast.PointerType{Pointee: ast.U8Type})

	default:
		panic("analyzer.analyzer.VisitLiteral() - Invalid token kind")
	}
}

func (a *analyzer) parseUint(l *ast.Literal, base int, errorMsg string) {
	str := l.Value.Token.Text

	if base == 10 {
		str = str[:len(str)-1]
	} else {
		str = str[2:]
	}

	v, err := strconv.ParseUint(str, base, 64)

	if err != nil {
		a.error(l, errorMsg)
		l.Result().SetInvalid()
	} else {
		type_ := ast.U64Type
		if v <= math.MaxUint32 {
			type_ = ast.U32Type
		}

		l.Result().Set(ast.Value, type_)
	}
}

func (a *analyzer) VisitStructInitializer(s *ast.StructInitializer) {
	a.acceptChildren(s)

	if s.Struct == nil {
		return
	}

	for _, field := range s.Fields {
		sField, _ := s.Struct.GetField(field.Name.Token.Text)

		if sField == nil {
			a.error(field.Name, "Field with the name '"+field.Name.Token.Text+"' doesn't exist on struct '"+s.Struct.Name()+"'.")
			continue
		}

		a.checkType(field.Value, sField.Type, true)
	}

	s.Result().Set(ast.Value, &ast.DeclType{
		Path: getDeclPath(s.Struct),
		Decl: s.Struct,
	})
}

func (a *analyzer) VisitParen(p *ast.Paren) {
	a.acceptChildren(p)

	if ast.IsValid(p.Expr) {
		*p.Result() = *p.Expr.Result()
	}
}

func (a *analyzer) VisitIdentifier(i *ast.Identifier) {
	var type_ ast.Type
	name := i.Path.SegmentAt(i.Path.SegmentCount() - 1)

	if i.Path.SegmentCount() == 1 {
		type_, _ = a.variables.Find(name)
	}

	if !ast.IsValid(type_) {
		lookup := getSymbolLookup(a.ctx, a.scope, i.Path)

		if !utils.IsNil(lookup) {
			type_ = lookup.GetFuncDecl(name)
		}
	}

	if !ast.IsValid(type_) {
		a.error(i.Path, "Symbol with the path '"+ast.PathString(i.Path)+"' doesn't exist in the current scope.")
		i.Result().SetInvalid()
	} else {
		i.Result().Set(ast.Address, type_)
	}
}

func (a *analyzer) VisitCall(c *ast.Call) {
	a.acceptChildren(c)

	if !ast.IsValid(c.Callee) || c.Callee.Result().Kind == ast.Invalid {
		return
	}

	f, ok := c.Callee.Result().Type.(ast.FuncType)
	if !ok {
		a.error(c.Callee, "Only function types can be called, not '"+c.Callee.Result().Type.String()+"'.")
		c.Result().SetInvalid()
		return
	}

	c.Result().Set(ast.Value, f.ReturnType())

	i := 0
	for expected := range f.ParamTypes() {
		if i >= len(c.Args) {
			a.error(c, "Not enough arguments.")
			return
		}

		// TODO: allow implicit casts
		a.checkType(c.Args[i], expected, false)
		i++
	}

	if i < len(c.Args) && !f.VarArgs() {
		a.error(c, "Too many arguments.")
	}
}

func (a *analyzer) VisitIndex(i *ast.Index) {
	a.acceptChildren(i)
	i.Result().SetInvalid()

	if ast.IsValid(i.Value) && i.Value.Result().Kind != ast.Invalid {
		// TODO: Allow indexing into temporary values
		if i.Value.Result().Kind == ast.Value {
			a.error(i.Value, "Indexing into temporary values is not allowed.")
		}

		switch type_ := i.Value.Result().Type.(type) {
		case *ast.ArrayType:
			i.Result().Set(i.Value.Result().Kind, type_.Element)
		case *ast.PointerType:
			i.Result().Set(i.Value.Result().Kind, type_.Pointee)

		default:
			a.error(i.Value, "Type '"+i.Value.Result().Type.String()+"' cannot be indexed.")
		}
	}

	if ast.IsValid(i.Index) && i.Index.Result().Kind != ast.Invalid {
		if p, ok := i.Index.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsInteger() {
			a.error(i.Index, "Only integer types can index, not '"+i.Index.Result().Type.String()+"'.")
		}
	}
}

func (a *analyzer) VisitMember(m *ast.Member) {
	a.acceptChildren(m)
	m.Result().SetInvalid()

	var decl *ast.Struct

	if ast.IsValid(m.Value) && m.Value.Result().Kind != ast.Invalid {
		t := m.Value.Result().Type
		skipError := false

		if p, ok := t.(*ast.PointerType); ok {
			t = p.Pointee
		}

		if d, ok := t.(*ast.DeclType); ok {
			if ast.IsValid(d.Decl) {
				if s, ok := d.Decl.(*ast.Struct); ok {
					decl = s
				}
			} else {
				skipError = true
			}
		}

		if !ast.IsValid(decl) && !skipError {
			a.error(m.Value, "Only struct types can have members, not '"+m.Value.Result().Type.String()+"'.")
		}
	}

	if decl != nil && m.Name != nil {
		field, _ := decl.GetField(m.Name.Token.Text)

		if field == nil {
			method := a.scope.GetStructMethod(decl, m.Name.Token.Text)

			if method == nil {
				a.error(m.Name, "Struct '"+decl.Name()+"' doesn't have a member with the name '"+m.Name.Token.Text+"'.")
			} else {
				m.Result().Set(ast.Address, method)
			}
		} else if ast.IsValid(field.Type) {
			m.Result().Set(m.Value.Result().Kind, field.Type)
		}
	}
}

func (a *analyzer) VisitUnary(u *ast.Unary) {
	a.acceptChildren(u)

	if !ast.IsValid(u.Expr) || u.Expr.Result().Kind == ast.Invalid {
		u.Result().SetInvalid()
		return
	}

	if u.Postfix {
		switch u.Op {
		// ++, --
		case lexer.PlusPlus, lexer.MinusMinus:
			if p, ok := u.Expr.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsNumeric() {
				a.error(u.Expr, "Needs to be a numeric type, not '"+u.Expr.Result().Type.String()+"'.")

				u.Result().SetInvalid()
				return
			}

			if u.Expr.Result().Kind != ast.Address {
				a.error(u.Expr, "Cannot assign a value into a temporary value.")
			}

			u.Result().Set(ast.Value, u.Expr.Result().Type)

		default:
			panic("analyzer.analyzer.VisitUnary() - Invalid postfix operator")
		}
	} else {
		switch u.Op {
		// ++, --
		case lexer.PlusPlus, lexer.MinusMinus:
			if p, ok := u.Expr.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsNumeric() {
				a.error(u.Expr, "Needs to be a numeric type, not '"+u.Expr.Result().Type.String()+"'.")

				u.Result().SetInvalid()
				return
			}

			if u.Expr.Result().Kind != ast.Address {
				a.error(u.Expr, "Cannot assign a value into a temporary value.")
			}

			u.Result().Set(ast.Value, u.Expr.Result().Type)

		// -
		case lexer.Minus:
			if p, ok := u.Expr.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsNumeric() || p.Kind.IsUnsignedInteger() {
				a.error(u.Expr, "Needs to be a signed numeric type, not '"+u.Expr.Result().Type.String()+"'.")

				u.Result().SetInvalid()
				return
			}

			u.Result().Set(ast.Value, u.Expr.Result().Type)

		// !
		case lexer.Bang:
			a.checkType(u.Expr, ast.BoolType, true)

			u.Result().Set(ast.Value, ast.BoolType)

		// &
		case lexer.Ampersand:
			if u.Expr.Result().Kind != ast.Address {
				a.error(u.Expr, "Cannot take an address of a temporary value.")
			}

			u.Result().Set(ast.Value, &ast.PointerType{Pointee: u.Expr.Result().Type})

		// *
		case lexer.Star:
			if p, ok := u.Expr.Result().Type.(*ast.PointerType); ok {
				u.Result().Set(ast.Address, p.Pointee)
				return
			}

			a.error(u.Expr, "Cannot dereference '"+u.Expr.Result().Type.String()+"'.")
			u.Result().SetInvalid()

		default:
			panic("analyzer.analyzer.VisitUnary() - Invalid prefix operator")
		}
	}
}

func (a *analyzer) VisitBinary(b *ast.Binary) {
	a.acceptChildren(b)

	if !ast.IsValid(b.Left) || !ast.IsValid(b.Right) {
		b.Result().SetInvalid()
		return
	}

	if b.Left.Result().Kind == ast.Invalid || b.Right.Result().Kind == ast.Invalid {
		b.Result().SetInvalid()
		return
	}

	switch b.Op {
	// Assignment
	case lexer.Equal, lexer.PlusEqual, lexer.MinusEqual, lexer.StarEqual, lexer.SlashEqual, lexer.PercentageEqual, lexer.PipeEqual, lexer.XorEqual, lexer.AmpersandEqual:
		if b.Op != lexer.Equal {
			if p, ok := b.Right.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsNumeric() {
				a.error(b.Right, "Needs to be a numeric type, not '"+b.Right.Result().Type.String()+"'.")

				b.Result().SetInvalid()
				return
			}
		}

		if b.Left.Result().Kind != ast.Address {
			a.error(b.Left, "Cannot assign a value into a temporary value.")
		}

		if !b.Left.Result().Type.Equals(b.Right.Result().Type) {
			a.error(b.Right, fmt.Sprintf("Cannot assign a value of type '%s' into type '%s'.", b.Right.Result().Type, b.Left.Result().Type))
		}

		b.Result().Set(ast.Value, b.Left.Result().Type)

	// Boolean
	case lexer.PipePipe, lexer.AmpersandAmpersand:
		a.checkType(b.Left, ast.BoolType, true)
		a.checkType(b.Right, ast.BoolType, true)

		b.Result().Set(ast.Value, ast.BoolType)

	// Equality
	case lexer.EqualEqual, lexer.BangEqual:
		if !b.Left.Result().Type.Equals(b.Right.Result().Type) {
			a.error(b, "Types need to be the same for an equality operator.")
		}

		if _, ok := b.Left.Result().Type.(*ast.PrimitiveType); !ok {
			if _, ok := b.Left.Result().Type.(*ast.PointerType); !ok {
				a.error(b.Left, "Can only check equality for primitive and pointer types, not '"+b.Left.Result().Type.String()+"'.")
			}
		}

		b.Result().Set(ast.Value, ast.BoolType)

	// Comparison
	case lexer.Less, lexer.LessEqual, lexer.Greater, lexer.GreaterEqual:
		if !b.Left.Result().Type.Equals(b.Right.Result().Type) {
			a.error(b, "Types need to be the same for a comparison operator.")
		}

		if p, ok := b.Left.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsNumeric() {
			a.error(b.Left, "Needs to be a numeric type, not '"+b.Left.Result().Type.String()+"'.")

			b.Result().SetInvalid()
			return
		}

		b.Result().Set(ast.Value, ast.BoolType)

	// Logical
	case lexer.Pipe, lexer.Xor, lexer.Ampersand:
		if !b.Left.Result().Type.Equals(b.Right.Result().Type) {
			a.error(b, "Types need to be the same for a logical operator.")

			b.Result().SetInvalid()
			return
		}

		if p, ok := b.Left.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsNumeric() {
			a.error(b.Left, "Needs to be a numeric type, not '"+b.Left.Result().Type.String()+"'.")

			b.Result().SetInvalid()
			return
		}

		b.Result().Set(ast.Value, b.Left.Result().Type)

	// Math
	case lexer.Plus, lexer.Minus, lexer.Star, lexer.Slash, lexer.Percentage:
		if !b.Left.Result().Type.Equals(b.Right.Result().Type) {
			a.error(b, "Types need to be the same for a math operator.")

			b.Result().SetInvalid()
			return
		}

		if p, ok := b.Left.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsNumeric() {
			a.error(b.Left, "Needs to be a numeric type, not '"+b.Left.Result().Type.String()+"'.")

			b.Result().SetInvalid()
			return
		}

		b.Result().Set(ast.Value, b.Left.Result().Type)

	default:
		panic("analyzer.analyzer.VisitBinary() - Invalid operator")
	}
}

func (a *analyzer) VisitCast(c *ast.Cast) {
	a.acceptChildren(c)
	c.Result().SetInvalid()

	if !ast.IsValid(c.Value) || !ast.IsValid(c.Type) || c.Value.Result().Kind == ast.Invalid {
		return
	}

	if _, ok := ast.GetCastKind(c.Value.Result().Type, c.Type, false); !ok {
		a.error(c, "Cannot cast from '"+c.Value.Result().Type.String()+"' to '"+c.Type.String()+"'.")
		return
	}

	c.Result().Set(ast.Value, c.Type)
}

// Utils

func (a *analyzer) acceptChildren(node ast.Node) {
	for child := range node.Children() {
		a.accept(child)
	}
}

func (a *analyzer) accept(node ast.Node) {
	switch node := node.(type) {
	case ast.Decl:
		node.Visit(a)
	case ast.Expr:
		node.Visit(a)

	default:
		a.acceptChildren(node)
	}
}

func firstNonNil(first, second ast.Node) ast.Node {
	if ast.IsValid(first) {
		return first
	}

	return second
}

func (a *analyzer) checkType(expr ast.Expr, expected ast.Type, allowImplicitCasts bool) bool {
	if !ast.IsValid(expr) || !ast.IsValid(expected) || expr.Result().Kind == ast.Invalid {
		return true
	}

	if allowImplicitCasts {
		if _, ok := ast.GetImplicitCastKind(expr.Result().Type, expected); ok {
			return true
		}
	}

	if !expr.Result().Type.Equals(expected) {
		a.error(expr, fmt.Sprintf("Expected type '%s' but got '%s'.", expected, expr.Result().Type))
		return false
	}

	return true
}

func (a *analyzer) error(node ast.Node, message string) {
	a.diagnostics = append(a.diagnostics, utils.Diagnostic{
		Kind:    utils.Error,
		Message: message,
		Range:   node.Range(),
	})
}

func errorSlice[T ast.Node](a *analyzer, nodes []T, message string) {
	a.diagnostics = append(a.diagnostics, utils.Diagnostic{
		Kind:    utils.Error,
		Message: message,
		Range: lexer.Range{
			Start: nodes[0].Range().Start,
			End:   nodes[len(nodes)-1].Range().End,
		},
	})
}
