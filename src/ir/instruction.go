package ir

type Instruction interface {
	RuntimeValue

	Name() string
	SetName(name string)

	next() Instruction
	setNext(in Instruction)
}

type baseInstruction struct {
	baseRuntimeValue

	type_ Type
	name  string

	nextIn Instruction
}

func (b *baseInstruction) Name() string {
	return b.name
}

func (b *baseInstruction) SetName(name string) {
	b.name = name
}

func (b *baseInstruction) next() Instruction {
	return b.nextIn
}

func (b *baseInstruction) setNext(i Instruction) {
	b.nextIn = i
}

type baseVoidInstruction struct {
	baseInstruction
}

func (b *baseVoidInstruction) Type() Type {
	return Void
}

// Terminator instructions

type Ret struct {
	baseVoidInstruction

	Value Value
}

type Br struct {
	baseVoidInstruction

	Label *Block
}

type BrCond struct {
	baseVoidInstruction

	Condition Value
	IfTrue    *Block
	IfFalse   *Block
}

// Unary instructions

type FNeg struct {
	baseInstruction

	Value Value
}

func (f *FNeg) Type() Type {
	return f.Value.Type()
}

// Binary instructions

type DivKind uint8

const (
	Unsigned DivKind = iota
	Signed
	Floating
)

type Add struct {
	baseInstruction

	Left  Value
	Right Value
}

func (a *Add) Type() Type {
	return a.Left.Type()
}

type Sub struct {
	baseInstruction

	Left  Value
	Right Value
}

func (s *Sub) Type() Type {
	return s.Left.Type()
}

type Mul struct {
	baseInstruction

	Left  Value
	Right Value
}

func (m *Mul) Type() Type {
	return m.Left.Type()
}

type Div struct {
	baseInstruction

	Kind  DivKind
	Left  Value
	Right Value
}

func (d *Div) Type() Type {
	return d.Left.Type()
}

type Rem struct {
	baseInstruction

	Kind  DivKind
	Left  Value
	Right Value
}

func (r *Rem) Type() Type {
	return r.Left.Type()
}

// Bitwise binary instructions

type Shl struct {
	baseInstruction

	Left  Value
	Right Value
}

func (s *Shl) Type() Type {
	return s.Left.Type()
}

type Shr struct {
	baseInstruction

	SignExt bool
	Left    Value
	Right   Value
}

func (s *Shr) Type() Type {
	return s.Left.Type()
}

type And struct {
	baseInstruction

	Left  Value
	Right Value
}

func (a *And) Type() Type {
	return a.Left.Type()
}

type Or struct {
	baseInstruction

	Left  Value
	Right Value
}

func (o *Or) Type() Type {
	return o.Left.Type()
}

type Xor struct {
	baseInstruction

	Left  Value
	Right Value
}

func (x *Xor) Type() Type {
	return x.Left.Type()
}

// Vector instructions

type ExtractElement struct {
	baseInstruction

	Value Value
	Index Value
}

func (e *ExtractElement) Type() Type {
	return e.Value.Type().(*VectorType).Element
}

type InsertElement struct {
	baseInstruction

	Value   Value
	Element Value
	Index   Value
}

func (i *InsertElement) Type() Type {
	return i.Value.Type()
}

type ShuffleVector struct {
	baseInstruction

	Value1 Value
	Value2 Value
	Mask   Value
}

func (s *ShuffleVector) Type() Type {
	return &VectorType{
		Length:  s.Mask.Type().(*VectorType).Length,
		Element: s.Value1.Type().(*VectorType).Element,
	}
}

// Aggregate instructions

type ExtractValue struct {
	baseInstruction

	Value   Value
	Indices []uint32
}

func (e *ExtractValue) Type() Type {
	typ := e.Value.Type()

	for _, index := range e.Indices {
		switch type_ := typ.(type) {
		case *ArrayType:
			typ = type_.Element
		case *StructType:
			typ = type_.Fields[index].Type
		case *RefStructType:
			typ = type_.Struct.Fields[index].Type

		default:
			panic("ir.ExtractValue.Type() - Invalid aggregate value type")
		}
	}

	return typ
}

type InsertValue struct {
	baseInstruction

	Value   Value
	Element Value
	Indices []uint32
}

func (i *InsertValue) Type() Type {
	return i.Value.Type()
}

// Memory access and addressing instructions

type Alloca struct {
	baseInstruction

	Typ   Type
	Count uint32
}

func (a *Alloca) Type() Type {
	return Pointer
}

type Load struct {
	baseInstruction

	Typ     Type
	Pointer Value
}

func (l *Load) Type() Type {
	return l.Typ
}

type Store struct {
	baseVoidInstruction

	Value   Value
	Pointer Value
}

type GetElementPtrConst struct {
	baseInstruction

	Typ     Type
	Pointer Value

	Indices [4]uint32
}

func (g *GetElementPtrConst) Type() Type {
	return Pointer
}

type GetElementPtrDyn struct {
	baseInstruction

	Typ     Type
	Pointer Value

	Indices [4]Value
}

func (g *GetElementPtrDyn) Type() Type {
	return Pointer
}

// Conversion instructions

type Trunc struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (t *Trunc) Type() Type {
	return t.Typ
}

type Ext struct {
	baseInstruction

	Kind  DivKind
	Value Value
	Typ   Type
}

func (e *Ext) Type() Type {
	return e.Typ
}

type FpToInt struct {
	baseInstruction

	Signed bool
	Value  Value
	Typ    Type
}

func (f *FpToInt) Type() Type {
	return f.Typ
}

type IntToFp struct {
	baseInstruction

	Signed bool
	Value  Value
	Typ    Type
}

func (i *IntToFp) Type() Type {
	return i.Typ
}

type PtrToInt struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (p *PtrToInt) Type() Type {
	return p.Typ
}

type IntToPtr struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (i *IntToPtr) Type() Type {
	return i.Typ
}

type BitCast struct {
	baseInstruction

	Value Value
	Typ   Type
}

func (b *BitCast) Type() Type {
	return b.Typ
}

// Other instructions

type CmpOp uint8

const (
	Eq CmpOp = iota
	Ne
	Gt
	Ge
	Lt
	Le
)

type ICmp struct {
	baseInstruction

	Op     CmpOp
	Signed bool
	Left   Value
	Right  Value
}

func (i *ICmp) Type() Type {
	return I1
}

type FCmp struct {
	baseInstruction

	Op      CmpOp
	Ordered bool
	Left    Value
	Right   Value
}

func (f *FCmp) Type() Type {
	return I1
}

type PhiPair struct {
	Block *Block
	Value Value
}

type Phi struct {
	baseInstruction

	Pairs []PhiPair
}

func (p *Phi) Type() Type {
	return p.Pairs[0].Value.Type()
}

type Select struct {
	baseInstruction

	Condition Value
	IfTrue    Value
	IfFalse   Value
}

func (s *Select) Type() Type {
	return s.IfTrue.Type()
}

type Call struct {
	baseInstruction

	Signature *Signature
	Callee    Value
	Args      []Value
}

func (c *Call) Type() Type {
	return c.Signature.Returns
}

// Debug instructions

type DbgDeclare struct {
	baseVoidInstruction

	Pointer     Value
	VariableRef MetaRef
	LocationRef MetaRef
}
