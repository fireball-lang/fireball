package main

import (
	"fireball/analyzer"
	"fireball/ast"
	"fireball/cst"
	"fmt"
	"os"
)

func main() {
	b, _ := os.ReadFile("example/test.fb")
	node, diagnostics := cst.Parse(string(b))

	printNode(&node, 0)

	file := ast.Convert(&node)

	fmt.Println()
	ast.Print(file)

	scope := analyzer.NewSimpleScope(analyzer.CollectTypeDecls(file))
	diagnostics = append(diagnostics, analyzer.Analyze(file, scope)...)

	fmt.Println()
	for _, diagnostic := range diagnostics {
		fmt.Println(diagnostic)
	}
}

func printNode(node *cst.Node, indent int) {
	for i := 0; i < indent; i++ {
		fmt.Print("  ")
	}

	fmt.Println(node)

	for i := range node.Children {
		printNode(&node.Children[i], indent+1)
	}
}
