package ast

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
