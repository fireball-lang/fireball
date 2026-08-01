package codegen

import (
	"fireball/ast"
	"fireball/ir"
	"fireball/types"
	"slices"
)

func (c *codegen) AddSummaryCall(ref ir.SummaryRef) {
	call := ir.FunctionSummaryCall{Callee: ref}

	if !slices.Contains(c.summaryCalls, call) {
		c.summaryCalls = append(c.summaryCalls, call)
	}
}

func (c *codegen) AddSummaryRef(ref ir.SummaryRef) {
	if !slices.Contains(c.summaryRefs, ref) {
		c.summaryRefs = append(c.summaryRefs, ref)
	}
}

func (c *codegen) GetGlobalVarRef(g *ast.GlobalVar) ir.SummaryRef {
	name := GlobalVarLinkName(g)
	return c.GetSummaryRef(name, true)
}

func (c *codegen) GetFunctionSummaryRef(f *ast.Func, typ *types.Func, in *types.Interface) ir.SummaryRef {
	name := FuncLinkName(f, typ, in)
	return c.GetSummaryRef(name, false)
}

func (c *codegen) AddSummaryGlobalVar(g *ast.GlobalVar) {
	if !c.moduleSummaryRef.Valid() {
		return
	}

	ref := c.GetGlobalVarRef(g)
	c.AddSummaryRef(ref)
}

func (c *codegen) AddSummaryCallee(f *ast.Func, typ *types.Func, in *types.Interface, call bool) {
	if !c.moduleSummaryRef.Valid() {
		return
	}

	ref := c.GetFunctionSummaryRef(f, typ, in)

	if call {
		c.AddSummaryCall(ref)
	} else {
		c.AddSummaryRef(ref)
	}
}

func (c *codegen) GetSummaryRef(name string, variable bool) ir.SummaryRef {
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

func (c *codegen) CollectSummaryRefs(refs []ir.SummaryRef, value ir.Value) []ir.SummaryRef {
	switch value := value.(type) {
	case *ir.GlobalVar:
		refs = append(refs, c.GetSummaryRef(value.Name, true))
	case *ir.Function:
		refs = append(refs, c.GetSummaryRef(value.Name, false))

	case *ir.Array:
		for _, element := range value.Elements {
			refs = c.CollectSummaryRefs(refs, element)
		}

	case *ir.Struct:
		for _, field := range value.Fields {
			refs = c.CollectSummaryRefs(refs, field)
		}
	}

	return refs
}
