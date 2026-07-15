package lsp

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
)

func (hi *highlighter) VisitBool(_ *ast.Bool) int {
	return 0
}

func (hi *highlighter) VisitNumber(_ *ast.Number) int {
	return 0
}

func (hi *highlighter) VisitCharacter(_ *ast.Character) int {
	return 0
}

func (hi *highlighter) VisitString(_ *ast.String) int {
	return 0
}

func (hi *highlighter) VisitNull(_ *ast.Null) int {
	return 0
}

func (hi *highlighter) VisitStructInitializer(s *ast.StructInitializer) int {
	hi.VisitType(s.Type)

	for _, field := range s.Fields {
		hi.AddFull(field.Name, propertyKind)
		hi.VisitExpr(field.Value)
	}

	return 0
}

func (hi *highlighter) VisitArrayInitializer(a *ast.ArrayInitializer) int {
	hi.VisitType(a.Type)

	for _, element := range a.Elements {
		hi.VisitExpr(element)
	}

	return 0
}

func (hi *highlighter) VisitSizeOf(s *ast.SizeOf) int {
	hi.VisitType(s.Type)

	return 0
}

func (hi *highlighter) VisitAlignOf(a *ast.AlignOf) int {
	hi.VisitType(a.Type)

	return 0
}

func (hi *highlighter) VisitOffsetOf(o *ast.OffsetOf) int {
	hi.VisitType(o.Type)
	hi.AddFull(o.Field, propertyKind)

	return 0
}

func (hi *highlighter) VisitPrefix(p *ast.Prefix) int {
	hi.VisitExpr(p.Expr)

	return 0
}

func (hi *highlighter) VisitPostfix(p *ast.Postfix) int {
	hi.VisitExpr(p.Expr)

	return 0
}

func (hi *highlighter) VisitBinary(b *ast.Binary) int {
	hi.VisitExpr(b.Left)
	hi.VisitExpr(b.Right)

	return 0
}

func (hi *highlighter) VisitIdentifier(i *ast.Identifier) int {
	if len(i.Path.Entries) == 0 {
		return 0
	}

	// Entries before last one
	for _, entry := range i.Path.Entries[:len(i.Path.Entries)-1] {
		if typ, ok := hi.file.NodeTypes[entry]; ok {
			hi.AddType(entry, typ)
		}
	}

	// Last entry
	if info, ok := hi.file.ExprInfos[i]; ok {
		entry := i.Path.Entries[len(i.Path.Entries)-1]

		switch info.Symbol {
		case symbols.Invalid:

		case symbols.Struct:
			hi.Add(entry, classKind)

		case symbols.Enum:
			hi.Add(entry, enumKind)

		case symbols.Interface:
			hi.Add(entry, interfaceKind)

		case symbols.Func:
			hi.Add(entry, functionKind)

		case symbols.TypeParam:
			hi.Add(entry, genericKind)

		case symbols.Case:
			hi.Add(entry, enumMemberKind)

		case symbols.Param:
			kind := parameterKind

			if len(i.Path.Entries) == 1 && entry.Token.Text == "self" {
				if f := ast.GetClosestParent[*ast.Func](i); f != nil && f.Receiver != nil {
					kind = keywordKind
				}
			}

			hi.Add(entry, kind)

		case symbols.Var:
			hi.Add(entry, variableKind)
		}
	}

	return 0
}

func (hi *highlighter) VisitIndex(i *ast.Index) int {
	hi.VisitExpr(i.Expr)
	hi.VisitExpr(i.Index)

	return 0
}

func (hi *highlighter) VisitMember(m *ast.Member) int {
	hi.VisitExpr(m.Expr)

	if info, ok := hi.file.ExprInfos[m]; ok {
		switch info.Node.(type) {
		case *ast.Func:
			hi.AddFull(m.Name, functionKind)
		case *ast.Field:
			hi.AddFull(m.Name, propertyKind)
		}
	}

	return 0
}

func (hi *highlighter) VisitCall(c *ast.Call) int {
	hi.VisitExpr(c.Callee)

	for _, arg := range c.TypeArgs {
		hi.VisitType(arg)
	}

	for _, arg := range c.Args {
		hi.VisitExpr(arg)
	}

	return 0
}

func (hi *highlighter) VisitCast(c *ast.Cast) int {
	hi.VisitExpr(c.Expr)
	hi.VisitType(c.Type)

	return 0
}

func (hi *highlighter) VisitBadExpr(_ *ast.BadExpr) int {
	return 0
}

// Utils

func (hi *highlighter) VisitExpr(expr ast.Expr) {
	if !core.IsNil(expr) {
		ast.VisitExpr(hi, expr)
	}
}
