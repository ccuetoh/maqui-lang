package main

import (
	"fmt"
	"go.maqui.dev/pkg"
)

func main() {
	c := maqui.NewCompiler()
	compileErr, err := c.Compile("./main.mq")
	if err != nil {
		panic(err)
	}

	if len(compileErr) != 0 {
		for _, err := range compileErr {
			fmt.Println(err)
		}

		return
	}

	fmt.Println("Ok")
}
