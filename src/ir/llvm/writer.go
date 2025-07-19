package llvm

import (
	"bufio"
	"fireball/ir"
	"fireball/utils"
	"io"
	"strconv"
)

type writer struct {
	module *ir.Module
	out    *bufio.Writer

	blockNameMap map[*ir.Block]string
	valueNameMap map[ir.Value]string

	instructionIndex uint32
	instructionIds   map[ir.Instruction]uint32

	metaHasField bool

	err error
}

func Write(module *ir.Module, out io.Writer) error {
	w := &writer{
		module: module,
		out:    bufio.NewWriter(out),
	}

	w.string("source_filename = \"")
	w.string(module.Path)
	w.string("\"\n")

	w.string("target datalayout = \"")
	w.string(module.DataLayout)
	w.string("\"\n")

	w.string("target triple = \"")
	w.string(module.Triple)
	w.string("\"\n\n")

	w.namedStructs()
	if w.err != nil {
		return w.err
	}

	w.globalVars()
	if w.err != nil {
		return w.err
	}

	w.functions()
	if w.err != nil {
		return w.err
	}

	w.namedMetaNodes()
	if w.err != nil {
		return w.err
	}

	w.metaNodes()
	if w.err != nil {
		return w.err
	}

	return w.out.Flush()
}

func (w *writer) namedStructs() {
	hasSome := false

	for name, st := range w.module.NamedStructs() {
		w.rune('%')
		w.identifier(name)
		w.string(" = type ")
		w.typ(&st)
		w.rune('\n')

		hasSome = true
	}

	if hasSome {
		w.rune('\n')
	}
}

func (w *writer) globalVars() {
	seen := make(map[*ir.GlobalVar]any)
	hasSome := false

	for gVar := range w.module.GlobalVars() {
		if _, ok := seen[gVar]; !ok && gVar.Flags&ir.Constant != 0 {
			w.globalVar(gVar)
			seen[gVar] = nil
			hasSome = true
		}
	}

	if hasSome {
		w.rune('\n')
	}
	hasSome = false

	for gVar := range w.module.GlobalVars() {
		if _, ok := seen[gVar]; !ok && gVar.Flags&ir.External == 0 {
			w.globalVar(gVar)
			seen[gVar] = nil
			hasSome = true
		}
	}

	if hasSome {
		w.rune('\n')
	}
	hasSome = false

	for gVar := range w.module.GlobalVars() {
		if _, ok := seen[gVar]; !ok {
			w.globalVar(gVar)
			seen[gVar] = nil
			hasSome = true
		}
	}

	if hasSome {
		w.rune('\n')
	}
	hasSome = false
}

func (w *writer) globalVar(gVar *ir.GlobalVar) {
	w.rune('@')
	w.identifier(gVar.Name)
	w.string(" = ")

	if gVar.Flags&ir.Private != 0 {
		w.string("private ")
	} else if gVar.Flags&ir.External != 0 {
		w.string("external ")
	}

	if gVar.Flags&ir.UnnamedAddr != 0 {
		w.string("unnamed_addr ")
	}

	if gVar.Flags&ir.Constant != 0 {
		w.string("constant ")
	} else {
		w.string("global ")
	}

	w.typ(gVar.Typ)

	if !utils.IsNil(gVar.Initializer) {
		w.rune(' ')
		w.value(gVar.Initializer)
	}

	if gVar.Meta().Valid() {
		w.string(", !dbg ")
		w.metaRef(gVar.Meta())
	}

	w.rune('\n')
}

func (w *writer) functions() {
	for fun := range w.module.Functions() {
		if fun.Flags&ir.Declare == 0 {
			w.function(fun)
			w.rune('\n')
		}
	}

	for fun := range w.module.Functions() {
		if fun.Flags&ir.Declare != 0 {
			w.function(fun)
			w.rune('\n')
		}
	}
}

