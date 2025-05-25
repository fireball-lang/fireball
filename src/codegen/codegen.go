package codegen

import (
	"fireball/analyzer"
	"fireball/ast"
	"fireball/lexer"
	"fireball/llvm"
	"strconv"
	"strings"
)

type codegen struct {
	module              *llvm.Module
	stringConstantCount int

	functions                   map[*ast.Func]*llvm.ExternFunction
	additionalExternalFunctions map[*ast.Func]any
	types                       []typeMapping

	fun         *llvm.Function
	identifiers map[string]int

	variables       analyzer.VariableTracker[llvm.IdentifierValue]
	variableAllocas map[*ast.Var]llvm.IdentifierValue

	exprValue llvm.Value
}

func Gen(file *ast.File, path string) *llvm.Module {
	c := codegen{
		module:                      llvm.NewModule(path, "", ""),
		functions:                   make(map[*ast.Func]*llvm.ExternFunction),
		additionalExternalFunctions: make(map[*ast.Func]any),
	}

	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.Func); ok {
			c.collectFunc(f)
		}
	}

	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.Func); ok {
			c.VisitFunc(f)
		}
	}

	for f := range c.additionalExternalFunctions {
		c.module.NewExternFunction(f.Name(), c.getType(f))
	}

	return c.module
}

// Declarations

func (c *codegen) collectFunc(f *ast.Func) {
	type_ := ast.PointerType{Pointee: f}
	value := llvm.FakeFunctionValue(c.getType(&type_), f.Name())

	c.functions[f] = &value
}

func (c *codegen) VisitFunc(f *ast.Func) {
	// Header
	type_ := c.getType(f)

	if ast.IsValid(f.Body) {
		paramNames := make([]string, len(f.Params))

		for i, param := range f.Params {
			paramNames[i] = param.Name.Token.Text
		}

		c.fun = c.module.NewFunction(f.Name(), type_, paramNames)
	} else {
		c.module.NewExternFunction(f.Name(), type_)
		return
	}

	// Body
	c.variables.PushScope()

	c.fun.Block(llvm.NamedIdentifier("func.entry"))

	c.identifiers = make(map[string]int)
	c.variableAllocas = make(map[*ast.Var]llvm.IdentifierValue)
	c.collectVariables(f.Body)

	for i, param := range f.Params {
		c.setSourceLocation(param)
		type_ := c.getType(param.Type)

		ptr := llvm.Alloca(c.fun, type_, 1, 1, "param."+param.Name.Token.Text)
		c.fun.LocalVariable(ptr, param.Name.Token.Text, uint32(i+1))

		llvm.Store(c.fun, llvm.NamedIdentifierValue(type_, param.Name.Token.Text), ptr)

		c.variables.Add(param.Name.Token.Text, param.Type, ptr)
	}

	c.visit(f.Body)

	if f.ReturnType().Equals(ast.VoidType) {
		expr := ast.GetLastExpr(f.Body)

		if _, ok := expr.(*ast.Return); !ok {
			llvm.Ret(c.fun)
		}
	}

	c.fun.End()
	c.variables.PopScope()
	c.fun = nil
}

func (c *codegen) collectVariables(expr ast.Expr) {
	if v, ok := expr.(*ast.Var); ok {
		type_ := v.Type

		if !ast.IsValid(type_) {
			type_ = v.Value.Result().Type
		}

		c.setSourceLocation(v)
		name := c.getNamedIdentifierString("var." + v.Name.Token.Text)

		c.variableAllocas[v] = llvm.Alloca(c.fun, c.getType(type_), 1, 1, name)
	}

	for node := range expr.Children() {
		if expr, ok := node.(ast.Expr); ok {
			c.collectVariables(expr)
		}
	}
}

// Expressions

func (c *codegen) VisitBlock(b *ast.Block) {
	c.fun.PushScope()
	c.variables.PushScope()

	for _, expr := range b.Exprs {
		c.visit(expr)
	}

	c.variables.PopScope()
	c.fun.PopScope()
}

func (c *codegen) VisitVar(v *ast.Var) {
	var type_ ast.Type

	ptr := c.variableAllocas[v]
	c.fun.LocalVariable(ptr, v.Name.Token.Text, 0)

	if ast.IsValid(v.Value) {
		type_ = v.Value.Result().Type

		value := c.visitLoad(v.Value)
		llvm.Store(c.fun, value, ptr)
	} else {
		type_ = v.Type

		value := llvm.ZeroInitialize(c.getType(type_))
		llvm.Store(c.fun, value, ptr)
	}

	c.variables.Add(v.Name.Token.Text, type_, ptr)
}

