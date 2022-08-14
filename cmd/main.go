package main

import (
	"go.maqui.dev/pkg"
)

func main() {
	c := maqui.NewCompiler()
	err := c.Compile("./test/main.mq")
	if err != nil {
		panic(err)
	}
}
