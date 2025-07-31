package codegen

import (
	"fireball/ast"
	"fireball/ir"
	"slices"
)

func (c *codegen) addSummaryCall(ref ir.SummaryRef) {
	call := ir.FunctionSummaryCall{Callee: ref}

	if !slices.Contains(c.summaryCalls, call) {
		c.summaryCalls = append(c.summaryCalls, call)
	}
}

func (c *codegen) addSummaryRef(ref ir.SummaryRef) {
	if !slices.Contains(c.summaryRefs, ref) {
		c.summaryRefs = append(c.summaryRefs, ref)
	}
}

func (c *codegen) getFunctionSummaryRef(f *ast.Func) ir.SummaryRef {
	name := GetFuncLinkName(f)
	return c.getSummaryRef(name, false)
}

func (c *codegen) getGlobalVarSummaryRef(g *ast.GlobalVar) ir.SummaryRef {
	name := GetGlobalVarLinkName(g)
	return c.getSummaryRef(name, true)
}

func (c *codegen) getVTableSummaryRef(decl ast.Decl, in *ast.Interface) ir.SummaryRef {
	name := GetVTableLinkName(decl, in)
	return c.getSummaryRef(name, true)
}

func (c *codegen) getTypeInfoSummaryRef(decl ast.Decl) ir.SummaryRef {
	name := GetTypeInfoLinkName(decl)
	return c.getSummaryRef(name, true)
}

func (c *codegen) addSummaryCallee(node ast.Node, f *ast.Func) {
	if !c.moduleSummaryRef.Valid() {
		return
	}

	if _, ok := f.Parent().(*ast.Interface); ok {
		return
	}

	ref := c.getFunctionSummaryRef(f)

	if call, ok := ast.ParentSkipParens(node).(*ast.Call); ok && call.Callee == node {
		c.addSummaryCall(ref)
	} else {
		c.addSummaryRef(ref)
	}
}

func (c *codegen) getSummaryRef(name string, variable bool) ir.SummaryRef {
	// Check existing summaries
	for summary, ref := range c.module.Summaries() {
		switch summary := summary.(type) {
		case *ir.SymbolSummary:
			if summary.Name == name {
				return ref
			}

		case *ir.FunctionSummary:
			if !variable && summary.Name == name {
				return ref
			}

		case *ir.VariableSummary:
			if variable && summary.Name == name {
				return ref
			}
		}
	}

	// Create symbol summary
	ref := c.module.AddSummary(&ir.SymbolSummary{
		Name: name,
	})

	return ref
}
