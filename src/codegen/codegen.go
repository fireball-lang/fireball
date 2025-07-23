package codegen

import (
	"fireball/abi"
	"fireball/analyzer"
	"fireball/ast"
	"fireball/ir"
	"fireball/lexer"
	"fireball/utils"
	"slices"
	"strconv"
	"strings"
)

type codegen struct {
	module              *ir.Module
	stringConstantCount int
	arch                abi.Arch
	callConv            abi.CallConv

	types   TypeCache
	emitter ir.Emitter

	fileRef ir.MetaRef
	unitRef ir.MetaRef

	globalVars map[*ast.GlobalVar]*ir.GlobalVar
	functions  map[*ast.Func]*ir.Function

	fun          *ir.Function
	funReturnPtr ir.Value

	variables analyzer.VariableTracker[ir.Value]

	loopConditionL *ir.Block
	loopEndL       *ir.Block

	exprValue ir.Value
}

func Emit(file *ast.File, path string, arch abi.Arch, callConv abi.CallConv) *ir.Module {
	module := ir.NewModule()
	module.Path = path

	c := codegen{
		module:   module,
		arch:     arch,
		callConv: callConv,

		types:   TypeCache{Arch: arch, CallConv: callConv, Module: module},
		emitter: ir.Emitter{Module: module},

		globalVars: make(map[*ast.GlobalVar]*ir.GlobalVar),
		functions:  make(map[*ast.Func]*ir.Function),
	}

	// Setup meta

	c.fileRef = module.AddMeta(&ir.FileMeta{
		Path: path,
	})
	c.emitter.PushScope(c.fileRef)

	c.types.FileRef = c.fileRef

	c.unitRef = module.AddMeta(&ir.CompileUnitMeta{
		File:          c.fileRef,
		Producer:      "fireball",
		IsOptimized:   false,
		Enums:         0,
		RetainedTypes: 0,
		Globals:       0,
		Imports:       0,
	})

	c.module.AddNamedMetaRefs(
		"llvm.dbg.cu",
		c.unitRef,
	)

	c.module.AddNamedMetaRefs(
		"llvm.module.flags",
		c.module.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "Dwarf Version"},
			{Number: 4},
		}}),
		c.module.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 2},
			{Text: "Debug Info Version"},
			{Number: 3},
		}}),
		c.module.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 1},
			{Text: "wchar_size"},
			{Number: 4},
		}}),
		c.module.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 8},
			{Text: "PIC Level"},
			{Number: 2},
		}}),
		c.module.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "PIE Level"},
			{Number: 2},
		}}),
		c.module.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "uwtable"},
			{Number: 2},
		}}),
		c.module.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "frame-pointer"},
			{Number: 2},
		}}),
	)

	c.module.AddNamedMetaRefs(
		"llvm.ident",
		c.module.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Text: "fireball"},
		}}),
	)

	// Collect symbols

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.Struct, *ast.Enum:
			c.newTypeInfo(decl)

		case *ast.Impl:
			for _, method := range decl.Methods {
				c.newFunction(method)
			}

		case *ast.GlobalVar:
			c.newGlobalVar(decl)

		case *ast.Func:
			c.newFunction(decl)
		}
	}

	// Create vtables

	for _, decl := range file.Decls {
		if decl, ok := decl.(*ast.Impl); ok {
			if ast.IsValid(decl.Interface) {
				c.newVTable(decl)
			}
		}
	}

	// Emit functions

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

	return c.module
}

// Declarations

func (c *codegen) newTypeInfo(decl ast.Decl) {
	gVar := c.module.NewGlobalVar(GetTypeInfoLinkName(decl), ir.I8)
	gVar.Flags = ir.Constant
	gVar.Initializer = &ir.ZeroInitializer{Typ: ir.I8}
}

func (c *codegen) newVTable(impl *ast.Impl) {
	typ := &ir.ArrayType{
		Length:  1 + uint32(len(impl.Interface.Methods)),
		Element: ir.Pointer,
	}

	pointers := make([]ir.Value, 0, typ.Length)
	pointers = append(pointers, c.getTypeInfoPointer(impl.Decl))

	for _, method := range impl.Interface.Methods {
		for _, function := range impl.Methods {
			if method.Name() == function.Name() && ast.FuncSignatureEquals(method, function) {
				pointers = append(pointers, c.functions[function])
				break
			}
		}
	}

	gVar := c.module.NewGlobalVar(GetVTableLinkName(impl.Decl, impl.Interface), typ)
	gVar.Flags = ir.Constant
	gVar.Initializer = &ir.Array{Elements: pointers}
}

