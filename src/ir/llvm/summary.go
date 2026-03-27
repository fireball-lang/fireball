package llvm

import "fireball/ir"

func (w *writer) summary(summary ir.Summary) {
	switch summary := summary.(type) {
	case *ir.ModuleSummary:
		w.beginSummary("module")
		w.fieldString("path", summary.Path)
		w.fieldSliceUint("hash", summary.Hash[:])
		w.rune(')')

	case *ir.SymbolSummary:
		w.beginSummary("gv")
		w.fieldString("name", summary.Name)
		w.rune(')')

	case *ir.FunctionSummary:
		w.beginSummary("gv")
		w.fieldString("name", summary.Name)
		w.string(", ")
		w.beginSummary("summaries")
		w.beginSummary("function")

		w.fieldSummaryRef("module", summary.Module)
		w.fieldLinkSummaryFlags("flags", summary.LinkFlags)
		w.fieldUint("insts", summary.InstructionCount)
		w.fieldFunctionSummaryFlags("funcFlags", summary.Flags)
		w.fieldSliceFunctionSummaryCallee("calls", summary.Calls)
		w.fieldSliceSummaryRef("refs", summary.Refs)

		w.string(")))")

	case *ir.VariableSummary:
		w.beginSummary("gv")
		w.fieldString("name", summary.Name)
		w.string(", ")
		w.beginSummary("summaries")
		w.beginSummary("variable")

		w.fieldSummaryRef("module", summary.Module)
		w.fieldLinkSummaryFlags("flags", summary.LinkFlags)
		w.fieldVariableSummaryFlags("varFlags", summary.Flags)
		w.fieldSliceSummaryRef("refs", summary.Refs)

		w.string(")))")

	case *ir.SimpleSummary:
		w.string(summary.Name)
		w.string(": ")
		w.uint(uint64(summary.Value), 10)

	default:
		panic("llvm.writer.summary() - Invalid summary")
	}
}

// Utils

func (w *writer) beginSummary(name string) {
	w.string(name)
	w.string(": (")

	w.objectHasField = false
}

func (w *writer) fieldBoolInt(name string, value bool) {
	if w.objectHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": ")

	if value {
		w.string("1")
	} else {
		w.string("0")
	}

	w.objectHasField = true
}

func (w *writer) fieldSummaryRef(name string, ref ir.SummaryRef) {
	if w.objectHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": ^")
	w.uint(uint64(ref.Value()), 10)

	w.objectHasField = true
}

func (w *writer) fieldSliceSummaryRef(name string, refs []ir.SummaryRef) {
	if len(refs) == 0 {
		return
	}

	if w.objectHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": (")

	for i, ref := range refs {
		if i > 0 {
			w.string(", ")
		}

		w.rune('^')
		w.uint(uint64(ref.Value()), 10)
	}

	w.rune(')')
	w.objectHasField = true
}

func (w *writer) fieldSliceUint(name string, values []uint32) {
	if w.objectHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": (")

	for i, value := range values {
		if i > 0 {
			w.string(", ")
		}

		w.uint(uint64(value), 10)
	}

	w.rune(')')
	w.objectHasField = true
}

func (w *writer) fieldLinkSummaryFlags(name string, flags ir.LinkSummaryFlags) {
	if w.objectHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": (")
	w.objectHasField = false

	w.fieldRaw("linkage", flags.Linkage.String())
	w.fieldRaw("visibility", flags.Visibility.String())
	w.fieldBoolInt("notEligibleToImport", flags.NotEligibleToImport)
	w.fieldBoolInt("live", flags.Live)
	w.fieldBoolInt("dsoLocal", flags.DsoLocal)
	w.fieldBoolInt("canAutoHide", flags.CanAutoHide)
	w.fieldRaw("importType", flags.ImportType.String())

	w.rune(')')
	w.objectHasField = true
}

func (w *writer) fieldFunctionSummaryFlags(name string, flags ir.FunctionSummaryFlags) {
	if w.objectHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": (")
	w.objectHasField = false

	w.fieldBoolInt("readNone", flags&ir.FuncReadNone != 0)
	w.fieldBoolInt("readOnly", flags&ir.FuncReadOnly != 0)
	w.fieldBoolInt("noRecurse", flags&ir.FuncNoRecurse != 0)
	w.fieldBoolInt("returnDoesNotAlias", flags&ir.FuncReturnDoesNotAlias != 0)
	w.fieldBoolInt("noInline", flags&ir.FuncNoInline != 0)
	w.fieldBoolInt("alwaysInline", flags&ir.FuncAlwaysInline != 0)
	w.fieldBoolInt("noUnwind", flags&ir.FuncNoUnwind != 0)
	w.fieldBoolInt("mayThrow", flags&ir.FuncMayThrow != 0)
	w.fieldBoolInt("hasUnknownCall", flags&ir.FuncHasUnknownCall != 0)
	w.fieldBoolInt("mustBeUnreachable", flags&ir.FuncMustBeUnreachable != 0)

	w.rune(')')
	w.objectHasField = true
}

func (w *writer) fieldSliceFunctionSummaryCallee(name string, calls []ir.FunctionSummaryCall) {
	if len(calls) == 0 {
		return
	}

	if w.objectHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": (")

	for i, call := range calls {
		if i > 0 {
			w.string(", ")
		}

		w.string("(callee: ^")
		w.uint(uint64(call.Callee.Value()), 10)
		w.rune(')')
	}

	w.rune(')')
	w.objectHasField = true
}

func (w *writer) fieldVariableSummaryFlags(name string, flags ir.VariableSummaryFlags) {
	if w.objectHasField {
		w.string(", ")
	}

	w.string(name)
	w.string(": (")
	w.objectHasField = false

	w.fieldBoolInt("readonly", flags&ir.VarReadOnly != 0)
	w.fieldBoolInt("writeonly", flags&ir.VarWriteOnly != 0)
	w.fieldBoolInt("constant", flags&ir.VarConstant != 0)

	w.rune(')')
	w.objectHasField = true
}
