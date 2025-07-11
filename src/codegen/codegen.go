package codegen

import (
	"fireball/abi"
	"fireball/analyzer"
	"fireball/ast"
	"fireball/lexer"
	"fireball/llvm"
	"fireball/utils"
	"strconv"
	"strings"
)

type codegen struct {
	module              *llvm.Module
	stringConstantCount int
	arch                abi.Arch
	callConv            abi.CallConv

	types TypeCache

	globalVars                   map[*ast.GlobalVar]*llvm.GlobalValue
	additionalExternalGlobalVars map[*ast.GlobalVar]any

	functions                   map[*ast.Func]*llvm.ExternFunction
	additionalExternalFunctions map[*ast.Func]any

	fun          *llvm.Function
	funReturnPtr llvm.Value
	identifiers  map[string]int

	variables analyzer.VariableTracker[llvm.IdentifierValue]
	allocas   map[ast.Expr]llvm.IdentifierValue
	allocas2  map[ast.Expr]llvm.IdentifierValue

	loopConditionL llvm.Identifier
	loopEndL       llvm.Identifier

	exprValue llvm.Value
}

func Emit(file *ast.File, path string, arch abi.Arch, callConv abi.CallConv) *llvm.Module {
	module := llvm.NewModule(path, "", "")

	c := codegen{
		module:   module,
		arch:     arch,
		callConv: callConv,

		types: TypeCache{Arch: arch, CallConv: callConv, Module: module},

		globalVars:                   make(map[*ast.GlobalVar]*llvm.GlobalValue),
		additionalExternalGlobalVars: make(map[*ast.GlobalVar]any),

		functions:                   make(map[*ast.Func]*llvm.ExternFunction),
		additionalExternalFunctions: make(map[*ast.Func]any),
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Impl:
			for _, method := range decl.Methods {
				c.collectFunc(method)
			}

		case *ast.GlobalVar:
			c.collectGlobalVar(decl)

		case *ast.Func:
			c.collectFunc(decl)
		}
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Impl:
			for _, method := range decl.Methods {
				c.VisitFunc(method)
			}

		case *ast.Func:
			c.VisitFunc(decl)
		}
	}

	for g := range c.additionalExternalGlobalVars {
		c.module.NewExternGlobalVariable(GetGlobalVarLinkName(g), c.types.Get(g.Type))
	}

	for f := range c.additionalExternalFunctions {
		c.module.NewExternFunction(GetFuncLinkName(f), c.types.Get(f))
	}

	return c.module
}

// Declarations

func (c *codegen) collectGlobalVar(g *ast.GlobalVar) {
	c.globalVars[g] = c.module.NewGlobalVariable(GetGlobalVarLinkName(g), c.types.Get(g.Type))
}

func (c *codegen) collectFunc(f *ast.Func) {
	value := llvm.FakeFunctionValue(c.types.Get(f), GetFuncLinkName(f))
	c.functions[f] = &value
}