func (c *codegen) newGlobalVar(g *ast.GlobalVar) {
	typ := c.types.Get(g.Type)

	gVar := c.module.NewGlobalVar(GetGlobalVarLinkName(g), typ)
	gVar.Initializer = &ir.ZeroInitializer{Typ: typ}

	c.globalVars[g] = gVar
}

func (c *codegen) newFunction(f *ast.Func) *ir.Function {
	typ := c.types.Get(f).(*ir.FunctionType)
	paramNames := make([]string, 0, len(typ.Params))

	returnTypeRegs := c.callConv.Classify(f.ReturnType())
	if len(returnTypeRegs) == 1 && returnTypeRegs[0].Class == abi.Memory {
		paramNames = append(paramNames, "func.return_value")
	}

	if _, ok := f.Parent().(*ast.Impl); ok {
		paramNames = append(paramNames, "this")
	}

	for _, param := range f.Params {
		paramNames = append(paramNames, param.Name.Token.Text)
	}

	fun := c.module.NewFunction(GetFuncLinkName(f), typ, paramNames)

	if f.Body != nil {
		fun.Flags = ir.DsoLocal
	} else {
		fun.Flags = ir.Declare | ir.DsoLocal
	}

	c.functions[f] = fun
	return fun
}

func (c *codegen) VisitFunc(f *ast.Func) {
	if f.Body == nil {
		return
	}

	// Meta
	typeRef := c.module.GetMeta(c.types.GetMeta(f)).(*ir.DerivedTypeMeta).Base

	ref := c.module.AddMeta(&ir.SubprogramMeta{
		Name:     f.Name(),
		LinkName: GetFuncLinkName(f),
		Type:     typeRef,
		Scope:    c.emitter.PeekScope(),
		Unit:     c.unitRef,
		File:     c.fileRef,
		Line:     getClosestValidRange(f).Start.Line,
	})

	c.emitter.PushScope(ref)
	defer c.emitter.PopScope()

	// Body

	c.fun = c.functions[f]
	c.fun.SetMeta(ref)

	returnTypeRegs := c.callConv.Classify(f.ReturnType())

	c.variables.PushScope()

	c.emitter.Begin(c.fun.NewBlock("func.entry"))

	paramValueI := 0

	c.funReturnPtr = nil
	if len(returnTypeRegs) == 1 && returnTypeRegs[0].Class == abi.Memory {
		c.funReturnPtr = c.fun.ParamValues[paramValueI]
		paramValueI++
	}

	argI := uint32(1)

	if impl, ok := f.Parent().(*ast.Impl); ok {
		c.emitter.SetDebugLocation(getClosestValidRange(f.NameN).Start)

		astType := ast.GetDeclPointerType(impl.Decl)
		typ := c.types.Get(astType)

		ptr := c.emitter.Alloca(typ, 1)
		ptr.SetName("param.this")

		c.emitDbgDeclare("this", astType, ptr, argI, f)
		argI++

		value := c.fun.ParamValues[paramValueI]
		paramValueI++

		c.emitter.Store(value, ptr)

		c.variables.Add("this", astType, ptr)
	}

	for _, param := range f.Params {
		c.emitter.SetDebugLocation(getClosestValidRange(param).Start)

		typ := c.types.Get(param.Type)

		ptr := c.emitter.Alloca(typ, 1)
		ptr.SetName("param." + param.Name.Token.Text)

		c.emitDbgDeclare(param.Name.Token.Text, param.Type, ptr, argI, param)
		argI++

		value := c.fun.ParamValues[paramValueI]
		paramValueI++

		regs := c.callConv.Classify(param.Type)
		if len(regs) == 1 && regs[0].Class == abi.Memory {
			value = c.emitter.Load(c.types.Get(param.Type), value)
		}

		c.emitter.Store(value, ptr)

		c.variables.Add(param.Name.Token.Text, param.Type, ptr)
	}

	c.visit(f.Body)

	if f.ReturnType().Equals(ast.VoidType) {
		expr := ast.GetLastExpr(f.Body)

		if _, ok := expr.(*ast.Return); !ok {
			c.emitter.Ret(nil)
		}
	}

	c.variables.PopScope()
	c.fun = nil
}