func (c *codegen) VisitIf(i *ast.If) {
	trueL := c.getNamedIdentifier("if.true")
	falseL := c.getNamedIdentifier("if.end")
	endL := falseL

	if ast.IsValid(i.Else) {
		falseL = c.getNamedIdentifier("if.false")
	}

	// Condition
	condition := c.visitLoad(i.Condition)
	llvm.BrCond(c.fun, condition, trueL, falseL)

	// Then
	c.fun.Block(trueL)
	c.visit(i.Then)
	llvm.Br(c.fun, endL)

	// Else
	if ast.IsValid(i.Else) {
		c.fun.Block(falseL)
		c.visit(i.Else)
		llvm.Br(c.fun, endL)
	}

	// End
	c.fun.Block(endL)
}

func (c *codegen) VisitWhile(w *ast.While) {
	conditionL := c.getNamedIdentifier("while.condition")
	bodyL := c.getNamedIdentifier("while.body")
	endL := c.getNamedIdentifier("while.end")

	llvm.Br(c.fun, conditionL)

	// Condition
	c.fun.Block(conditionL)
	condition := c.visitLoad(w.Condition)
	llvm.BrCond(c.fun, condition, bodyL, endL)

	// Body
	c.fun.Block(bodyL)
	c.visit(w.Body)
	llvm.Br(c.fun, conditionL)

	// End
	c.fun.Block(endL)
}

func (c *codegen) VisitReturn(r *ast.Return) {
	if ast.IsValid(r.Value) {
		value := c.visitLoad(r.Value)
		llvm.RetValue(c.fun, value)
	} else {
		llvm.Ret(c.fun)
	}
}

func (c *codegen) VisitLiteral(l *ast.Literal) {
	switch l.Value.Token.Kind {
	case lexer.Identifier:
		if l.Value.Token.Text == "true" {
			c.exprValue = llvm.True()
		} else {
			c.exprValue = llvm.False()
		}

	case lexer.Number:
		if strings.ContainsRune(l.Value.Token.Text, '.') {
			num, _ := strconv.ParseFloat(l.Value.Token.Text, 64)
			c.exprValue = llvm.Double(num)
		} else {
			num, _ := strconv.ParseInt(l.Value.Token.Text, 10, 32)
			c.exprValue = llvm.Int(llvm.I32, num)
		}

	case lexer.String:
		c.stringConstantCount++

		text := l.Value.Token.Text[1 : len(l.Value.Token.Text)-1]
		text = strings.ReplaceAll(text, "\\n", "\n")

		c.exprValue = c.module.NewStringConstant(
			"string."+strconv.FormatInt(int64(c.stringConstantCount), 10),
			text,
		)

	default:
		panic("codegen.codegen.VisitLiteral() - Invalid token kind")
	}
}

func (c *codegen) VisitParen(p *ast.Paren) {
	c.exprValue = c.visit(p.Expr)
}

func (c *codegen) VisitIdentifier(i *ast.Identifier) {
	name := i.Name.Token.Text

	var value llvm.Value
	type_, value := c.variables.Find(name)

	if !ast.IsValid(type_) {
		f := i.Result().Type.(*ast.Func)
		type_ = f

		if v, ok := c.functions[f]; ok {
			value = v
		} else {
			v := llvm.FakeFunctionValue(c.getType(f), f.Name())
			value = &v

			c.additionalExternalFunctions[f] = nil
		}
	}

	if !ast.IsValid(type_) {
		panic("codegen.codegen.VisitIdentifier() - Failed to find symbol")
	} else {
		c.exprValue = value
	}
}

func (c *codegen) VisitCall(call *ast.Call) {
	callee := c.visitLoad(call.Callee)
	args := make([]llvm.Value, len(call.Args))

	for i, arg := range call.Args {
		args[i] = c.visitLoad(arg)
	}

	callBuilder := llvm.Call(c.fun, callee, "")

	for _, arg := range args {
		llvm.Arg(&callBuilder, arg)
	}

	callBuilder.End()
}

func (c *codegen) VisitIndex(i *ast.Index) {
	ptr := c.visitLoad(i.Value)
	index := c.visitLoad(i.Index)

	c.exprValue = llvm.GetElementPtr1(c.fun, ptr, index, "")
}

func (c *codegen) VisitMember(m *ast.Member) {
	var decl *ast.Struct
	isPtr := false

	if p, ok := m.Value.Result().Type.(*ast.PointerType); ok {
		decl = p.Pointee.(*ast.DeclType).Decl.(*ast.Struct)
		isPtr = true
	} else {
		decl = m.Value.Result().Type.(*ast.DeclType).Decl.(*ast.Struct)
	}

	_, i := decl.GetField(m.Name.Token.Text)

	if m.Value.Result().Kind == ast.Value {
		value := c.visitLoad(m.Value)
		c.exprValue = llvm.ExtractValue(c.fun, value, uint32(i), "")
	} else {
		ptr := c.visit(m.Value)

		if isPtr {
			ptr = llvm.Load(c.fun, ptr, "")
		}

		c.exprValue = llvm.GetElementPtr2(c.fun, ptr, 0, uint32(i), "")
	}
}

