package ir

import (
	"crypto/md5"
	"encoding/binary"
	"fireball/core"
	"slices"
	"strings"
)

type summaryGen struct {
	m *Module

	gVarSummaryRefs map[*GlobalVar]SummaryRef
	funSummaryRefs  map[*Function]SummaryRef
}

func GenerateSummary(m *Module) {
	defer core.Scope()()

	sg := summaryGen{
		m:               m,
		gVarSummaryRefs: make(map[*GlobalVar]SummaryRef, len(m.globalVars)),
		funSummaryRefs:  make(map[*Function]SummaryRef, len(m.functions)),
	}

	// Create module summary

	mRef := m.AddSummary(&ModuleSummary{
		Path: m.Path,
		Hash: [5]uint32{},
	})

	m.AddSummary(&SimpleSummary{
		Name:  "flags",
		Value: 520,
	})

	m.AddSummary(&SimpleSummary{
		Name:  "blockcount",
		Value: 0,
	})

	// Create global variable / function summaries

	for gVar := range m.GlobalVars() {
		if gVar.Flags&External != 0 {
			continue
		}

		linkage := LinkageExternal
		if gVar.Flags&Private != 0 {
			linkage = LinkagePrivate
		} else if gVar.Flags&LinkOnce != 0 {
			linkage = LinkageLinkOnceODR
		}

		linkFlags := LinkSummaryFlags{
			Linkage:             linkage,
			Visibility:          VisibilityDefault,
			NotEligibleToImport: false,
			Live:                false,
			DsoLocal:            true,
			CanAutoHide:         linkage == LinkageLinkOnceODR,
			ImportType:          ImportDefinition,
		}

		var flags VariableSummaryFlags

		if gVar.Flags&Constant != 0 {
			flags = VarConstant | VarReadOnly
		}

		sg.gVarSummaryRefs[gVar] = m.AddSummary(&VariableSummary{
			Module:    mRef,
			Name:      gVar.Name,
			LinkFlags: linkFlags,
			Flags:     flags,
		})

		gVar.GUIDMeta = m.AddMeta(&GuidMeta{GUID: MD5Guid(gVar.Name)})
	}

	for fun := range m.Functions() {
		if fun.Flags&Declare != 0 {
			continue
		}

		fb := strings.HasPrefix(fun.Name, "fb$")

		linkFlags := LinkSummaryFlags{
			Linkage:             LinkageExternal,
			Visibility:          VisibilityDefault,
			NotEligibleToImport: false,
			Live:                !fb,
			DsoLocal:            true,
			CanAutoHide:         fun.Flags&LinkOnceODR != 0,
			ImportType:          ImportDefinition,
		}

		flags := FuncNoUnwind

		if fun.Flags&LinkOnceODR != 0 {
			linkFlags.Linkage = LinkageLinkOnceODR
		}

		sg.funSummaryRefs[fun] = m.AddSummary(&FunctionSummary{
			Module:    mRef,
			Name:      fun.Name,
			LinkFlags: linkFlags,
			Flags:     flags,
		})

		fun.GUIDMeta = m.AddMeta(&GuidMeta{GUID: MD5Guid(fun.Name)})
	}

	// Fill in references

	for gVar := range m.GlobalVars() {
		if gVar.Flags&External != 0 || core.IsNil(gVar.Initializer) {
			continue
		}

		summary := m.GetSummary(sg.gVarSummaryRefs[gVar]).(*VariableSummary)
		summary.Refs = sg.CollectSummaryRefs(gVar.Initializer, nil)
	}

	for fun := range m.Functions() {
		if fun.Flags&Declare != 0 {
			continue
		}

		summary := m.GetSummary(sg.funSummaryRefs[fun]).(*FunctionSummary)

		for _, block := range fun.Blocks {
			summary.InstructionCount += block.InstructionCount

			for inst := range block.Instructions() {
				// Calls
				if call, ok := inst.(*Call); ok {
					if fun, ok := call.Callee.(*Function); ok {
						refCall := FunctionSummaryCall{Callee: sg.GetFunctionRef(fun)}

						if !slices.Contains(summary.Calls, refCall) {
							summary.Calls = append(summary.Calls, refCall)
						}
					} else {
						summary.Flags |= FuncHasUnknownCall
						summary.Refs = sg.CollectSummaryRefs(call.Callee, summary.Refs)
					}

					for _, arg := range call.Args {
						summary.Refs = sg.CollectSummaryRefs(arg, summary.Refs)
					}

					continue
				}

				// Refs
				for value := range inst.Values() {
					summary.Refs = sg.CollectSummaryRefs(value, summary.Refs)
				}
			}
		}
	}
}

func (sg *summaryGen) CollectSummaryRefs(value Value, refs []SummaryRef) []SummaryRef {
	switch value := value.(type) {
	case *GlobalVar:
		refs = sg.AddGlobalVarRef(value, refs)

	case *Function:
		refs = sg.AddFunctionRef(value, refs)

	case *Array:
		for _, element := range value.Elements {
			refs = sg.CollectSummaryRefs(element, refs)
		}

	case *Struct:
		for _, field := range value.Fields {
			refs = sg.CollectSummaryRefs(field, refs)
		}
	}

	return refs
}

func (sg *summaryGen) AddGlobalVarRef(gVar *GlobalVar, refs []SummaryRef) []SummaryRef {
	ref := sg.GetGlobalVarRef(gVar)

	if !slices.Contains(refs, ref) {
		refs = append(refs, ref)
	}

	return refs
}

func (sg *summaryGen) AddFunctionRef(fun *Function, refs []SummaryRef) []SummaryRef {
	ref := sg.GetFunctionRef(fun)

	if !slices.Contains(refs, ref) {
		refs = append(refs, ref)
	}

	return refs
}

func (sg *summaryGen) GetGlobalVarRef(gVar *GlobalVar) SummaryRef {
	if ref, ok := sg.gVarSummaryRefs[gVar]; ok {
		return ref
	}

	ref := sg.m.AddSummary(&SymbolSummary{Name: gVar.Name})
	sg.gVarSummaryRefs[gVar] = ref

	return ref
}

func (sg *summaryGen) GetFunctionRef(fun *Function) SummaryRef {
	if ref, ok := sg.funSummaryRefs[fun]; ok {
		return ref
	}

	ref := sg.m.AddSummary(&SymbolSummary{Name: fun.Name})
	sg.funSummaryRefs[fun] = ref

	return ref
}

func MD5Guid(name string) uint64 {
	sum := md5.Sum([]byte(name))
	return binary.LittleEndian.Uint64(sum[:8])
}