// Expressions

func (c *codegen) VisitBlock(b *ast.Block) {
	range_ := getClosestValidRange(b)

	c.emitter.PushScope(c.module.AddMeta(&ir.LexicalBlockMeta{
		Scope:  c.emitter.PeekScope(),
		File:   c.fileRef,
		Line:   range_.Start.Line,
		Column: range_.Start.Column,
	}))
	c.variables.PushScope()

	for _, expr := range b.Exprs {
		c.visit(expr)
	}

	c.variables.PopScope()
	c.emitter.PopScope()
}

func (c *codegen) VisitVar(v *ast.Var) {
	type_ := v.ActualType()
	typ := c.types.Get(type_)

	ptr := c.emitAlloca("var."+v.Name.Token.Text, typ)
	c.emitDbgDeclare(v.Name.Token.Text, v.ActualType(), ptr, 0, v)

	var value ir.Value

	if ast.IsValid(v.Value) {
		value = c.visitLoadImplicitCast(v.Value, v.Type)
	} else {
		value = &ir.ZeroInitializer{Typ: typ}
	}

	c.emitter.Store(value, ptr)

	c.variables.Add(v.Name.Token.Text, type_, ptr)
}

func (c *codegen) VisitIf(i *ast.If) {
	trueL := c.fun.NewBlock("if.true")
	falseL := c.fun.NewBlock("if.end")
	endL := falseL

	if ast.IsValid(i.Else) {
		falseL = c.fun.NewBlock("if.false")
	}

	// Condition
	condition := c.visitLoadImplicitCast(i.Condition, ast.BoolType)
	c.emitter.BrCond(condition, trueL, falseL)

	// Then
	c.emitter.Begin(trueL)
	c.visit(i.Then)
	c.emitter.Br(endL)

	// Else
	if ast.IsValid(i.Else) {
		c.emitter.Begin(falseL)
		c.visit(i.Else)
		c.emitter.Br(endL)
	}

	// End
	c.emitter.Begin(endL)
}

func (c *codegen) VisitWhile(w *ast.While) {
	prevLoopConditionL := c.loopConditionL
	prevLoopEndL := c.loopEndL

	c.loopConditionL = c.fun.NewBlock("while.condition")
	bodyL := c.fun.NewBlock("while.body")
	c.loopEndL = c.fun.NewBlock("while.end")

	c.emitter.Br(c.loopConditionL)

	// Condition
	c.emitter.Begin(c.loopConditionL)
	condition := c.visitLoadImplicitCast(w.Condition, ast.BoolType)
	c.emitter.BrCond(condition, bodyL, c.loopEndL)

	// Body
	c.emitter.Begin(bodyL)
	c.visit(w.Body)
	c.emitter.Br(c.loopConditionL)

	// End
	c.emitter.Begin(c.loopEndL)

	c.loopConditionL = prevLoopConditionL
	c.loopEndL = prevLoopEndL
}

func (c *codegen) VisitBreak(b *ast.Break) {
	c.emitter.Br(c.loopEndL)
}

func (c *codegen) VisitContinue(co *ast.Continue) {
	c.emitter.Br(c.loopConditionL)
}

func (c *codegen) VisitReturn(r *ast.Return) {
	var value ir.Value

	if ast.IsValid(r.Value) {
		f := ast.Parent[*ast.Func](r)

		value = c.visitLoadClassified(r.Value, f.ReturnType(), func() ir.Value {
			return c.funReturnPtr
		}, true)

		if c.funReturnPtr != nil {
			value = nil
		}
	}

	c.emitter.Ret(value)
}

