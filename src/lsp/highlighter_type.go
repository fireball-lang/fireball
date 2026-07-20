package lsp

import (
	"fireball/ast"
	"fireball/core"
	"fireball/types"
)

func (hi *highlighter) VisitPrimitiveType(p *ast.PrimitiveType) int {
	hi.AddFull(p, typeKind)

	return 0
}

func (hi *highlighter) VisitArrayType(a *ast.ArrayType) int {
	hi.VisitType(a.Type)

	return 0
}

func (hi *highlighter) VisitPointerType(p *ast.PointerType) int {
	hi.VisitType(p.Pointee)

	return 0
}

func (hi *highlighter) VisitFuncType(f *ast.FuncType) int {
	for _, param := range f.Params {
		hi.AddFull(param.Name, parameterKind)
		hi.VisitType(param.Type)
	}

	hi.VisitType(f.Returns)

	return 0
}

func (hi *highlighter) VisitIdentifierType(i *ast.IdentifierType) int {
	if len(i.Path) > 0 {
		last := i.Path[len(i.Path)-1].Name
		hi.AddType(last, hi.file.NodeTypes[i])
	}

	for _, entry := range i.Path {
		for _, arg := range entry.TypeArgs {
			hi.VisitType(arg)
		}
	}

	return 0
}

func (hi *highlighter) VisitSelfType(s *ast.SelfType) int {
	hi.AddType(s, hi.file.NodeTypes[s])

	return 0
}

func (hi *highlighter) VisitBadType(_ *ast.BadType) int {
	return 0
}

// Utils

func (hi *highlighter) AddType(node ast.Node, typ types.Type) {
	switch typ := typ.(type) {
	case *types.Primitive:
		hi.AddFull(node, typeKind)
	case *types.Array:
		if node, ok := node.(*ast.ArrayType); ok {
			hi.AddType(node.Type, typ.Element)
		}
	case *types.Pointer:
		if node, ok := node.(*ast.PointerType); ok {
			hi.AddType(node.Pointee, typ.Pointee)
		}

	case *types.Struct:
		hi.Add(node, classKind)
	case *types.Enum:
		hi.Add(node, enumKind)
	case *types.Interface:
		hi.Add(node, interfaceKind)
	case *types.Func:
		hi.Add(node, functionKind)
	case *types.Param:
		hi.Add(node, genericKind)
	}
}

func (hi *highlighter) VisitType(t ast.Type) {
	if !core.IsNil(t) {
		ast.VisitType(hi, t)
	}
}
