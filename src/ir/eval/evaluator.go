package eval

import (
	"encoding/binary"
	"errors"
	"fireball/core"
	"fireball/ir"
	"math"
	"unicode/utf8"
)

type Error struct {
	Msg string

	File   string
	Line   uint32
	Column uint32
}

func (e Error) Error() string {
	return e.Msg
}

type Evaluator struct {
	RecursionLimit uint32
	MemoryLimit    uint64

	memory  []uint8
	heapPtr uint64

	globals map[*ir.GlobalVar]uint64
	funcs   map[*ir.Function]uint64

	globalByAddr map[uint64]*ir.GlobalVar
	funcByAddr   map[uint64]*ir.Function

	frames []frame

	prev ir.Instruction
	next ir.Instruction
}

type frame struct {
	heapPtr uint64

	prev ir.Instruction
	next ir.Instruction

	regs map[ir.Value]Register
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		RecursionLimit: 64,
		MemoryLimit:    64 * 4096,

		memory:  make([]uint8, 4096),
		heapPtr: 8,

		globals:      make(map[*ir.GlobalVar]uint64),
		funcs:        make(map[*ir.Function]uint64),
		globalByAddr: make(map[uint64]*ir.GlobalVar),
		funcByAddr:   make(map[uint64]*ir.Function),
	}
}

func (e *Evaluator) allocGlobal(g *ir.GlobalVar) Register {
	size := uint64(g.Typ.Info().Size)
	addr := e.push(size)

	e.globals[g] = addr
	e.globalByAddr[addr] = g

	return Register{Scalar: addr}
}

func (e *Evaluator) writeGlobal(g *ir.GlobalVar) {
	size := uint64(g.Typ.Info().Size)
	addr := e.globals[g]

	data := e.memory[addr : addr+size]
	e.writeIrValue(g.Initializer, data)
}

func (e *Evaluator) LoadGlobal(g *ir.GlobalVar) Register {
	reg := e.allocGlobal(g)

	if !core.IsNil(g.Initializer) {
		e.writeGlobal(g)
	}

	return reg
}

func (e *Evaluator) LoadFunc(f *ir.Function) Register {
	addr := FuncAddrFlag | (uint64(len(e.funcs)+1) * 8)

	e.funcs[f] = addr
	e.funcByAddr[addr] = f

	return Register{Scalar: addr}
}

func (e *Evaluator) Load(m *ir.Module) {
	for f := range m.Functions() {
		e.LoadFunc(f)
	}

	for g := range m.GlobalVars() {
		e.allocGlobal(g)
	}

	for g := range m.GlobalVars() {
		if !core.IsNil(g.Initializer) {
			e.writeGlobal(g)
		}
	}
}

func (e *Evaluator) Run(f *ir.Function, args ...Register) (out Register, outErr error) {
	defer func() {
		err := recover()
		if err == nil {
			return
		}

		if err, ok := err.(error); ok {
			if outErr, ok = errors.AsType[Error](err); ok {
				return
			}
		}

		panic(err)
	}()

	// Setup callstack
	frm := e.call(f)

	for i, value := range f.ParamValues {
		frm.regs[value] = args[i]
	}

	// Instruction loop
	for {
		// Get next instruction
		inst := e.next

		e.prev = e.next
		e.next = inst.Next()

		// Handle 'ret'
		if ret, ok := inst.(*ir.Ret); ok {
			// Get result register
			if !core.IsNil(ret.Value) {
				out = e.getRegister(ret.Value)
			}

			// Pop frame from callstack and return if top level
			if e.ret() {
				return
			}

			// Set result register on the previous instruction in the calling frame (the 'call' instruction)
			if !core.IsNil(ret.Value) {
				e.frames[len(e.frames)-1].regs[e.prev] = out
			}
		}

		// Handle other instructions and their results
		if reg, ok := e.evalInst(inst); ok {
			e.frames[len(e.frames)-1].regs[inst] = reg
		}
	}
}

// Utils

func (e *Evaluator) panic(msg string) {
	err := Error{Msg: msg}

	if e.prev.Meta().Valid() {
		module := e.prev.Block().Func.Module

		var ref ir.MetaRef

		switch meta := module.GetMeta(e.prev.Meta()).(type) {
		case *ir.LocationMeta:
			ref = meta.Scope
			err.Line = meta.Line
			err.Column = meta.Column
		}

		for ref.Valid() {
			switch meta := module.GetMeta(ref).(type) {
			case *ir.FileMeta:
				err.File = meta.Path
				ref = ir.MetaRef(0)

			case *ir.LexicalBlockMeta:
				ref = meta.File

			case *ir.SubprogramMeta:
				ref = meta.File

			default:
				ref = ir.MetaRef(0)
			}
		}
	}

	panic(err)
}

func (e *Evaluator) call(f *ir.Function) *frame {
	if uint32(len(e.frames)) > e.RecursionLimit {
		e.panic("recursion limit exceeded")
	}

	e.frames = append(e.frames, frame{
		heapPtr: e.heapPtr,
		prev:    e.prev,
		next:    e.next,
		regs:    make(map[ir.Value]Register),
	})

	e.next = f.Blocks[0].First()

	return &e.frames[len(e.frames)-1]
}

func (e *Evaluator) ret() bool {
	frm := e.frames[len(e.frames)-1]
	e.frames = e.frames[:len(e.frames)-1]

	e.heapPtr = frm.heapPtr
	e.prev = frm.prev
	e.next = frm.next

	return len(e.frames) == 0
}