func (c *codegen) VisitLiteral(l *ast.Literal) {
	str := l.Value.Token.Text

	switch l.Value.Token.Kind {
	case lexer.Identifier:
		if l.Value.Token.Text == "nil" {
			c.exprValue = &ir.Null{}
		} else if l.Value.Token.Text == "true" {
			c.exprValue = ir.True
		} else {
			c.exprValue = ir.False
		}

	case lexer.Integer:
		var value utils.Integer

		if strings.ContainsAny(str, "uU") {
			v, _ := strconv.ParseUint(str[:len(str)-1], 10, 64)
			value = utils.Unsigned(false, v)
		} else {
			v, _ := strconv.ParseInt(str, 10, 64)
			value = utils.Signed(v)
		}

		c.exprValue = &ir.Integer{
			Typ:   c.types.Get(l.Result().Type),
			Value: value,
		}

	case lexer.Floating:
		if strings.ContainsAny(str, "fF") {
			v, _ := strconv.ParseFloat(str[:len(str)-1], 32)
			c.exprValue = &ir.FloatV{Value: float32(v)}
		} else {
			v, _ := strconv.ParseFloat(str, 64)
			c.exprValue = &ir.DoubleV{Value: v}
		}

	case lexer.Hexadecimal:
		v, _ := strconv.ParseUint(str[2:], 16, 64)

		c.exprValue = &ir.Integer{
			Typ:   c.types.Get(l.Result().Type),
			Value: utils.Unsigned(false, v),
		}

	case lexer.Binary:
		v, _ := strconv.ParseUint(str[2:], 2, 64)

		c.exprValue = &ir.Integer{
			Typ:   c.types.Get(l.Result().Type),
			Value: utils.Unsigned(false, v),
		}

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

		c.exprValue = &ir.Integer{
			Typ:   c.types.Get(l.Result().Type),
			Value: utils.Unsigned(false, uint64(number)),
		}

	case lexer.String:
		c.stringConstantCount++

		s := stringBuilder{}
		analyzer.ParseString(l.Value.Token.Text[1:len(l.Value.Token.Text)-1], &s)
		s.WriteEscapeSequence(0)

		value := &ir.String{
			Length: s.length,
			Value:  s.String(),
		}

		gVar := c.module.NewGlobalVar(
			"string."+strconv.FormatInt(int64(c.stringConstantCount), 10),
			value.Type(),
		)

		gVar.Flags = ir.Private | ir.UnnamedAddr | ir.Constant
		gVar.Initializer = value

		c.exprValue = gVar

	default:
		panic("codegen.codegen.VisitLiteral() - Invalid token kind")
	}
}

