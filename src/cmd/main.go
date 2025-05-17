package main

import (
	"fireball/ast"
	"fireball/cst"
	"fmt"
	"os"
)

func main() {
	b, _ := os.ReadFile("example/test.fb")
	node, diagnostics := cst.Parse(string(b))

	for _, diagnostic := range diagnostics {
		fmt.Println(diagnostic)
	}

	fmt.Println()
	printNode(&node, 0)

	file := ast.Convert(&node)

	fmt.Println()
	ast.Print(file)
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
