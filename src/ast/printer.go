package ast

import "fmt"

type printer struct {
	indent int
}

func Print(f *File) {
	p := printer{}

	fmt.Println("File")

	p.indent++
	for _, decl := range f.Decls {
		p.visit(decl)
	}
	p.indent--
}

func (p *printer) visit(node Node) {
	if decl, ok := node.(Decl); ok {
		for i := 0; i < p.indent; i++ {
			fmt.Print("  ")
		}

		decl.Visit(p)
	} else if expr, ok := node.(Expr); ok {
		for i := 0; i < p.indent; i++ {
			fmt.Print("  ")
		}

		expr.Visit(p)
	}

	p.indent++
	for child := range node.Children() {
		p.visit(child)
	}
	p.indent--
}

// Decls

func (p *printer) VisitMod(s *Mod) {
	fmt.Print("Mod '")
	printPath(s.Path)
	fmt.Println('\'')
}

func (p *printer) VisitImport(i *Import) {
	fmt.Print("Import '")
	printPath(i.Path)
	fmt.Printf("' %d symbols\n", len(i.Symbols))
}

func (p *printer) VisitStruct(s *Struct) {
	fmt.Printf("Struct '%s'\n", s.Name())

	for _, field := range s.Fields {
		for i := 0; i < p.indent+1; i++ {
			fmt.Print("  ")
		}

		name := ""
		if field.Name != nil {
			name = field.Name.Token.Text
		}

		fmt.Printf("Field '%s'\n", name)
	}
}

func (p *printer) VisitImpl(i *Impl) {
	fmt.Printf("Impl '%s'\n", i.Name())
}

func (p *printer) VisitGlobalVar(g *GlobalVar) {
	fmt.Printf("GlobalVar '%s'\n", g.Name())
}

func (p *printer) VisitFunc(f *Func) {
	fmt.Printf("Func '%s'\n", f.Name())
}

// Exprs

func (p *printer) VisitBlock(b *Block) {
	fmt.Println("Block")
}

func (p *printer) VisitVar(v *Var) {
	name := ""
	if v.Name != nil {
		name = v.Name.Token.Text
	}

	fmt.Printf("Var '%s'\n", name)
}

func (p *printer) VisitIf(i *If) {
	fmt.Println("If")
}

func (p *printer) VisitWhile(w *While) {
	fmt.Println("While")
}

func (p *printer) VisitBreak(b *Break) {
	fmt.Println("Break")
}

func (p *printer) VisitContinue(c *Continue) {
	fmt.Println("Continue")
}

func (p *printer) VisitReturn(r *Return) {
	fmt.Println("Return")
}

func (p *printer) VisitLiteral(l *Literal) {
	fmt.Printf("Literal '%s'\n", l.Value.Token.Text)
}

func (p *printer) VisitStructInitializer(s *StructInitializer) {
	fmt.Printf("StructInitializer '%s'\n", s.Name.Token.Text)
}

func (p *printer) VisitParen(pa *Paren) {
	fmt.Println("Paren")
}

func (p *printer) VisitIdentifier(i *Identifier) {
	fmt.Printf("Identifier '")
	printPath(i.Path)
	fmt.Println('\'')
}

func (p *printer) VisitCall(c *Call) {
	fmt.Println("Call")
}

func (p *printer) VisitIndex(i *Index) {
	fmt.Println("Index")
}

func (p *printer) VisitMember(m *Member) {
	name := ""
	if m.Name != nil {
		name = m.Name.Token.Text
	}

	fmt.Printf("Member '%s'\n", name)
}

func (p *printer) VisitUnary(u *Unary) {
	mode := "prefix"
	if u.Postfix {
		mode = "postfix"
	}

	fmt.Printf("Unary '%s', %s\n", u.Op, mode)
}

func (p *printer) VisitBinary(b *Binary) {
	fmt.Printf("Binary '%s'\n", b.Op)
}

func (p *printer) VisitCast(c *Cast) {
	fmt.Printf("Cast '%s'\n", c.Type.String())
}

// Utils

func printPath(p *Path) {
	if p == nil {
		return
	}

	for i, segment := range p.Segments {
		if i > 0 {
			fmt.Print(':')
		}

		fmt.Print(segment.Token.Text)
	}
}