func (c *codegen) VisitStructInitializer(s *ast.StructInitializer) {
	var v ir.Value = &ir.ZeroInitializer{Typ: c.types.Get(s.Result().Type)}

	for _, field := range s.Fields {
		f, i := s.Struct.GetField(field.Name.Token.Text)

		value := c.visitLoadImplicitCast(field.Value, f.Type)
		v = c.emitter.InsertValue(v, value, uint32(i))
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
	args := make([]ir.Value, 0, len(call.Args))

	f, _ := call.Callee.Result().Type.(ast.FuncType)
	regs := c.callConv.Classify(f.ReturnType())
	var returnPtr ir.Value

	if len(regs) == 1 && regs[0].Class == abi.Memory {
		returnPtr = c.emitAlloca("abi.call.return", c.types.Get(f.ReturnType()))
		args = append(args, returnPtr)
	}

	switch f.Parent().(type) {
	case *ast.Impl:
		expr := call.Callee.(*ast.Member).Value
		value := c.visit(expr)

		if expr.Result().Kind == ast.Value {
			ptr := c.emitAlloca("abi.this", c.types.Get(expr.Result().Type))
			c.emitter.Store(value, ptr)

			value = ptr
		}

		args = append(args, value)

	case *ast.Interface:
		value := callee.(*ir.Load).Pointer.(*ir.GetElementPtrConst).Pointer.(*ir.ExtractValue).Value
		dataPtr := c.emitter.ExtractValue(value, 0)

		args = append(args, dataPtr)
	}

	for i, arg := range call.Args {
		var paramType ast.Type

		if i < f.ParamTypeCount() {
			paramType = f.ParamTypeAt(i)
		}

		args = append(args, c.visitLoadClassified(arg, paramType, func() ir.Value {
			return c.emitAlloca("abi.arg", c.types.Get(paramType))
		}, false))
	}

	c.exprValue = c.emitter.Call(c.types.Get(f), callee, args)

	if len(regs) > 0 {
		if regs[0].Class == abi.Memory {
			c.exprValue = c.declassify(call, f.ReturnType(), returnPtr)
		} else {
			c.exprValue = c.declassify(call, f.ReturnType(), c.exprValue)
		}
	}
}

func (c *codegen) VisitTypeCall(t *ast.TypeCall) {
	size, align := abi.TypeInfo(c.arch, t.Arg)

	value := size
	if t.Kind == ast.Alignof {
		value = align
	}

	c.exprValue = &ir.Integer{
		Typ:   ir.I32,
		Value: utils.Unsigned(false, uint64(value)),
	}
}

func (c *codegen) VisitIndex(i *ast.Index) {
	ptr := c.visit(i.Value)
	index := c.visitLoad(i.Index)

	typ := c.types.Get(i.Value.Result().Type)

	if _, ok := i.Value.Result().Type.(*ast.PointerType); ok {
		ptr = c.load(i.Value, ptr)
		c.exprValue = c.emitter.GetElementPtrDyn(typ, ptr, index, nil)
	} else {
		zero := &ir.Integer{Typ: ir.I32, Value: utils.Signed(0)}
		c.exprValue = c.emitter.GetElementPtrDyn(typ, ptr, zero, index)
	}
}

func (c *codegen) VisitMember(m *ast.Member) {
	// Enum case
	if i, ok := m.Value.(*ast.Identifier); ok {
		if decl, ok := i.Resolved.(*ast.Enum); ok {
			c.exprValue = &ir.Integer{
				Typ:   c.types.Get(decl.ActualType.(*ast.PrimitiveType)),
				Value: m.Resolved.(*ast.EnumCase).ActualValue,
			}

			return
		}
	}

	// Member
	var type_ *ast.DeclType
	isPtr := false

	if p, ok := m.Value.Result().Type.(*ast.PointerType); ok {
		type_ = p.Pointee.(*ast.DeclType)
		isPtr = true
	} else {
		type_ = m.Value.Result().Type.(*ast.DeclType)
	}

	// Method
	if f, ok := m.Result().Type.(*ast.Func); ok {
		if in, ok := f.Parent().(*ast.Interface); ok {
			// Dynamic dispatch
			v := c.visitLoad(m.Value)

			typ := &ir.ArrayType{
				Length:  1 + uint32(len(in.Methods)),
				Element: ir.Pointer,
			}

			index := uint32(slices.Index(in.Methods, f))

			vtablePtr := c.emitter.ExtractValue(v, 1)
			funPtrOffset := c.emitter.GetElementPtrConst(typ, vtablePtr, 0, 1+index)
			c.exprValue = c.emitter.Load(ir.Pointer, funPtrOffset)
		} else {
			// Static dispatch
			c.exprValue = c.getValueForFunc(f)
		}

		return
	}

	// Field
	field, i := type_.Decl.(*ast.Struct).GetField(m.Name.Token.Text)

	if m.Value.Result().Kind == ast.Value {
		value := c.visitLoad(m.Value)

		if isPtr {
			value = c.emitter.GetElementPtrConst(c.types.Get(type_), value, 0, uint32(i))
			c.exprValue = c.emitter.Load(c.types.Get(field.Type), value)
		} else {
			c.exprValue = c.emitter.ExtractValue(value, uint32(i))
		}
	} else {
		value := c.visit(m.Value)

		if isPtr {
			value = c.emitter.Load(ir.Pointer, value)
		}

		c.exprValue = c.emitter.GetElementPtrConst(c.types.Get(type_), value, 0, uint32(i))
	}
}

func (c *codegen) VisitUnary(u *ast.Unary) {
	if u.Postfix {
		switch u.Op {
		// ++, --
		case lexer.PlusPlus, lexer.MinusMinus:
			ptr := c.visit(u.Expr)

			oldValue := c.emitter.Load(c.types.Get(u.Expr.Result().Type), ptr)
			newValue := c.binarySimple(u.Op, oldValue, c.getConstantOne(u.Expr.Result().Type), u.Result().Type)

			c.emitter.Store(newValue, ptr)

			c.exprValue = oldValue

		default:
			panic("codegen.codegen.VisitUnary() - Invalid postfix operator")
		}
	} else {
		switch u.Op {
		// ++, --
		case lexer.PlusPlus, lexer.MinusMinus:
			ptr := c.visit(u.Expr)

			oldValue := c.emitter.Load(c.types.Get(u.Expr.Result().Type), ptr)
			newValue := c.binarySimple(u.Op, oldValue, c.getConstantOne(u.Expr.Result().Type), u.Result().Type)

			c.emitter.Store(newValue, ptr)

			c.exprValue = newValue

		// -
		case lexer.Minus:
			value := c.visitLoad(u.Expr)

			if u.Expr.Result().Type.(*ast.PrimitiveType).Kind.IsFloating() {
				c.exprValue = c.emitter.Fneg(value)
			} else {
				zero := &ir.Integer{Typ: c.types.Get(u.Expr.Result().Type), Value: utils.Signed(0)}
				c.exprValue = c.emitter.Sub(zero, value)
			}

		// !
		case lexer.Bang:
			value := c.visitLoadImplicitCast(u.Expr, ast.BoolType)
			c.exprValue = c.emitter.Xor(value, ir.True)

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

		c.emitter.Store(value, ptr)

	case lexer.PlusEqual, lexer.MinusEqual, lexer.StarEqual, lexer.SlashEqual, lexer.PercentageEqual, lexer.PipeEqual, lexer.XorEqual, lexer.AmpersandEqual:
		ptr := c.visit(b.Left)

		left := c.emitter.Load(c.types.Get(b.Left.Result().Type), ptr)
		right := c.visitLoad(b.Right)

		value := c.binarySimple(b.Op, left, right, b.Result().Type)
		c.emitter.Store(value, ptr)

	// Boolean
	case lexer.PipePipe:
		leftL := c.fun.NewBlock("or.left")
		rightL := c.fun.NewBlock("or.right")
		exitL := c.fun.NewBlock("or.exit")

		c.emitter.Br(leftL)

		// Left
		c.emitter.Begin(leftL)
		left := c.visitLoadImplicitCast(b.Left, ast.BoolType)
		c.emitter.BrCond(left, exitL, rightL)

		// Right
		c.emitter.Begin(rightL)
		right := c.visitLoadImplicitCast(b.Right, ast.BoolType)
		c.emitter.Br(exitL)

		// Exit
		c.emitter.Begin(exitL)

		c.exprValue = c.emitter.Phi(
			ir.PhiPair{Block: leftL, Value: left},
			ir.PhiPair{Block: rightL, Value: right},
		)

	case lexer.AmpersandAmpersand:
		leftL := c.fun.NewBlock("and.left")
		rightL := c.fun.NewBlock("and.right")
		exitL := c.fun.NewBlock("and.exit")

		c.emitter.Br(leftL)

		// Left
		c.emitter.Begin(leftL)
		left := c.visitLoadImplicitCast(b.Left, ast.BoolType)
		c.emitter.BrCond(left, rightL, exitL)

		// Right
		c.emitter.Begin(rightL)
		right := c.visitLoadImplicitCast(b.Right, ast.BoolType)
		c.emitter.Br(exitL)

		// Exit
		c.emitter.Begin(exitL)

		c.exprValue = c.emitter.Phi(
			ir.PhiPair{Block: leftL, Value: ir.False},
			ir.PhiPair{Block: rightL, Value: right},
		)

	// Equality
	case lexer.EqualEqual, lexer.BangEqual:
		left := c.visitLoad(b.Left)
		right := c.visitLoad(b.Right)

		switch type_ := b.Left.Result().Type.(type) {
		case *ast.PrimitiveType:
			kind := type_.Kind
			op := utils.Ternary(b.Op == lexer.BangEqual, ir.Ne, ir.Eq)

			if kind.IsFloating() {
				c.exprValue = c.emitter.FCmp(op, true, left, right)
			} else {
				c.exprValue = c.emitter.ICmp(op, kind.IsSignedInteger(), left, right)
			}

		case *ast.PointerType:
			op := utils.Ternary(b.Op == lexer.BangEqual, ir.Ne, ir.Eq)
			c.exprValue = c.emitter.ICmp(op, false, left, right)

		case *ast.DeclType:
			switch decl := type_.Decl.(type) {
			case *ast.Enum:
				op := utils.Ternary(b.Op == lexer.BangEqual, ir.Ne, ir.Eq)
				signed := decl.ActualType.(*ast.PrimitiveType).Kind.IsSignedInteger()

				c.exprValue = c.emitter.ICmp(op, signed, left, right)

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

		var op ir.CmpOp

		//goland:noinspection GoSwitchMissingCasesForIotaConsts
		switch b.Op {
		case lexer.Less:
			op = ir.Lt
		case lexer.LessEqual:
			op = ir.Le
		case lexer.Greater:
			op = ir.Gt
		case lexer.GreaterEqual:
			op = ir.Ge
		}

		if kind.IsFloating() {
			c.exprValue = c.emitter.FCmp(op, true, left, right)
		} else {
			c.exprValue = c.emitter.ICmp(op, kind.IsSignedInteger(), left, right)
		}

	// Logical, Math
	default:
		c.exprValue = c.binarySimple(b.Op, c.visitLoad(b.Left), c.visitLoad(b.Right), b.Result().Type)
	}
}

func (c *codegen) binarySimple(op lexer.TokenKind, left, right ir.Value, type_ ast.Type) ir.Value {
	switch op {
	// Logical
	case lexer.Pipe, lexer.PipeEqual:
		return c.emitter.Or(left, right)
	case lexer.Xor, lexer.XorEqual:
		return c.emitter.Xor(left, right)
	case lexer.Ampersand, lexer.AmpersandEqual:
		return c.emitter.And(left, right)

	// Math
	case lexer.Plus, lexer.PlusEqual, lexer.PlusPlus:
		return c.emitter.Add(left, right)
	case lexer.Minus, lexer.MinusEqual, lexer.MinusMinus:
		return c.emitter.Sub(left, right)
	case lexer.Star, lexer.StarEqual:
		return c.emitter.Mul(left, right)
	case lexer.Slash, lexer.SlashEqual:
		return c.emitter.Div(getIrDivKind(type_), left, right)
	case lexer.Percentage, lexer.PercentageEqual:
		return c.emitter.Rem(getIrDivKind(type_), left, right)

	// Invalid
	default:
		panic("codegen.codegen.binarySimple() - Invalid operator")
	}
}

func (c *codegen) VisitIs(i *ast.Is) {
	value := c.visitLoad(i.Value)

	valueVTable := c.emitter.ExtractValue(value, 1)
	valueTypeInfo := c.emitter.Load(ir.Pointer, valueVTable)

	typeTypeInfo := c.getTypeInfoPointer(i.Type.(*ast.PointerType).Pointee.(*ast.DeclType).Decl)

	c.exprValue = c.emitter.ICmp(ir.Eq, false, valueTypeInfo, typeTypeInfo)
}

func (c *codegen) VisitCast(cast *ast.Cast) {
	value := c.visitLoad(cast.Value)

	from := cast.Value.Result().Type
	to := cast.Type

	kind, ok := analyzer.GetCastKind(nil, from, to, false)
	if !ok {
		panic("codegen.codegen.VisitCast() - Invalid cast kind")
	}

	c.exprValue = c.cast(value, from, to, kind)
}

func getIrDivKind(type_ ast.Type) ir.DivKind {
	kind := type_.(*ast.PrimitiveType).Kind

	if kind.IsFloating() {
		return ir.Floating
	}
	if kind.IsSignedInteger() {
		return ir.Signed
	}

	return ir.Unsigned
}

func (c *codegen) cast(value ir.Value, from, to ast.Type, kind analyzer.CastKind) ir.Value {
	typ := c.types.Get(to)

	if declType, ok := from.(*ast.DeclType); ok {
		if enum, ok := declType.Decl.(*ast.Enum); ok {
			from = enum.ActualType
		}
	}

	switch kind {
	case analyzer.Nop:
		return value
	case analyzer.Extend:
		return c.emitter.Ext(getIrDivKind(from), value, typ)
	case analyzer.Truncate:
		return c.emitter.Trunc(value, typ)
	case analyzer.IntegerToFloating:
		return c.emitter.IntToFp(from.(*ast.PrimitiveType).Kind.IsSignedInteger(), value, typ)
	case analyzer.FloatingToInteger:
		return c.emitter.FpToInt(to.(*ast.PrimitiveType).Kind.IsSignedInteger(), value, typ)
	case analyzer.IntegerToPointer:
		return c.emitter.IntToPtr(value, typ)
	case analyzer.PointerToInteger:
		return c.emitter.PtrToInt(value, typ)

	case analyzer.PointerToInterface:
		decl := from.(*ast.PointerType).Pointee.(*ast.DeclType).Decl
		in := to.(*ast.DeclType).Decl.(*ast.Interface)

		v := &ir.Struct{
			Typ: typ,
			Fields: []ir.Value{
				&ir.Null{},
				c.getVTablePointer(decl, in),
			},
		}

		return c.emitter.InsertValue(v, value, 0)

	case analyzer.InterfaceToPointer:
		return c.emitter.ExtractValue(value, 0)

	default:
		panic("codegen.codegen.cast() - Invalid cast kind")
	}
}

// Utils

func (c *codegen) emitAlloca(name string, typ ir.Type) ir.Value {
	in := &ir.Alloca{
		Typ:   typ,
		Count: 1,
	}

	in.SetName(name)
	in.SetMeta(c.emitter.GetLocMetaRef())

	c.fun.Blocks[0].AddFirst(in)
	return in
}

func (c *codegen) getTypeInfoPointer(decl ast.Decl) ir.Value {
	typeInfoName := GetTypeInfoLinkName(decl)

	for gVar := range c.module.GlobalVars() {
		if gVar.Name == typeInfoName {
			return gVar
		}
	}

	gVar := c.module.NewGlobalVar(typeInfoName, ir.I8)
	gVar.Flags = ir.External | ir.Constant

	return gVar
}

func (c *codegen) getVTablePointer(decl ast.Decl, in *ast.Interface) ir.Value {
	vtableName := GetVTableLinkName(decl, in)

	for gVar := range c.module.GlobalVars() {
		if gVar.Name == vtableName {
			return gVar
		}
	}

	typ := &ir.ArrayType{
		Length:  1 + uint32(len(in.Methods)),
		Element: ir.Pointer,
	}

	gVar := c.module.NewGlobalVar(vtableName, typ)
	gVar.Flags = ir.External | ir.Constant

	return gVar
}

func (c *codegen) emitDbgDeclare(name string, type_ ast.Type, ptr ir.Value, arg uint32, node ast.Node) {
	c.emitter.DbgDeclare(
		ptr,
		c.module.AddMeta(&ir.LocalVariableMeta{
			Name:  name,
			Type:  c.types.GetMeta(type_),
			Arg:   arg,
			Scope: c.emitter.PeekScope(),
			File:  c.fileRef,
			Line:  getClosestValidRange(node).Start.Line,
		}),
		c.emitter.GetLocMetaRef(),
	)
}

func (c *codegen) getValueForGlobalVar(g *ast.GlobalVar) ir.Value {
	if v, ok := c.globalVars[g]; ok {
		return v
	} else {
		gVar := c.module.NewGlobalVar(GetGlobalVarLinkName(g), c.types.Get(g.Type))
		gVar.Flags = ir.External

		c.globalVars[g] = gVar
		return gVar
	}
}

func (c *codegen) getValueForFunc(f *ast.Func) ir.Value {
	if v, ok := c.functions[f]; ok {
		return v
	} else {
		fun := c.newFunction(f)
		fun.Flags |= ir.Declare

		return fun
	}
}

func (c *codegen) getConstantOne(type_ ast.Type) ir.Value {
	kind := type_.(*ast.PrimitiveType).Kind

	if kind == ast.F32 {
		return &ir.FloatV{Value: 1}
	}
	if kind == ast.F64 {
		return &ir.DoubleV{Value: 1}
	}

	return &ir.Integer{
		Typ:   c.types.Get(type_),
		Value: utils.Signed(1),
	}
}

func (c *codegen) load(expr ast.Expr, value ir.Value) ir.Value {
	if expr.Result().Kind == ast.Address {
		if _, ok := value.(*ir.Function); !ok {
			value = c.emitter.Load(c.types.Get(expr.Result().Type), value)
		}
	}

	return value
}

func (c *codegen) implicitCast(value ir.Value, from, to ast.Type) ir.Value {
	if ast.IsValid(from) && ast.IsValid(to) {
		if kind, ok := analyzer.GetImplicitCastKind(nil, from, to); ok {
			value = c.cast(value, from, to, kind)
		}
	}

	return value
}

func (c *codegen) visitLoadImplicitCast(expr ast.Expr, to ast.Type) ir.Value {
	value := c.visitLoad(expr)
	value = c.implicitCast(value, expr.Result().Type, to)

	return value
}

func (c *codegen) visitLoad(expr ast.Expr) ir.Value {
	value := c.visit(expr)
	return c.load(expr, value)
}

func (c *codegen) visit(expr ast.Expr) ir.Value {
	c.emitter.SetDebugLocation(getClosestValidRange(expr).Start)

	if expr.Result().Kind == ast.Invalid {
		panic("codegen.codegen.Visit() - Expression result is invalid.")
	}

	c.exprValue = nil
	expr.Visit(c)

	return c.exprValue
}
