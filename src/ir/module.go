package ir

import (
	"fireball/core"
	"iter"
	"maps"
	"slices"
)

type Module struct {
	Path       string
	DataLayout string
	Triple     string

	namedStructs map[string]*RefStructType

	globalVars       []*GlobalVar
	globalVarNameMap map[string]*GlobalVar

	functions       []*Function
	functionNameMap map[string]*Function

	namedMetaRefs map[string][]MetaRef

	metaNodeRefMap map[MetaRef]MetaNode
	headMetaNode   MetaNode
	tailMetaNode   MetaNode
	metaNodeCount  uint32

	headSummary  Summary
	tailSummary  Summary
	summaryCount uint32
}

func NewModule() *Module {
	return &Module{
		namedStructs:     make(map[string]*RefStructType),
		namedMetaRefs:    make(map[string][]MetaRef),
		functionNameMap:  make(map[string]*Function),
		globalVarNameMap: make(map[string]*GlobalVar),
		metaNodeRefMap:   make(map[MetaRef]MetaNode),
	}
}

func (m *Module) IsEmpty() bool {
	return len(m.functions) == 0 && len(m.globalVars) == 0
}

// Named structs

func (m *Module) NamedStruct(name string, s StructType) *RefStructType {
	if m.SymbolExists(name) {
		panic("ir.Module.NamedStruct() - Symbol with the name '" + name + "' already exists")
	}

	ref := &RefStructType{
		Name:   name,
		Struct: s,
	}

	m.namedStructs[name] = ref

	return ref
}

func (m *Module) NamedStructs() iter.Seq2[string, StructType] {
	return func(yield func(string, StructType) bool) {
		for name, ref := range m.namedStructs {
			if !yield(name, ref.Struct) {
				return
			}
		}
	}
}

// Global variables

func (m *Module) NewGlobalVar(name string, typ Type) *GlobalVar {
	if m.SymbolExists(name) {
		panic("ir.Module.NewGlobalVar() - Symbol with the name '" + name + "' already exists")
	}

	gVar := &GlobalVar{
		Module: m,
		Name:   name,
		Typ:    typ,
	}

	m.globalVars = append(m.globalVars, gVar)
	m.globalVarNameMap[name] = gVar

	return gVar
}

func (m *Module) GetGlobalVar(name string) *GlobalVar {
	return m.globalVarNameMap[name]
}

func (m *Module) GlobalVars() iter.Seq[*GlobalVar] {
	return slices.Values(m.globalVars)
}

// Functions

func (m *Module) NewFunction(name string, sig *Signature, params []Param) *Function {
	if m.SymbolExists(name) {
		panic("ir.Module.NewFunction() - Symbol with the name '" + name + "' already exists")
	}

	if len(sig.Params) != len(params) {
		panic("ir.Module.NewFunction() - Count of parameter types and params does not match")
	}

	paramValues := make([]Value, len(sig.Params))

	for i, param := range sig.Params {
		paramValues[i] = &ParamValue{
			Typ:  param,
			Name: params[i].Name,
		}
	}

	fun := &Function{
		Module:      m,
		Name:        name,
		Signature:   sig,
		Params:      params,
		ParamValues: paramValues,
	}

	m.functions = append(m.functions, fun)
	m.functionNameMap[name] = fun

	return fun
}

func (m *Module) GetFunction(name string) *Function {
	return m.functionNameMap[name]
}

func (m *Module) Functions() iter.Seq[*Function] {
	return slices.Values(m.functions)
}

// Named meta nodes

func (m *Module) AddNamedMetaRefs(name string, refs ...MetaRef) {
	if _, ok := m.namedMetaRefs[name]; ok {
		panic("ir.Module.AddNamedMetaRefs() - Meta node with name '" + name + "' already exists")
	}

	m.namedMetaRefs[name] = refs
}

func (m *Module) NamedMetaRefs() iter.Seq2[string, []MetaRef] {
	return maps.All(m.namedMetaRefs)
}

// Meta nodes

func (m *Module) AddMeta(node MetaNode) MetaRef {
	if core.IsNil(m.tailMetaNode) {
		m.headMetaNode = node
	} else {
		m.tailMetaNode.setNext(node)
	}

	m.tailMetaNode = node

	ref := MetaRef(m.metaNodeCount + 1)
	m.metaNodeCount++

	m.metaNodeRefMap[ref] = node

	return ref
}

func (m *Module) GetMeta(ref MetaRef) MetaNode {
	if node, ok := m.metaNodeRefMap[ref]; ok {
		return node
	}

	panic("ir.Module.GetMeta() - Invalid meta reference")
}

func (m *Module) MetaNodes() iter.Seq[MetaNode] {
	return iterLinkedList(m.headMetaNode)
}

// Summaries

func (m *Module) AddSummary(summary Summary) SummaryRef {
	if core.IsNil(m.tailSummary) {
		m.headSummary = summary
	} else {
		m.tailSummary.setNext(summary)
	}

	m.tailSummary = summary

	ref := SummaryRef(m.summaryCount + 1)
	m.summaryCount++

	return ref
}

func (m *Module) GetSummary(ref SummaryRef) Summary {
	i := uint32(0)

	for summary := range m.Summaries() {
		if i == ref.Value() {
			return summary
		}

		i++
	}

	panic("ir.Module.GetSummary() - Invalid summary reference")
}

func (m *Module) Summaries() iter.Seq2[Summary, SummaryRef] {
	return func(yield func(Summary, SummaryRef) bool) {
		node := m.headSummary
		i := uint64(1)

		for !core.IsNil(node) {
			if !yield(node, SummaryRef(i)) {
				return
			}

			node = node.next()
			i++
		}
	}
}

// Utils

func (m *Module) SymbolExists(name string) bool {
	if _, ok := m.namedStructs[name]; ok {
		return true
	}

	if _, ok := m.globalVarNameMap[name]; ok {
		return true
	}

	if _, ok := m.functionNameMap[name]; ok {
		return ok
	}

	return false
}