func (w *writer) function(fun *ir.Function) {
	// Header
	if fun.Flags&ir.Declare != 0 {
		w.string("declare ")
	} else {
		w.string("define ")
	}

	if fun.Flags&ir.DsoLocal != 0 {
		w.string("dso_local ")
	}

	w.typ(fun.Typ.Returns)
	w.string(" @")
	w.identifier(fun.Name)
	w.rune('(')

	for i, param := range fun.Typ.Params {
		if i > 0 {
			w.string(", ")
		}

		w.typ(param)

		if len(fun.ParamNames) > 0 {
			w.string(" %")
			w.string(fun.ParamNames[i])
		}
	}

	if fun.Typ.VarArgs {
		if len(fun.Typ.Params) > 0 {
			w.string(", ...")
		} else {
			w.string("...")
		}
	}

	w.rune(')')

	if fun.Meta().Valid() {
		w.string(", !dbg ")
		w.metaRef(fun.Meta())
	}

	// Body
	if len(fun.Blocks) > 0 {
		// Collect duplicate names
		totalNameCounts := make(map[string]int)

		for _, block := range fun.Blocks {
			count := totalNameCounts[block.Name] + 1
			totalNameCounts[block.Name] = count

			for in := range block.Instructions() {
				if in.Name() != "" && !isVoid(in.Type()) {
					count := totalNameCounts[in.Name()] + 1
					totalNameCounts[in.Name()] = count
				}
			}
		}

		// Create name map
		nameCounts := make(map[string]int)

		w.blockNameMap = make(map[*ir.Block]string)
		w.valueNameMap = make(map[ir.Value]string)

		for _, block := range fun.Blocks {
			if totalNameCounts[block.Name] > 1 {
				count := nameCounts[block.Name] + 1
				nameCounts[block.Name] = count
				w.blockNameMap[block] = block.Name + "." + strconv.Itoa(count)
			}

			for in := range block.Instructions() {
				name := in.Name()

				if name != "" && !isVoid(in.Type()) {
					if totalNameCounts[name] > 1 {
						count := nameCounts[name] + 1
						nameCounts[name] = count
						w.valueNameMap[in] = name + "." + strconv.Itoa(count)
					}
				}
			}
		}

		// Instructions
		w.string(" {\n")

		w.instructionIndex = 0
		w.instructionIds = make(map[ir.Instruction]uint32)

		for _, block := range fun.Blocks {
			if name, ok := w.blockNameMap[block]; ok {
				w.identifier(name)
			} else {
				w.identifier(block.Name)
			}
			w.string(":\n")

			for in := range block.Instructions() {
				w.string("  ")

				if in.Name() == "" && !isVoid(in.Type()) {
					w.instructionIds[in] = w.instructionIndex
					w.instructionIndex++
				}

				w.instruction(in)
			}
		}

		w.string("}\n")
	} else {
		w.rune('\n')
	}
}

func (w *writer) namedMetaNodes() {
	hasSome := false

	for name, refs := range w.module.NamedMetaRefs() {
		w.rune('!')
		w.string(name)
		w.string(" = !{")

		for i, ref := range refs {
			if i > 0 {
				w.string(", ")
			}

			w.metaRef(ref)
		}

		w.string("}\n")

		hasSome = true
	}

	if hasSome {
		w.rune('\n')
	}
}

func (w *writer) metaNodes() {
	i := uint64(0)

	for node := range w.module.MetaNodes() {
		w.rune('!')
		w.uint(i, 10)
		w.string(" = ")
		w.meta(node)
		w.rune('\n')

		i++
	}
}

// Utils

func (w *writer) rune(r rune) {
	_, w.err = w.out.WriteRune(r)
}

func (w *writer) string(str string) {
	_, w.err = w.out.WriteString(str)
}

func (w *writer) uint(value uint64, base int) {
	var buffer [16]byte
	bytes := strconv.AppendUint(buffer[0:0], value, base)
	_, w.err = w.out.Write(bytes)
}
