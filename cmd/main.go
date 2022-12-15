package main

import (
	"fmt"
	"go.maqui.dev/pkg"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Expected one argument: source location")
		return
	}

	source := os.Args[1]

	c := maqui.NewCompiler(maqui.Target{
		Arch:   maqui.X86_64,
		Vendor: maqui.Unknown,
		OS:     maqui.Linux,
	})

	compileErr, err := c.Compile(source)
	if err != nil {
		panic(err.Error())
	}

	if len(compileErr) != 0 {
		for _, err := range compileErr {
			fmt.Println(err)
		}

		return
	}

	fmt.Println("Ok")
}
