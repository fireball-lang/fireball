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

type variable struct {
	node     ast.Node
	readonly bool
}

type analyzer struct {
	ctx   Context
	scope Scope

	fun       *ast.Func
	variables VariableTracker[variable]

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

	if len(s.Fields) == 0 {
		a.error(firstNonNil(s.NameN, s), "Struct needs to have at least one field.")
	}

	if structContainsType(s, s) {
		a.error(firstNonNil(s.NameN, s), "Struct cannot be recursive.")
	}

	names := make(map[string]bool)

	for _, field := range s.Fields {
		if ast.VoidType.Equals(field.Type) {
			a.error(field.Type, "Type 'void' cannot be used for fields.")
		}

		if field.Name != nil {
			name := field.Name.Token.Text

			if _, ok := names[name]; ok {
				a.error(firstNonNil(field.Name, field), "Field with the name '"+name+"' already exists.")
			}

			names[name] = true
		}
	}
}

func (a *analyzer) VisitEnum(e *ast.Enum) {
	caseNameMap := make(map[string]any)

	for _, c := range e.Cases {
		if c.Name != nil {
			if _, ok := caseNameMap[c.Name.Token.Text]; ok {
				a.error(c.Name, fmt.Sprintf("Enum case with the name '%s' already exists.", c.Name.Token.Text))
			} else {
				caseNameMap[c.Name.Token.Text] = nil
			}
		}
	}
}

func (a *analyzer) VisitInterface(i *ast.Interface) {
	methodNameMap := make(map[string]any)

	for _, method := range i.Methods {
		if method.Name() != "" {
			if _, ok := methodNameMap[method.Name()]; ok {
				a.error(method.NameN, "Method with the name '"+method.Name()+"' already exists.")
			} else {
				methodNameMap[method.Name()] = nil
			}
		}
	}
}

func (a *analyzer) VisitImpl(i *ast.Impl) {
	a.acceptChildren(i)

	if ast.IsValid(i.Decl) {
		structModulePath := ast.Root(i.Decl).ModulePath()
		implModulePath := ast.Root(i).ModulePath()

		if !ast.PathEquals(structModulePath, implModulePath) {
			a.error(firstNonNil(i.DeclName, i), "Can only implement methods for types in the same module.")
		}
	}

	if ast.IsValid(i.Interface) {
		for _, interfaceMethod := range i.Interface.Methods {
			contains := false

			for _, implMethod := range i.Methods {
				if implMethod.Name() == interfaceMethod.Name() && ast.FuncSignatureEquals(implMethod, interfaceMethod) {
					contains = true
					break
				}
			}

			if !contains {
				a.error(firstNonNil(i.DeclName, i), "Implementation is missing method '"+strings.Replace(interfaceMethod.String(), "fn ", interfaceMethod.Name(), 1)+"' from interface '"+i.Interface.Name()+"'.")
				break
			}
		}
	}
}

func (a *analyzer) VisitGlobalVar(g *ast.GlobalVar) {
	a.acceptChildren(g)

	if ast.IsValid(g.Type) && g.Type.Equals(ast.VoidType) {
		a.error(g.Type, "Type 'void' cannot be used for global variables.")
	}
}

