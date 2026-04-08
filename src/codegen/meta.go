package codegen

import (
	"fireball/ast"
	"fireball/ir"
	"fireball/types"
)

func (c *codegen) emitDbgDeclare(name string, typ types.Type, ptr ir.Value, arg uint32, node ast.Node) {
	ref := c.module.AddMeta(&ir.LocalVariableMeta{
		Name:  name,
		Type:  c.types.GetMeta(typ),
		Arg:   arg,
		Scope: c.emitter.PeekScope(),
		File:  c.fileRef,
		Line:  node.Range().Start.Line,
	})

	c.emitter.DbgDeclare(
		ptr,
		ref,
		c.emitter.GetLocMetaRef(),
	)
}

func (c *codegen) emitMetaScope(node ast.Node) ir.MetaRef {
	return c.module.AddMeta(&ir.LexicalBlockMeta{
		Scope:  c.emitter.PeekScope(),
		File:   c.fileRef,
		Line:   node.Range().Start.Line,
		Column: node.Range().Start.Column,
	})
}

func AddModuleMetaFlags(m *ir.Module, lto bool) {
	flags := []ir.MetaRef{
		m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "Dwarf Version"},
			{Number: 4},
		}}),
		m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 2},
			{Text: "Debug Info Version"},
			{Number: 3},
		}}),
		m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 1},
			{Text: "wchar_size"},
			{Number: 4},
		}}),
		m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 8},
			{Text: "PIC Level"},
			{Number: 2},
		}}),
		m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "PIE Level"},
			{Number: 2},
		}}),
		m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "uwtable"},
			{Number: 2},
		}}),
		m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "frame-pointer"},
			{Number: 2},
		}}),
	}

	if lto {
		flags = append(
			flags,
			m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
				{Number: 1},
				{Text: "EnableSplitLTOUnit"},
				{Number: 1},
			}}),
			m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
				{Number: 1},
				{Text: "UnifiedLTO"},
				{Number: 1},
			}}),
		)
	}

	m.AddNamedMetaRefs("llvm.module.flags", flags...)

	m.AddNamedMetaRefs(
		"llvm.ident",
		m.AddMeta(&ir.RawMeta{Values: []ir.RawMetaValue{
			{Text: "fireball"},
		}}),
	)
}
