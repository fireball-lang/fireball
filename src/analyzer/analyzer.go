package analyzer

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/utils"
	"fmt"
	"strings"
)

type analyzer struct {
	fun       *ast.Func
	scope     Scope
	variables VariableTracker[any]

	diagnostics []utils.Diagnostic
}

func Analyze(f *ast.File, scope Scope) []utils.Diagnostic {
	a := analyzer{scope: scope}
	a.accept(f)

	return a.diagnostics
}

// Declarations

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

func (a *analyzer) VisitFunc(f *ast.Func) {
	a.variables.PushScope()

	for _, param := range f.Params {
		if param.Name != nil && ast.IsValid(param.Type) {
			name := param.Name.Token.Text

			if !a.variables.Add(name, param.Type, nil) {
				a.error(param.Name, "Parameter with the name '"+name+"' already exists in this scope.")
			}
		}
	}

	if ast.IsValid(f.Body) {
		a.fun = f
		a.acceptChildren(f)
		a.fun = nil

		if !f.ReturnType().Equals(ast.VoidType) {
			expr := ast.GetLastExpr(f.Body)

			if _, ok := expr.(*ast.Return); !ok {
				var node ast.Node = f.NameN
				if !ast.IsValid(node) {
					node = f
				}

				a.error(node, "Function '"+f.Name()+"' is missing a return statement.")
			}
		}
	}

	for _, param := range f.Params {
		if ast.VoidType.Equals(param.Type) {
			a.error(param.Type, "Type 'void' cannot be used for parameters.")
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

	if a.checkType(v.Value, v.Type) {
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

	a.checkType(i.Condition, ast.BoolType)

	i.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitWhile(w *ast.While) {
	a.acceptChildren(w)

	a.checkType(w.Condition, ast.BoolType)

	w.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitReturn(r *ast.Return) {
	a.acceptChildren(r)

	a.checkType(r.Value, a.fun.ReturnType())

	r.Result().Set(ast.Value, ast.VoidType)
}

func (a *analyzer) VisitLiteral(l *ast.Literal) {
	switch l.Value.Token.Kind {
	case lexer.Identifier:
		l.Result().Set(ast.Value, ast.BoolType)

	case lexer.Number:
		type_ := ast.I32Type

		if strings.ContainsRune(l.Value.Token.Text, '.') {
			type_ = ast.F64Type
		}

		l.Result().Set(ast.Value, type_)

	case lexer.String:
		l.Result().Set(ast.Value, &ast.PointerType{Pointee: ast.U8Type})

	default:
		panic("analyzer.analyzer.VisitLiteral() - Invalid token kind")
	}
}

func (a *analyzer) VisitParen(p *ast.Paren) {
	a.acceptChildren(p)

	if ast.IsValid(p.Expr) {
		*p.Result() = *p.Expr.Result()
	}
}

func (a *analyzer) VisitIdentifier(i *ast.Identifier) {
	name := i.Name.Token.Text
	type_, _ := a.variables.Find(name)

	if !ast.IsValid(type_) {
		file := ast.Root(i)

		for _, decl := range file.Decls {
			if f, ok := decl.(*ast.Func); ok && f.Name() == name {
				type_ = f
				break
			}
		}
	}

	if !ast.IsValid(type_) {
		a.error(i.Name, "Symbol with the name '"+name+"' doesn't exist.")
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

		a.checkType(c.Args[i], expected)
		i++
	}

	if i < len(c.Args) && !f.VarArgs() {
		a.error(c, "Too many arguments.")
	}
}

func (a *analyzer) VisitIndex(i *ast.Index) {
	a.acceptChildren(i)

	err := i.Value.Result().Kind == ast.Invalid

	if ast.IsValid(i.Value) && i.Value.Result().Kind != ast.Invalid {
		if i.Value.Result().Kind != ast.Address {
			a.error(i.Value, "Cannot index into a temporary value.")
			err = true
		}

		if _, ok := i.Value.Result().Type.(*ast.PointerType); !ok {
			a.error(i.Value, "Type '"+i.Value.Result().Type.String()+"' cannot be indexed.")
			err = true
		}
	}

	if ast.IsValid(i.Index) && i.Index.Result().Kind != ast.Invalid {
		if p, ok := i.Index.Result().Type.(*ast.PrimitiveType); !ok || !p.Kind.IsInteger() {
			a.error(i.Index, "Only integer types can index, not '"+i.Index.Result().Type.String()+"'.")
			err = true
		}
	}

	if err {
		i.Result().SetInvalid()
	} else {
		i.Result().Set(i.Value.Result().Kind, i.Value.Result().Type.(*ast.PointerType).Pointee)
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
			a.error(m.Name, "Struct '"+decl.Name()+"' doesn't have a member with the name '"+m.Name.Token.Text+"'.")
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
			a.checkType(u.Expr, ast.BoolType)

			u.Result().Set(ast.Value, ast.BoolType)

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
		a.checkType(b.Left, ast.BoolType)
		a.checkType(b.Right, ast.BoolType)

		b.Result().Set(ast.Value, ast.BoolType)

	// Equality
	case lexer.EqualEqual, lexer.BangEqual:
		if !b.Left.Result().Type.Equals(b.Right.Result().Type) {
			a.error(b, "Types need to be the same for an equality operator.")
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

	case *ast.DeclType:
		decl := a.scope.GetTypeDecl(node.Name.Token.Text)

		if ast.IsValid(decl) {
			node.Decl = decl
		} else {
			a.error(node.Name, "Type with the name '"+node.Name.Token.Text+"' doesn't exist.")
		}

	default:
		a.acceptChildren(node)
	}
}

func (a *analyzer) checkType(expr ast.Expr, expected ast.Type) bool {
	if ast.IsValid(expr) && ast.IsValid(expected) && expr.Result().Kind != ast.Invalid && !expr.Result().Type.Equals(expected) {
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