func (a *analyzer) VisitFunc(f *ast.Func) {
	a.variables.PushScope()

	if impl, ok := f.Parent().(*ast.Impl); ok && ast.IsValid(impl.Decl) {
		type_ := ast.GetDeclPointerType(impl.Decl)

		a.variables.Add("this", type_, variable{
			node:     impl,
			readonly: true,
		})
	}

	for _, param := range f.Params {
		if param.Name != nil && ast.IsValid(param.Type) {
			name := param.Name.Token.Text

			if !a.variables.Add(name, param.Type, variable{node: param}) {
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
	b.Result().Set(ast.None, ast.VoidType)

	a.variables.PopScope()
}

func (a *analyzer) VisitVar(v *ast.Var) {
	a.acceptChildren(v)

	if a.checkType(v.Value, v.Type) {
		if t := v.ActualType(); ast.IsValid(t) {
			if t.Equals(ast.VoidType) {
				var node ast.Node = v.Type

				if !ast.IsValid(node) {
					node = v.Value
				}

				a.error(node, "Type 'void' cannot be used for variables.")
			} else {
				if !a.variables.Add(v.Name.Token.Text, t, variable{node: v}) {
					a.error(v.Name, "Variable with the name '"+v.Name.Token.Text+"' already exists in this scope.")
				}
			}
		}
	}

	v.Result().Set(ast.None, ast.VoidType)
}

func (a *analyzer) VisitIf(i *ast.If) {
	a.acceptChildren(i)

	a.checkType(i.Condition, ast.BoolType)

	i.Result().Set(ast.None, ast.VoidType)
}

func (a *analyzer) VisitWhile(w *ast.While) {
	prevIsLoopBody := a.isLoopBody
	a.isLoopBody = true

	a.acceptChildren(w)

	a.isLoopBody = prevIsLoopBody

	a.checkType(w.Condition, ast.BoolType)

	w.Result().Set(ast.None, ast.VoidType)
}

func (a *analyzer) VisitBreak(b *ast.Break) {
	if !a.isLoopBody {
		a.error(b, "Break can only be inside a loop body.")
	}

	b.Result().Set(ast.None, ast.VoidType)
}

func (a *analyzer) VisitContinue(c *ast.Continue) {
	if !a.isLoopBody {
		a.error(c, "Continue can only be inside a loop body.")
	}

	c.Result().Set(ast.None, ast.VoidType)
}

func (a *analyzer) VisitReturn(r *ast.Return) {
	a.acceptChildren(r)

	// TODO: allow implicit casts
	a.checkType(r.Value, a.fun.ReturnType())

	r.Result().Set(ast.None, ast.VoidType)
}

func (a *analyzer) VisitLiteral(l *ast.Literal) {
	str := l.Value.Token.Text

	switch l.Value.Token.Kind {
	case lexer.Identifier:
		if l.Value.Token.Text == "nil" {
			l.Result().Set(ast.None, &ast.PointerType{Pointee: ast.VoidType})
		} else {
			l.Result().Set(ast.None, ast.BoolType)
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

				l.Result().Set(ast.None, type_)
			}
		}

	case lexer.Floating:
		if strings.ContainsAny(str, "fF") {
			if _, err := strconv.ParseFloat(str[:len(str)-1], 32); err != nil {
				a.error(l, "Invalid 32-bit floating number.")
			}

			l.Result().Set(ast.None, ast.F32Type)
		} else {
			if _, err := strconv.ParseFloat(str, 64); err != nil {
				a.error(l, "Invalid 64-bit floating number.")
			}

			l.Result().Set(ast.None, ast.F64Type)
		}

	case lexer.Hexadecimal:
		a.parseUint(l, 16, "Invalid hexadecimal integer.")

	case lexer.Binary:
		a.parseUint(l, 2, "Invalid binary integer.")

	case lexer.Character:
		l.Result().Set(ast.None, ast.U8Type)

	case lexer.String:
		s := stringBuilder{startPos: l.Range().Start}
		ParseString(l.Value.Token.Text, &s)

		a.diagnostics = append(a.diagnostics, s.errors...)

		l.Result().Set(ast.None, &ast.PointerType{Pointee: ast.U8Type})

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

		l.Result().Set(ast.None, type_)
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

		a.checkType(field.Value, sField.Type)
	}

	s.Result().Set(ast.None, &ast.DeclType{
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
	var node ast.Node
	var flags ast.ExprResultFlags

	name := i.Path.SegmentAt(i.Path.SegmentCount() - 1)

	// Variables
	if i.Path.SegmentCount() == 1 {
		// Local
		t, v := a.variables.Find(name)

		type_ = t
		node = v.node
		flags = ast.Addressable

		if !v.readonly {
			flags |= ast.Assignable
		}

		if ast.IsValid(t) && v.readonly {
			if b, ok := i.Parent().(*ast.Binary); ok {
				//goland:noinspection GoSwitchMissingCasesForIotaConsts
				switch b.Op {
				case lexer.Equal, lexer.PlusEqual, lexer.MinusEqual, lexer.StarEqual, lexer.SlashEqual, lexer.PercentageEqual, lexer.PipeEqual, lexer.XorEqual, lexer.AmpersandEqual:
					a.error(b, "Variable '"+name+"' is readonly.")
				}
			}
		}

		// Global
		if !ast.IsValid(type_) {
			g := a.scope.GetGlobalVar(name)

			if g != nil {
				type_ = g.Type
				node = g
				flags = ast.Addressable | ast.Assignable
			}
		}
	}

	// Function
	if !ast.IsValid(type_) {
		lookup := getSymbolLookup(a.ctx, a.scope, i.Path)

		if !utils.IsNil(lookup) {
			f := lookup.GetFuncDecl(name)

			type_ = f
			node = f
		}
	}

	// Type
	if !ast.IsValid(type_) {
		if _, ok := i.Parent().(*ast.Member); ok {
			lookup := getSymbolLookup(a.ctx, a.scope, i.Path)

			if !utils.IsNil(lookup) {
				decl := lookup.GetTypeDecl(name)

				if ast.IsValid(decl) {
					type_ = &ast.DeclType{Path: getDeclPath(decl), Decl: decl}
					node = decl
				}
			}
		}
	}

	// Result
	if !ast.IsValid(type_) {
		a.error(i.Path, "Symbol with the path '"+ast.PathString(i.Path)+"' doesn't exist in the current scope.")

		i.Result().SetInvalid()
		i.Resolved = nil
	} else {
		i.Result().Set(flags, type_)
		i.Resolved = node
	}
}

func (a *analyzer) VisitCall(c *ast.Call) {
	a.acceptChildren(c)

	if !ast.IsValid(c.Callee) || c.Callee.Result().Flags.IsInvalid() {
		return
	}

	f, ok := c.Callee.Result().Type.(ast.FuncType)
	if !ok {
		a.error(c.Callee, "Only function types can be called, not '"+c.Callee.Result().Type.String()+"'.")
		c.Result().SetInvalid()
		return
	}

	c.Result().Set(ast.None, f.ReturnType())

	i := 0
	for expected := range f.ParamTypes() {
		if i >= len(c.Args) {
			a.error(c, "Not enough arguments.")
			return
		}

		// TODO: allow implicit casts
		a.checkType(c.Args[i], expected)
		i++
	}

	if i < len(c.Args) && !f.VarArgs() {
		a.error(c, "Too many arguments.")
	}
}

func (a *analyzer) VisitTypeCall(t *ast.TypeCall) {
	a.acceptChildren(t)
	t.Result().Set(ast.None, ast.U32Type)
}

func (a *analyzer) VisitIndex(i *ast.Index) {
	a.acceptChildren(i)
	i.Result().SetInvalid()

	if ast.IsValid(i.Value) && !i.Value.Result().Flags.IsInvalid() {
		// TODO: Allow indexing into temporary values
		if !i.Value.Result().Flags.IsAssignable() {
			a.error(i.Value, "Indexing into temporary values is not allowed.")
		}

		switch type_ := i.Value.Result().Type.(type) {
		case *ast.ArrayType:
			i.Result().Set(i.Value.Result().Flags, type_.Element)
		case *ast.PointerType:
			i.Result().Set(i.Value.Result().Flags, type_.Pointee)

		default:
			a.error(i.Value, "Type '"+i.Value.Result().Type.String()+"' cannot be indexed.")
		}
	}

	if ast.IsValid(i.Index) && !i.Index.Result().Flags.IsInvalid() {
		if p, ok := i.Index.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsInteger() {
			a.error(i.Index, "Only integer types can index, not '"+i.Index.Result().Type.String()+"'.")
		}
	}
}

func (a *analyzer) VisitMember(m *ast.Member) {
	a.acceptChildren(m)

	m.Result().SetInvalid()
	m.Resolved = nil

	// Type
	if ast.IsValid(m.Value) && !m.Value.Result().Flags.IsInvalid() && m.Name != nil {
		if ident, ok := m.Value.(*ast.Identifier); ok {
			// Enum case
			if decl, ok := ident.Resolved.(*ast.Enum); ok {
				var enumCase *ast.EnumCase

				for _, c := range decl.Cases {
					if c.Name != nil && c.Name.Token.Text == m.Name.Token.Text {
						enumCase = c
						break
					}
				}

				if enumCase != nil {
					m.Result().Set(ast.None, m.Value.Result().Type)
					m.Resolved = enumCase

					return
				}
			}

			// Static method
			if decl, ok := ident.Resolved.(ast.Decl); ok {
				method := a.scope.GetDeclMethod(decl, m.Name.Token.Text, true)

				if method != nil {
					m.Result().Set(ast.None, method)
					m.Resolved = method

					return
				}
			}
		}
	}

	// Member
	var decl ast.Decl

	if ast.IsValid(m.Value) && !m.Value.Result().Flags.IsInvalid() {
		t := m.Value.Result().Type
		skipError := false

		if p, ok := t.(*ast.PointerType); ok {
			t = p.Pointee
		}

		if d, ok := t.(*ast.DeclType); ok {
			if ast.IsValid(d.Decl) {
				decl = d.Decl
			} else {
				skipError = true
			}
		}

		if !ast.IsValid(decl) && !skipError {
			a.error(m.Value, "Type '"+m.Value.Result().Type.String()+"' cannot have members.")
		}
	}

	if decl != nil && m.Name != nil {
		// Field
		var field *ast.Field

		if s, ok := decl.(*ast.Struct); ok {
			field, _ = s.GetField(m.Name.Token.Text)
		}

		if field == nil {
			// Method
			method := a.scope.GetDeclMethod(decl, m.Name.Token.Text, false)

			if method == nil {
				a.error(m.Name, "Type '"+decl.Name()+"' doesn't have a member with the name '"+m.Name.Token.Text+"'.")
			} else {
				m.Result().Set(ast.None, method)
				m.Resolved = method
			}
		} else if ast.IsValid(field.Type) {
			flags := ast.None

			if m.Value.Result().Flags.IsAddressable() {
				flags = ast.Addressable
			}
			if _, ok := m.Value.Result().Type.(*ast.PointerType); ok || flags.IsAddressable() {
				flags |= ast.Assignable
			}

			m.Result().Set(flags, field.Type)
			m.Resolved = field
		}
	}
}

func (a *analyzer) VisitUnary(u *ast.Unary) {
	a.acceptChildren(u)

	if !ast.IsValid(u.Expr) || u.Expr.Result().Flags.IsInvalid() {
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

			if !u.Expr.Result().Flags.IsAssignable() {
				a.error(u.Expr, "Cannot assign a value into a temporary value.")
			}

			u.Result().Set(ast.None, u.Expr.Result().Type)

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

			if !u.Expr.Result().Flags.IsAssignable() {
				a.error(u.Expr, "Cannot assign a value into a temporary value.")
			}

			u.Result().Set(ast.None, u.Expr.Result().Type)

		// -
		case lexer.Minus:
			if p, ok := u.Expr.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsNumeric() || p.Kind.IsUnsignedInteger() {
				a.error(u.Expr, "Needs to be a signed numeric type, not '"+u.Expr.Result().Type.String()+"'.")

				u.Result().SetInvalid()
				return
			}

			u.Result().Set(ast.None, u.Expr.Result().Type)

		// !
		case lexer.Bang:
			a.checkType(u.Expr, ast.BoolType)

			u.Result().Set(ast.None, ast.BoolType)

		// &
		case lexer.Ampersand:
			if !u.Expr.Result().Flags.IsAssignable() {
				a.error(u.Expr, "Cannot take an address of a temporary value.")
			}

			u.Result().Set(ast.None, &ast.PointerType{Pointee: u.Expr.Result().Type})

		// *
		case lexer.Star:
			if p, ok := u.Expr.Result().Type.(*ast.PointerType); ok {
				u.Result().Set(ast.Addressable|ast.Assignable, p.Pointee)
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

	if b.Left.Result().Flags.IsInvalid() || b.Right.Result().Flags.IsInvalid() {
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

		if !b.Left.Result().Flags.IsAssignable() {
			a.error(b.Left, "Cannot assign a value into a temporary value.")
		}

		if !b.Left.Result().Type.Equals(b.Right.Result().Type) {
			a.error(b.Right, fmt.Sprintf("Cannot assign a value of type '%s' into type '%s'.", b.Right.Result().Type, b.Left.Result().Type))
		}

		b.Result().Set(ast.None, b.Left.Result().Type)

	// Boolean
	case lexer.PipePipe, lexer.AmpersandAmpersand:
		a.checkType(b.Left, ast.BoolType)
		a.checkType(b.Right, ast.BoolType)

		b.Result().Set(ast.None, ast.BoolType)

	// Equality
	case lexer.EqualEqual, lexer.BangEqual:
		if !b.Left.Result().Type.Equals(b.Right.Result().Type) {
			a.error(b, "Types need to be the same for an equality operator.")
		}

		if _, ok := b.Left.Result().Type.(*ast.PrimitiveType); !ok {
			if _, ok := b.Left.Result().Type.(*ast.PointerType); !ok {
				if _, ok := ast.GetDeclFromDeclType[*ast.Enum](b.Left.Result().Type); !ok {
					a.error(b.Left, "Can only check equality for primitive and pointer types, not '"+b.Left.Result().Type.String()+"'.")
				}
			}
		}

		b.Result().Set(ast.None, ast.BoolType)

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

		b.Result().Set(ast.None, ast.BoolType)

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

		b.Result().Set(ast.None, b.Left.Result().Type)

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

		b.Result().Set(ast.None, b.Left.Result().Type)

	default:
		panic("analyzer.analyzer.VisitBinary() - Invalid operator")
	}
}

func (a *analyzer) VisitIs(i *ast.Is) {
	a.acceptChildren(i)
	i.Result().Set(ast.None, ast.BoolType)

	if ast.IsValid(i.Value) && !i.Value.Result().Flags.IsInvalid() {
		if _, ok := ast.GetDeclFromDeclType[*ast.Interface](i.Value.Result().Type); !ok {
			a.error(i.Value, "Value needs be an interface, not '"+i.Value.Result().Type.String()+"'.")
		}
	}

	if ast.IsValid(i.Type) {
		var pointee ast.Type

		if p, ok := i.Type.(*ast.PointerType); ok {
			pointee = p.Pointee
		} else {
			a.error(i.Type, "Type needs to be a pointer to a declaration, not '"+i.Type.String()+"'.")
		}

		if ast.IsValid(pointee) {
			if _, ok := pointee.(*ast.DeclType); !ok {
				a.error(i.Type, "Type needs to be a pointer to a declaration, not '"+i.Type.String()+"'.")
			}
		}
	}
}

func (a *analyzer) VisitCast(c *ast.Cast) {
	a.acceptChildren(c)
	c.Result().SetInvalid()

	if !ast.IsValid(c.Value) || !ast.IsValid(c.Type) || c.Value.Result().Flags.IsInvalid() {
		return
	}

	if _, ok := GetCastKind(a.ctx, c.Value.Result().Type, c.Type, false); !ok {
		a.error(c, "Cannot cast from '"+c.Value.Result().Type.String()+"' to '"+c.Type.String()+"'.")
		return
	}

	c.Result().Set(ast.None, c.Type)
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

func (a *analyzer) checkType(expr ast.Expr, expected ast.Type) bool {
	if !ast.IsValid(expr) || !ast.IsValid(expected) || expr.Result().Flags.IsInvalid() {
		return true
	}

	if _, ok := GetImplicitCastKind(a.ctx, expr.Result().Type, expected); ok {
		return true
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
