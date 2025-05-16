package main

import (
	"fireball/lexer"
	"fmt"
	"os"
)

func main() {
	b, _ := os.ReadFile("example/test.fb")
	l := lexer.NewLexer(string(b))

	for {
		t := l.Next()
		fmt.Println(t)

		if t.Kind == lexer.Eof {
			break
		}
	}
}
