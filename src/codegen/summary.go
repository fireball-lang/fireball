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

func (c *codegen) GetFunctionSummaryRef(f *ast.Func, typ *types.Func, in *types.Interface) ir.SummaryRef {
	name := FuncLinkName(f, typ, in)
	return c.GetSummaryRef(name, false)
}

func (c *codegen) AddSummaryCallee(node ast.Node, f *ast.Func, typ *types.Func, in *types.Interface) {
	if !c.moduleSummaryRef.Valid() {
		return
	}

	ref := c.GetFunctionSummaryRef(f, typ, in)

	if call, ok := node.Parent().(*ast.Call); ok && call.Callee == node {
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