func (c *codegen) VisitUnary(u *ast.Unary) {
	if u.Postfix {
		switch u.Op {
		// ++, --
		case lexer.PlusPlus, lexer.MinusMinus:
			ptr := c.visit(u.Expr)

			oldValue := llvm.Load(c.fun, ptr, "")
			newValue := llvm.Add(c.fun, oldValue, c.getConstantOne(u.Expr.Result().Type), "")

			llvm.Store(c.fun, newValue, ptr)
			c.exprValue = oldValue

		default:
			panic("codegen.codegen.VisitUnary() - Invalid postfix operator")
		}
	} else {
		switch u.Op {
		// ++, --
		case lexer.PlusPlus, lexer.MinusMinus:
			ptr := c.visit(u.Expr)

			oldValue := llvm.Load(c.fun, ptr, "")
			newValue := llvm.Add(c.fun, oldValue, c.getConstantOne(u.Expr.Result().Type), "")

			llvm.Store(c.fun, newValue, ptr)
			c.exprValue = newValue

		// -
		case lexer.Minus:
			value := c.visitLoad(u.Expr)

			if u.Expr.Result().Type.(*ast.PrimitiveType).Kind.IsFloating() {
				c.exprValue = llvm.NegF(c.fun, value, "")
			} else {
				c.exprValue = llvm.Sub(c.fun, llvm.Int(c.getType(u.Expr.Result().Type), 0), value, "")
			}

		// !
		case lexer.Bang:
			value := c.visitLoad(u.Expr)
			c.exprValue = llvm.Xor(c.fun, value, llvm.True(), "")

		default:
			panic("codegen.codegen.VisitUnary() - Invalid prefix operator")
		}
	}
}

func (c *codegen) VisitBinary(b *ast.Binary) {
	switch b.Op {
	// Assignment
	case lexer.Equal:
		ptr := c.visit(b.Left)

		value := c.visitLoad(b.Right)
		llvm.Store(c.fun, value, ptr)

	case lexer.PlusEqual, lexer.MinusEqual, lexer.StarEqual, lexer.SlashEqual, lexer.PercentageEqual, lexer.PipeEqual, lexer.XorEqual, lexer.AmpersandEqual:
		ptr := c.visit(b.Left)

		left := llvm.Load(c.fun, ptr, "")
		right := c.visitLoad(b.Right)

		value := c.binarySimple(b.Op, left, right)
		llvm.Store(c.fun, value, ptr)

	// Boolean
	case lexer.PipePipe:
		leftL := c.getNamedIdentifier("or.left")
		rightL := c.getNamedIdentifier("or.right")
		exitL := c.getNamedIdentifier("or.exit")

		llvm.Br(c.fun, leftL)

		// Left
		c.fun.Block(leftL)
		left := c.visitLoad(b.Left)
		llvm.BrCond(c.fun, left, exitL, rightL)

		// Right
		c.fun.Block(rightL)
		right := c.visitLoad(b.Right)
		llvm.Br(c.fun, exitL)

		// Exit
		c.fun.Block(exitL)
		c.exprValue = llvm.Phi(c.fun, left, leftL, right, rightL, "")

	case lexer.AmpersandAmpersand:
		leftL := c.getNamedIdentifier("and.left")
		rightL := c.getNamedIdentifier("and.right")
		exitL := c.getNamedIdentifier("and.exit")

		llvm.Br(c.fun, leftL)

		// Left
		c.fun.Block(leftL)
		left := c.visitLoad(b.Left)
		llvm.BrCond(c.fun, left, rightL, exitL)

		// Right
		c.fun.Block(rightL)
		right := c.visitLoad(b.Right)
		llvm.Br(c.fun, exitL)

		// Exit
		c.fun.Block(exitL)
		c.exprValue = llvm.Phi(c.fun, llvm.False(), leftL, right, rightL, "")

	// Equality
	case lexer.EqualEqual, lexer.BangEqual:
		kind := b.Left.Result().Type.(*ast.PrimitiveType).Kind

		left := c.visitLoad(b.Left)
		right := c.visitLoad(b.Right)

		if kind.IsFloating() {
			op := llvm.FOEQ
			if b.Op == lexer.BangEqual {
				op = llvm.FONQ
			}

			c.exprValue = llvm.CmpF(c.fun, op, left, right, "")
		} else {
			op := llvm.IEQ
			if b.Op == lexer.BangEqual {
				op = llvm.INQ
			}

			c.exprValue = llvm.CmpI(c.fun, op, left, right, "")
		}

	// Comparison
	case lexer.Less, lexer.LessEqual, lexer.Greater, lexer.GreaterEqual:
		kind := b.Left.Result().Type.(*ast.PrimitiveType).Kind

		left := c.visitLoad(b.Left)
		right := c.visitLoad(b.Right)

		if kind.IsFloating() {
			var op llvm.CmpFOp

			//goland:noinspection GoSwitchMissingCasesForIotaConsts
			switch b.Op {
			case lexer.Less:
				op = llvm.FOLT
			case lexer.LessEqual:
				op = llvm.FOLE
			case lexer.Greater:
				op = llvm.FOGT
			case lexer.GreaterEqual:
				op = llvm.FOGE
			}

			c.exprValue = llvm.CmpF(c.fun, op, left, right, "")
		} else {
			var op llvm.CmpIOp

			if kind.IsUnsignedInteger() {
				//goland:noinspection GoSwitchMissingCasesForIotaConsts
				switch b.Op {
				case lexer.Less:
					op = llvm.IULT
				case lexer.LessEqual:
					op = llvm.IULE
				case lexer.Greater:
					op = llvm.IUGT
				case lexer.GreaterEqual:
					op = llvm.IUGE
				}
			} else {
				//goland:noinspection GoSwitchMissingCasesForIotaConsts
				switch b.Op {
				case lexer.Less:
					op = llvm.ISLT
				case lexer.LessEqual:
					op = llvm.ISLE
				case lexer.Greater:
					op = llvm.ISGT
				case lexer.GreaterEqual:
					op = llvm.ISGE
				}
			}

			c.exprValue = llvm.CmpI(c.fun, op, left, right, "")
		}

	// Logical, Math
	default:
		c.exprValue = c.binarySimple(b.Op, c.visitLoad(b.Left), c.visitLoad(b.Right))
	}
}