func (e *Evaluator) getRegister(val ir.Value) Register {
	// Check current frame's register map
	if reg, ok := e.frames[len(e.frames)-1].regs[val]; ok {
		return reg
	}

	// Convert value to register
	switch val := val.(type) {
	case *ir.Null:
		return Register{}

	case *ir.ZeroInitializer:
		return getZeroInitializedRegister(val.Typ)

	case *ir.Integer:
		return Register{Scalar: val.Value.TwosComplement()}

	case *ir.FloatV:
		return Float32Reg(val.Value)

	case *ir.DoubleV:
		return Float64Reg(val.Value)

	case *ir.Array:
		aggregate := make([]Register, len(val.Elements))

		for i, element := range val.Elements {
			aggregate[i] = e.getRegister(element)
		}

		return Register{Aggregate: aggregate}

	case *ir.Struct:
		aggregate := make([]Register, len(val.Fields))

		for i, field := range val.Fields {
			aggregate[i] = e.getRegister(field)
		}

		return Register{Aggregate: aggregate}

	case *ir.GlobalVar:
		if addr, ok := e.globals[val]; ok {
			return Register{Scalar: addr}
		}

		panic("ir.eval.Evaluator.getRegister() - Invalid global variable: '" + val.Name + "'")

	case *ir.Function:
		if addr, ok := e.funcs[val]; ok {
			return Register{Scalar: addr}
		}

		panic("ir.eval.Evaluator.getRegister() - Invalid function: '" + val.Name + "'")

	default:
		panic("ir.eval.Evaluator.getRegister() - Invalid value")
	}
}

func getZeroInitializedRegister(typ ir.Type) Register {
	switch typ := typ.(type) {
	case *ir.SimpleType, *ir.IntegerType:
		return Register{}

	case *ir.VectorType:
		aggregate := make([]Register, typ.Length)
		zero := getZeroInitializedRegister(typ.Element)

		for i := range typ.Length {
			aggregate[i] = zero
		}

		return Register{Aggregate: aggregate}

	case *ir.ArrayType:
		aggregate := make([]Register, typ.Length)
		zero := getZeroInitializedRegister(typ.Element)

		for i := range typ.Length {
			aggregate[i] = zero
		}

		return Register{Aggregate: aggregate}

	case *ir.StructType:
		aggregate := make([]Register, len(typ.Fields))

		for i, field := range typ.Fields {
			aggregate[i] = getZeroInitializedRegister(field.Type)
		}

		return Register{Aggregate: aggregate}

	case *ir.RefStructType:
		return getZeroInitializedRegister(&typ.Struct)

	default:
		panic("ir.eval.getZeroInitializedRegister() - Invalid type")
	}
}

func (e *Evaluator) writeIrValue(constant ir.Value, data []uint8) {
	size := uint64(constant.Type().Info().Size)

	switch val := constant.(type) {
	case *ir.Null, *ir.ZeroInitializer:
		for i := range size {
			data[i] = 0
		}

	case *ir.Integer:
		var bits uint64

		if val.Value.Negative() {
			bits = ^val.Value.Raw() + 1
		} else {
			bits = val.Value.Raw()
		}

		switch size {
		case 1:
			data[0] = uint8(bits)
		case 2:
			binary.NativeEndian.PutUint16(data, uint16(bits))
		case 4:
			binary.NativeEndian.PutUint32(data, uint32(bits))
		case 8:
			binary.NativeEndian.PutUint64(data, bits)
		default:
			panic("Invalid integer size")
		}

	case *ir.FloatV:
		binary.NativeEndian.PutUint32(data, math.Float32bits(val.Value))

	case *ir.DoubleV:
		binary.NativeEndian.PutUint64(data, math.Float64bits(val.Value))

	case *ir.String:
		i := 0

		for _, ch := range val.Runes {
			i += utf8.EncodeRune(data[i:], ch)
		}

		if val.NullTerminated {
			data[i] = 0
		}

	case *ir.Vector:
		elemSize := uint64(val.Elements[0].Type().Info().Size)

		for i, elem := range val.Elements {
			offset := elemSize * uint64(i)

			e.writeIrValue(elem, data[offset:offset+elemSize])
		}

	case *ir.Array:
		elemSize := uint64(val.Elements[0].Type().Info().Size)

		for i, elem := range val.Elements {
			offset := elemSize * uint64(i)

			e.writeIrValue(elem, data[offset:offset+elemSize])
		}

	case *ir.Struct:
		var s *ir.StructType

		if r, ok := val.Typ.(*ir.RefStructType); ok {
			s = &r.Struct
		} else {
			s = val.Typ.(*ir.StructType)
		}

		for i, r := range ir.GetStructFieldRanges(s) {
			e.writeIrValue(val.Fields[i], data[r.Offset:r.Offset+r.Size])
		}

	case *ir.GlobalVar:
		if addr, ok := e.globals[val]; ok {
			binary.NativeEndian.PutUint64(data, addr)
			return
		}

		panic("ir.eval.Evaluator.writeConstant() - Unknown global variable")

	case *ir.Function:
		if addr, ok := e.funcs[val]; ok {
			binary.NativeEndian.PutUint64(data, addr)
			return
		}

		panic("ir.eval.Evaluator.writeConstant() - Unknown function")

	default:
		panic("ir.eval.Evaluator.writeConstant() - Invalid initializer")
	}
}

func (e *Evaluator) push(size uint64) uint64 {
	if e.heapPtr+size >= uint64(len(e.memory)) {
		newSize := max(uint64(len(e.memory))*2, e.heapPtr+size)

		if newSize > e.MemoryLimit {
			e.panic("memory limit exceeded")
		}

		memory := make([]uint8, newSize)
		copy(memory[:len(e.memory)], e.memory)

		e.memory = memory
	}

	addr := e.heapPtr
	e.heapPtr += size

	return addr
}
