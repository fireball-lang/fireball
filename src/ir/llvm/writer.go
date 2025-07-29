package llvm

import (
	"fireball/ir"
	"fireball/utils"
	"io"
	"strconv"
	"unicode/utf8"
)

type writer struct {
	module *ir.Module

	out    io.Writer
	buffer []byte

	blockNameMap map[*ir.Block]string
	valueNameMap map[ir.Value]string

	instructionIndex uint32
	instructionIds   map[ir.Instruction]uint32

	metaHasField bool
}

func Write(module *ir.Module, out io.Writer) error {
	w := &writer{
		module: module,
		out:    out,
		buffer: make([]byte, 0, 9*1024),
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

	if err := w.namedStructs(); err != nil {
		return err
	}

	if err := w.globalVars(); err != nil {
		return err
	}

	if err := w.functions(); err != nil {
		return err
	}

	if err := w.namedMetaNodes(); err != nil {
		return err
	}

	if err := w.metaNodes(); err != nil {
		return err
	}

	return w.flush()
}

func (w *writer) namedStructs() error {
	hasSome := false

	for name, st := range w.module.NamedStructs() {
		w.rune('%')
		w.identifier(name)
		w.string(" = type ")
		w.typ(&st)
		w.rune('\n')

		if w.needsFlush() {
			if err := w.flush(); err != nil {
				return err
			}
		}

		hasSome = true
	}

	if hasSome {
		w.rune('\n')
	}

	return nil
}

func (w *writer) globalVars() error {
	seen := make(map[*ir.GlobalVar]any)
	hasSome := false

	for gVar := range w.module.GlobalVars() {
		if _, ok := seen[gVar]; !ok && gVar.Flags&ir.Constant != 0 {
			if err := w.globalVar(gVar); err != nil {
				return err
			}

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
			if err := w.globalVar(gVar); err != nil {
				return err
			}

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
			if err := w.globalVar(gVar); err != nil {
				return err
			}

			seen[gVar] = nil
			hasSome = true
		}
	}

	if hasSome {
		w.rune('\n')
	}
	hasSome = false

	return nil
}

func (w *writer) globalVar(gVar *ir.GlobalVar) error {
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

	if w.needsFlush() {
		if err := w.flush(); err != nil {
			return err
		}
	}

	return nil
}

func (w *writer) functions() error {
	for fun := range w.module.Functions() {
		if fun.Flags&ir.Declare == 0 {
			if err := w.function(fun); err != nil {
				return err
			}

			w.rune('\n')
		}
	}

	for fun := range w.module.Functions() {
		if fun.Flags&ir.Declare != 0 {
			if err := w.function(fun); err != nil {
				return err
			}

			w.rune('\n')
		}
	}

	return nil
}

func (w *writer) function(fun *ir.Function) error {
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
		w.string(" !dbg ")
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

		w.instructionIndex = 0
		w.instructionIds = make(map[ir.Instruction]uint32)

		for _, block := range fun.Blocks {
			if totalNameCounts[block.Name] > 1 {
				count := nameCounts[block.Name] + 1
				nameCounts[block.Name] = count
				w.blockNameMap[block] = block.Name + "." + strconv.Itoa(count)
			}

			for in := range block.Instructions() {
				name := in.Name()

				if !isVoid(in.Type()) {
					if name == "" {
						w.instructionIds[in] = w.instructionIndex
						w.instructionIndex++
					} else {
						if totalNameCounts[name] > 1 {
							count := nameCounts[name] + 1
							nameCounts[name] = count
							w.valueNameMap[in] = name + "." + strconv.Itoa(count)
						}
					}
				}
			}
		}

		// Instructions
		w.string(" {\n")

		for _, block := range fun.Blocks {
			if name, ok := w.blockNameMap[block]; ok {
				w.identifier(name)
			} else {
				w.identifier(block.Name)
			}
			w.string(":\n")

			for in := range block.Instructions() {
				w.string("  ")
				w.instruction(in)
			}
		}

		w.string("}\n")
	} else {
		w.rune('\n')
	}

	if w.needsFlush() {
		if err := w.flush(); err != nil {
			return err
		}
	}

	return nil
}

func (w *writer) namedMetaNodes() error {
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

		if w.needsFlush() {
			if err := w.flush(); err != nil {
				return err
			}
		}

		hasSome = true
	}

	if hasSome {
		w.rune('\n')
	}

	return nil
}

func (w *writer) metaNodes() error {
	i := uint64(0)

	for node := range w.module.MetaNodes() {
		w.rune('!')
		w.uint(i, 10)
		w.string(" = ")
		w.meta(node)
		w.rune('\n')

		if w.needsFlush() {
			if err := w.flush(); err != nil {
				return err
			}
		}

		i++
	}

	return nil
}

// Utils

func (w *writer) rune(r rune) {
	w.buffer = utf8.AppendRune(w.buffer, r)
}

func (w *writer) string(str string) {
	w.buffer = append(w.buffer, str...)
}

func (w *writer) uint(value uint64, base int) {
	w.buffer = strconv.AppendUint(w.buffer, value, base)
}

func (w *writer) needsFlush() bool {
	return len(w.buffer) >= 8*1024
}

func (w *writer) flush() error {
	_, err := w.out.Write(w.buffer)
	w.buffer = w.buffer[0:0]

	return err
}