func (c *codegen) binarySimple(op lexer.TokenKind, left, right llvm.Value) llvm.Value {
	switch op {
	// Logical
	case lexer.Pipe, lexer.PipeEqual:
		return llvm.Or(c.fun, left, right, "")
	case lexer.Xor, lexer.XorEqual:
		return llvm.Xor(c.fun, left, right, "")
	case lexer.Ampersand, lexer.AmpersandEqual:
		return llvm.And(c.fun, left, right, "")

	// Math
	case lexer.Plus, lexer.PlusEqual:
		return llvm.Add(c.fun, left, right, "")
	case lexer.Minus, lexer.MinusEqual:
		return llvm.Sub(c.fun, left, right, "")
	case lexer.Star, lexer.StarEqual:
		return llvm.Mul(c.fun, left, right, "")
	case lexer.Slash, lexer.SlashEqual:
		return llvm.Div(c.fun, left, right, "")
	case lexer.Percentage, lexer.PercentageEqual:
		return llvm.Rem(c.fun, left, right, "")

	default:
		panic("codegen.codegen.binarySimple() - Invalid operator")
	}
}

// Utils

func (c *codegen) getConstantOne(type_ ast.Type) llvm.Value {
	kind := type_.(*ast.PrimitiveType).Kind

	if kind == ast.F32 {
		return llvm.Float(1)
	}
	if kind == ast.F64 {
		return llvm.Double(1)
	}

	return llvm.Int(c.getType(type_), 1)
}

func (c *codegen) getNamedIdentifier(name string) llvm.Identifier {
	return llvm.NamedIdentifier(c.getNamedIdentifierString(name))
}

func (c *codegen) getNamedIdentifierString(name string) string {
	count := 0

	if c, ok := c.identifiers[name]; ok {
		count = c
	}

	count++
	c.identifiers[name] = count

	return name + "." + strconv.FormatInt(int64(count), 10)
}

func (c *codegen) visitLoad(expr ast.Expr) llvm.Value {
	value := c.visit(expr)

	if expr.Result().Kind == ast.Address {
		if _, ok := value.(*llvm.Function); !ok {
			if _, ok := value.(*llvm.ExternFunction); !ok {
				value = llvm.Load(c.fun, value, "")
			}
		}
	}

	return value
}

func (c *codegen) visit(expr ast.Expr) llvm.Value {
	c.setSourceLocation(expr)

	c.exprValue = llvm.IdentifierValue{}
	expr.Visit(c)
	return c.exprValue
}

func (c *codegen) setSourceLocation(node ast.Node) {
	loc := node.Range().Start

	if loc.Line != 0 && loc.Column != 0 {
		c.fun.SetSourceLocation(loc.Line, loc.Column)
	}
}
