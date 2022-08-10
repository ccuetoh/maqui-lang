package main

import (
	"fmt"
	"os"

	"go.maqui.dev/pkg"
)

func main() {
	f, err := os.Open("./test/main.mq")
	if err != nil {
		panic(err)
	}

	lexer := maqui.NewLexer(f)

	tokens, err := lexer.RunBlocking()
	if err != nil {
		panic(err)
	}

	for _, t := range tokens {
		fmt.Println(t)
	}
}
