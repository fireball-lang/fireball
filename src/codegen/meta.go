package codegen

import (
	"fireball/ast"
	"fireball/ir"
	"fireball/types"
)

func (c *Codegen) emitDbgDeclare(name string, typ types.Type, ptr ir.Value, arg uint32, node ast.Node) {
	ref := c.Module.AddMeta(&ir.LocalVariableMeta{
		Name:  name,
		Type:  c.Types.GetMeta(typ),
		Arg:   arg,
		Scope: c.Emitter.PeekScope(),
		File:  c.FileRef,
		Line:  node.Range().Start.Line,
	})

	c.Emitter.DbgDeclare(
		ptr,
		ref,
		c.Emitter.GetLocMetaRef(),
	)
}

func (c *Codegen) emitMetaScope(node ast.Node) ir.MetaRef {
	return c.Module.AddMeta(&ir.LexicalBlockMeta{
		Scope:  c.Emitter.PeekScope(),
		File:   c.FileRef,
		Line:   node.Range().Start.Line,
		Column: node.Range().Start.Column,
	})
}

func AddModuleMetaFlags(m *ir.Module, lto bool) {
	flags := m.AddMetas([]ir.MetaNode{
		&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "Dwarf Version"},
			{Number: 4},
		}},
		&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 2},
			{Text: "Debug Info Version"},
			{Number: 3},
		}},
		&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 1},
			{Text: "wchar_size"},
			{Number: 4},
		}},
		&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 8},
			{Text: "PIC Level"},
			{Number: 2},
		}},
		&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "PIE Level"},
			{Number: 2},
		}},
		&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "uwtable"},
			{Number: 2},
		}},
		&ir.RawMeta{Values: []ir.RawMetaValue{
			{Number: 7},
			{Text: "frame-pointer"},
			{Number: 2},
		}},
	})

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