func (c *codegen) VisitFunc(f *ast.Func) {
	// Header
	type_ := c.types.Get(f)

	returnTypeRegs := c.callConv.Classify(f.ReturnType())

	if ast.IsValid(f.Body) {
		paramNames := make([]string, 0, len(f.Params))

		if len(returnTypeRegs) == 1 && returnTypeRegs[0].Class == abi.Memory {
			paramNames = append(paramNames, "func.return_value")
		}

		if _, ok := f.Parent().(*ast.Impl); ok {
			paramNames = append(paramNames, "this")
		}

		for _, param := range f.Params {
			paramNames = append(paramNames, param.Name.Token.Text)
		}

		c.fun = c.module.NewFunction(GetFuncLinkName(f), f.Name(), type_, paramNames)
	} else {
		c.module.NewExternFunction(GetFuncLinkName(f), type_)
		return
	}

	// Body
	c.variables.PushScope()

	c.fun.Block(llvm.NamedIdentifier("func.entry"))

	c.identifiers = make(map[string]int)
	c.allocas = make(map[ast.Expr]llvm.IdentifierValue)
	c.allocas2 = make(map[ast.Expr]llvm.IdentifierValue)
	c.collectAllocas(f.Body)

	c.funReturnPtr = nil
	if len(returnTypeRegs) == 1 && returnTypeRegs[0].Class == abi.Memory {
		c.funReturnPtr = llvm.NamedIdentifierValue(c.types.Get(f.ReturnType()), "func.return_value")
	}

	paramI := uint32(1)

	if impl, ok := f.Parent().(*ast.Impl); ok {
		c.setSourceLocation(f.NameN)

		astType := ast.GetStructPointerType(impl.Struct)
		llvmType := c.types.Get(astType)
		_, align := abi.TypeInfo(c.arch, astType)

		ptr := llvm.Alloca(c.fun, llvmType, 1, align, "param.this")
		c.fun.LocalVariable(ptr, "this", paramI)

		value := llvm.NamedIdentifierValue(llvmType, "this")
		llvm.Store(c.fun, value, ptr)

		c.variables.Add("this", astType, ptr)

		paramI++
	}

	for _, param := range f.Params {
		c.setSourceLocation(param)

		type_ := c.types.Get(param.Type)
		_, align := abi.TypeInfo(c.arch, param.Type)

		ptr := llvm.Alloca(c.fun, type_, 1, align, "param."+param.Name.Token.Text)
		c.fun.LocalVariable(ptr, param.Name.Token.Text, paramI)

		t := getClassifiedLlvmType(&c.types, param.Type, false)
		value := llvm.NamedIdentifierValue(t, param.Name.Token.Text)

		regs := c.callConv.Classify(param.Type)
		if len(regs) == 1 && regs[0].Class == abi.Memory {
			value = llvm.Load(c.fun, value, "")
		}

		llvm.Store(c.fun, value, ptr)

		c.variables.Add(param.Name.Token.Text, param.Type, ptr)

		paramI++
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

	ptr := c.allocas[v]
	c.fun.LocalVariable(ptr, v.Name.Token.Text, 0)

	if ast.IsValid(v.Value) {
		type_ = v.Value.Result().Type

		value := c.visitLoadImplicitCast(v.Value, v.Type)
		llvm.Store(c.fun, value, ptr)
	} else {
		type_ = v.Type

		value := llvm.ZeroInitialize(c.types.Get(type_))
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
	condition := c.visitLoadImplicitCast(i.Condition, ast.BoolType)
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
	prevLoopConditionL := c.loopConditionL
	prevLoopEndL := c.loopEndL

	c.loopConditionL = c.getNamedIdentifier("while.condition")
	bodyL := c.getNamedIdentifier("while.body")
	c.loopEndL = c.getNamedIdentifier("while.end")

	llvm.Br(c.fun, c.loopConditionL)

	// Condition
	c.fun.Block(c.loopConditionL)
	condition := c.visitLoadImplicitCast(w.Condition, ast.BoolType)
	llvm.BrCond(c.fun, condition, bodyL, c.loopEndL)

	// Body
	c.fun.Block(bodyL)
	c.visit(w.Body)
	llvm.Br(c.fun, c.loopConditionL)

	// End
	c.fun.Block(c.loopEndL)

	c.loopConditionL = prevLoopConditionL
	c.loopEndL = prevLoopEndL
}

func (c *codegen) VisitBreak(b *ast.Break) {
	llvm.Br(c.fun, c.loopEndL)
}

func (c *codegen) VisitContinue(co *ast.Continue) {
	llvm.Br(c.fun, c.loopConditionL)
}

func (c *codegen) VisitReturn(r *ast.Return) {
	if ast.IsValid(r.Value) {
		f := ast.Parent[*ast.Func](r)

		value := c.visitLoadClassified(r.Value, f.ReturnType(), func() llvm.Value {
			return c.funReturnPtr
		}, true)

		if c.funReturnPtr != nil {
			llvm.Ret(c.fun)
		} else {
			llvm.RetValue(c.fun, value)
		}
	} else {
		llvm.Ret(c.fun)
	}
}

func (c *codegen) VisitLiteral(l *ast.Literal) {
	str := l.Value.Token.Text

	switch l.Value.Token.Kind {
	case lexer.Identifier:
		if l.Value.Token.Text == "nil" {
			c.exprValue = llvm.Null()
		} else if l.Value.Token.Text == "true" {
			c.exprValue = llvm.True()
		} else {
			c.exprValue = llvm.False()
		}

	case lexer.Integer:
		if strings.ContainsAny(str, "uU") {
			v, _ := strconv.ParseUint(str[:len(str)-1], 10, 64)
			c.exprValue = llvm.Uint(c.types.Get(l.Result().Type), v)
		} else {
			v, _ := strconv.ParseInt(str, 10, 64)
			c.exprValue = llvm.Int(c.types.Get(l.Result().Type), v)
		}

	case lexer.Floating:
		if strings.ContainsAny(str, "fF") {
			v, _ := strconv.ParseFloat(str[:len(str)-1], 32)
			c.exprValue = llvm.Float(float32(v))
		} else {
			v, _ := strconv.ParseFloat(str, 64)
			c.exprValue = llvm.Double(v)
		}

	case lexer.Hexadecimal:
		v, _ := strconv.ParseUint(str[2:], 16, 64)
		c.exprValue = llvm.Uint(c.types.Get(l.Result().Type), v)

	case lexer.Binary:
		v, _ := strconv.ParseUint(str[2:], 2, 64)
		c.exprValue = llvm.Uint(c.types.Get(l.Result().Type), v)

	case lexer.Character:
		char := l.Value.Token.Text[1 : len(l.Value.Token.Text)-1]
		var number uint8

		switch char {
		case "\\0":
			number = '\000'
		case "\\a":
			number = '\a'
		case "\\b":
			number = '\n'
		case "\\f":
			number = '\f'
		case "\\n":
			number = '\n'
		case "\\r":
			number = '\r'
		case "\\t":
			number = '\t'
		case "\\v":
			number = '\v'

		default:
			number = char[len(char)-1]
		}

		c.exprValue = llvm.Uint(c.types.Get(l.Result().Type), uint64(number))

	case lexer.String:
		c.stringConstantCount++

		s := stringBuilder{}
		analyzer.ParseString(l.Value.Token.Text[1:len(l.Value.Token.Text)-1], &s)
		s.WriteRune('\000')

		c.exprValue = c.module.NewStringConstant(
			"string."+strconv.FormatInt(int64(c.stringConstantCount), 10),
			s.String(),
			s.length,
		)

	default:
		panic("codegen.codegen.VisitLiteral() - Invalid token kind")
	}
}

func (c *codegen) VisitStructInitializer(s *ast.StructInitializer) {
	var v llvm.Value = llvm.ZeroInitialize(c.types.Get(s.Result().Type))

	for _, field := range s.Fields {
		f, i := s.Struct.GetField(field.Name.Token.Text)

		value := c.visitLoadImplicitCast(field.Value, f.Type)
		v = llvm.InsertValue(c.fun, v, value, uint32(i), "")
	}

	c.exprValue = v
}

func (c *codegen) VisitParen(p *ast.Paren) {
	c.exprValue = c.visit(p.Expr)
}

func (c *codegen) VisitIdentifier(i *ast.Identifier) {
	switch node := i.Resolved.(type) {
	case *ast.Func:
		c.exprValue = c.getValueForFunc(node)

	case *ast.Var, *ast.Impl, *ast.Param:
		_, c.exprValue = c.variables.Find(i.Path.SegmentAt(0))

		if utils.IsNil(c.exprValue) {
			panic("codegen.codegen.VisitIdentifier() - Failed to find local variable")
		}

	case *ast.GlobalVar:
		c.exprValue = c.getValueForGlobalVar(node)

	default:
		panic("codegen.codegen.VisitIdentifier() - Invalid node type")
	}
}

func (c *codegen) VisitCall(call *ast.Call) {
	callee := c.visitLoad(call.Callee)
	args := make([]llvm.Value, 0, len(call.Args))

	f, _ := call.Callee.Result().Type.(ast.FuncType)
	regs := c.callConv.Classify(f.ReturnType())

	if len(regs) == 1 && regs[0].Class == abi.Memory {
		ptr := c.allocas[call]
		args = append(args, ptr)
	}

	if _, ok := f.Parent().(*ast.Impl); ok {
		expr := call.Callee.(*ast.Member).Value
		value := c.visit(expr)

		if expr.Result().Kind == ast.Value {
			ptr := c.allocas2[call]
			llvm.Store(c.fun, value, ptr)

			value = ptr
		}

		args = append(args, value)
	}

	for i, arg := range call.Args {
		var paramType ast.Type

		if i < f.ParamTypeCount() {
			paramType = f.ParamTypeAt(i)
		}

		args = append(args, c.visitLoadClassified(arg, paramType, func() llvm.Value {
			return c.allocas[arg]
		}, false))
	}

	callBuilder := llvm.Call(c.fun, callee, "")

	for _, arg := range args {
		llvm.Arg(&callBuilder, arg)
	}

	c.exprValue = callBuilder.End()

	if len(regs) > 0 {
		if regs[0].Class == abi.Memory {
			ptr := c.allocas[call]
			c.exprValue = c.declassify(call, f.ReturnType(), ptr)
		} else {
			c.exprValue = c.declassify(call, f.ReturnType(), c.exprValue)
		}
	}
}

func (c *codegen) VisitIndex(i *ast.Index) {
	ptr := c.visit(i.Value)
	index := c.visitLoad(i.Index)

	if _, ok := i.Value.Result().Type.(*ast.PointerType); ok {
		ptr = c.load(i.Value, ptr)
		c.exprValue = llvm.GetElementPtr1(c.fun, ptr, index, "")
	} else {
		c.exprValue = llvm.GetElementPtr2Dyn(c.fun, ptr, llvm.Int(llvm.I32, 0), index, "")
	}
}

func (c *codegen) VisitMember(m *ast.Member) {
	// Enum case
	if i, ok := m.Value.(*ast.Identifier); ok {
		if decl, ok := i.Resolved.(*ast.Enum); ok {
			backing := decl.ActualType.(*ast.PrimitiveType)
			value := m.Resolved.(*ast.EnumCase).ActualValue

			if backing.Kind.IsSignedInteger() {
				c.exprValue = llvm.Int(c.types.Get(backing), value.Signed())
			} else {
				c.exprValue = llvm.Uint(c.types.Get(backing), value.Unsigned())
			}

			return
		}
	}

	// Member
	var decl *ast.Struct
	isPtr := false

	if p, ok := m.Value.Result().Type.(*ast.PointerType); ok {
		decl = p.Pointee.(*ast.DeclType).Decl.(*ast.Struct)
		isPtr = true
	} else {
		decl = m.Value.Result().Type.(*ast.DeclType).Decl.(*ast.Struct)
	}

	// Method
	if f, ok := m.Result().Type.(*ast.Func); ok {
		c.exprValue = c.getValueForFunc(f)
		return
	}

	// Field
	_, i := decl.GetField(m.Name.Token.Text)

	if m.Value.Result().Kind == ast.Value {
		if _, ok := m.Value.Result().Type.(*ast.PointerType); ok {
			ptr := c.visitLoad(m.Value)
			ptr = llvm.GetElementPtr2Const(c.fun, ptr, 0, uint32(i), "")

			c.exprValue = llvm.Load(c.fun, ptr, "")
		} else {
			value := c.visitLoad(m.Value)

			c.exprValue = llvm.ExtractValue(c.fun, value, uint32(i), "")
		}
	} else {
		ptr := c.visit(m.Value)

		if isPtr {
			ptr = llvm.Load(c.fun, ptr, "")
		}

		c.exprValue = llvm.GetElementPtr2Const(c.fun, ptr, 0, uint32(i), "")
	}
}

func (c *codegen) VisitUnary(u *ast.Unary) {
	if u.Postfix {
		switch u.Op {
		// ++, --
		case lexer.PlusPlus, lexer.MinusMinus:
			ptr := c.visit(u.Expr)

			oldValue := llvm.Load(c.fun, ptr, "")
			newValue := c.binarySimple(u.Op, oldValue, c.getConstantOne(u.Expr.Result().Type))

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
			newValue := c.binarySimple(u.Op, oldValue, c.getConstantOne(u.Expr.Result().Type))

			llvm.Store(c.fun, newValue, ptr)
			c.exprValue = newValue

		// -
		case lexer.Minus:
			value := c.visitLoad(u.Expr)

			if u.Expr.Result().Type.(*ast.PrimitiveType).Kind.IsFloating() {
				c.exprValue = llvm.NegF(c.fun, value, "")
			} else {
				c.exprValue = llvm.Sub(c.fun, llvm.Int(c.types.Get(u.Expr.Result().Type), 0), value, "")
			}

		// !
		case lexer.Bang:
			value := c.visitLoadImplicitCast(u.Expr, ast.BoolType)
			c.exprValue = llvm.Xor(c.fun, value, llvm.True(), "")

		// &
		case lexer.Ampersand:
			c.exprValue = c.visit(u.Expr)

		// *
		case lexer.Star:
			c.exprValue = c.visitLoad(u.Expr)

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
		left := c.visitLoadImplicitCast(b.Left, ast.BoolType)
		llvm.BrCond(c.fun, left, exitL, rightL)

		// Right
		c.fun.Block(rightL)
		right := c.visitLoadImplicitCast(b.Right, ast.BoolType)
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
		left := c.visitLoadImplicitCast(b.Left, ast.BoolType)
		llvm.BrCond(c.fun, left, rightL, exitL)

		// Right
		c.fun.Block(rightL)
		right := c.visitLoadImplicitCast(b.Right, ast.BoolType)
		llvm.Br(c.fun, exitL)

		// Exit
		c.fun.Block(exitL)
		c.exprValue = llvm.Phi(c.fun, llvm.False(), leftL, right, rightL, "")

	// Equality
	case lexer.EqualEqual, lexer.BangEqual:
		left := c.visitLoad(b.Left)
		right := c.visitLoad(b.Right)

		switch type_ := b.Left.Result().Type.(type) {
		case *ast.PrimitiveType:
			kind := type_.Kind

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

		case *ast.PointerType:
			c.exprValue = llvm.CmpI(c.fun, llvm.IEQ, left, right, "")

		case *ast.DeclType:
			switch type_.Decl.(type) {
			case *ast.Enum:
				c.exprValue = llvm.CmpI(c.fun, llvm.IEQ, left, right, "")

			default:
				panic("codegen.codegen.VisitBinary() - Equality - Invalid DeclType declaration")
			}

		default:
			panic("codegen.codegen.VisitBinary() - Equality - Invalid type")
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
		if b.Range().Start.Line == 9 {
			println()
		}

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
	case lexer.Plus, lexer.PlusEqual, lexer.PlusPlus:
		return llvm.Add(c.fun, left, right, "")
	case lexer.Minus, lexer.MinusEqual, lexer.MinusMinus:
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

func (c *codegen) VisitCast(cast *ast.Cast) {
	value := c.visitLoad(cast.Value)

	kind, ok := ast.GetCastKind(cast.Value.Result().Type, cast.Type, false)
	if !ok {
		panic("codegen.codegen.VisitCast() - Invalid cast kind")
	}

	c.exprValue = c.cast(value, cast.Type, kind)
}

func (c *codegen) cast(value llvm.Value, to ast.Type, kind ast.CastKind) llvm.Value {
	type_ := c.types.Get(to)

	switch kind {
	case ast.Nop:
		return llvm.ChangeValueType(value, type_)
	case ast.Extend:
		return llvm.Ext(c.fun, value, type_, "")
	case ast.Truncate:
		return llvm.Trunc(c.fun, value, type_, "")
	case ast.IntegerToFloating:
		return llvm.IntToFloating(c.fun, value, type_, "")
	case ast.FloatingToInteger:
		return llvm.FloatingToInt(c.fun, value, type_, "")
	case ast.IntegerToPointer:
		return llvm.IntToPtr(c.fun, value, type_, "")
	case ast.PointerToInteger:
		return llvm.PtrToInt(c.fun, value, type_, "")

	default:
		panic("codegen.codegen.cast() - Invalid cast kind")
	}
}

// Utils

func (c *codegen) getValueForGlobalVar(g *ast.GlobalVar) llvm.Value {
	if v, ok := c.globalVars[g]; ok {
		return v
	} else {
		v := llvm.FakeGlobalValue(c.types.Get(g.Type), GetGlobalVarLinkName(g))
		c.additionalExternalGlobalVars[g] = nil

		return &v
	}
}

func (c *codegen) getValueForFunc(f *ast.Func) llvm.Value {
	if v, ok := c.functions[f]; ok {
		return v
	} else {
		v := llvm.FakeFunctionValue(c.types.Get(f), GetFuncLinkName(f))
		c.additionalExternalFunctions[f] = nil

		return &v
	}
}

func (c *codegen) getConstantOne(type_ ast.Type) llvm.Value {
	kind := type_.(*ast.PrimitiveType).Kind

	if kind == ast.F32 {
		return llvm.Float(1)
	}
	if kind == ast.F64 {
		return llvm.Double(1)
	}

	return llvm.Int(c.types.Get(type_), 1)
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

func (c *codegen) load(expr ast.Expr, value llvm.Value) llvm.Value {
	if expr.Result().Kind == ast.Address {
		if _, ok := value.(*llvm.Function); !ok {
			if _, ok := value.(*llvm.ExternFunction); !ok {
				value = llvm.Load(c.fun, value, "")
			}
		}
	}

	return value
}

func (c *codegen) implicitCast(value llvm.Value, from, to ast.Type) llvm.Value {
	if ast.IsValid(from) && ast.IsValid(to) {
		if kind, ok := ast.GetImplicitCastKind(from, to); ok {
			value = c.cast(value, to, kind)
		}
	}

	return value
}

func (c *codegen) visitLoadImplicitCast(expr ast.Expr, to ast.Type) llvm.Value {
	value := c.visitLoad(expr)
	value = c.implicitCast(value, expr.Result().Type, to)

	return value
}

func (c *codegen) visitLoad(expr ast.Expr) llvm.Value {
	value := c.visit(expr)
	return c.load(expr, value)
}

func (c *codegen) visit(expr ast.Expr) llvm.Value {
	c.setSourceLocation(expr)

	if expr.Result().Kind == ast.Invalid {
		panic("codegen.codegen.Visit() - Expression result is invalid.")
	}

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
