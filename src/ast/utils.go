package ast

import (
	"fireball/lexer"
)

func Root(node Node) *File {
	for {
		if !IsValid(node) {
			return nil
		}

		if f, ok := node.(*File); ok {
			return f
		}

		node = node.Parent()
	}
}

func GetLastExpr(expr Expr) Expr {
	for {
		if b, ok := expr.(*Block); ok {
			if len(b.Exprs) == 0 {
				return expr
			}

			expr = b.Exprs[len(b.Exprs)-1]
		} else {
			return expr
		}
	}
}

func GetLeafAtPos(node Node, pos lexer.Pos) *Leaf {
	r := node.Range()

	if leaf, ok := node.(*Leaf); ok && r.Contains(pos) {
		return leaf
	}

	if r.IsZero() || r.Contains(pos) {
		for child := range node.Children() {
			if leaf := GetLeafAtPos(child, pos); leaf != nil {
				return leaf
			}
		}
	}

	return nil
}

func GetStructPointerType(s *Struct) *PointerType {
	modulePath := Root(s).ModulePath()

	path := &Path{Segments: make([]*Leaf, len(modulePath.Segments)+1)}
	copy(path.Segments, modulePath.Segments)
	path.Segments[len(path.Segments)-1] = &Leaf{Token: lexer.Token{Kind: lexer.Identifier, Text: s.Name()}}

	return &PointerType{Pointee: &DeclType{
		Path: path,
		Decl: s,
	}}
}
