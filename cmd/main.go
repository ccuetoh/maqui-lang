package main

import (
	"fmt"
	"go.maqui.dev/pkg"
)

func main() {
	c := maqui.NewCompiler()
	err, compileErr := c.Compile("./test/main.mq")
	if err != nil {
		panic(err)
	}

	if len(compileErr) != 0 {
		printErrors(compileErr)
	}
}

func printErrors(errors []maqui.CompileError) {
	for _, err := range errors {
		switch e := err.(type) {
		case *maqui.BadExprError:
			fmt.Println("Bad expression:", e.Expr.Error, "at", e.Expr.Location)
		case *maqui.UndefinedError:
			fmt.Println("Undefined value:", e.Name, "at", e.Loc)
		case *maqui.IncompatibleTypesError:
			fmt.Println("Incompatible types:", e, "and", e.Type2, "at", e.Loc)
		case *maqui.UndefinedOperationError:
			fmt.Println("Undefined operation:", e.Op, "for type", e.Type, "at", e.Loc)
		case *maqui.UndefinedUnitaryError:
			fmt.Println("Undefined unitary:", e.Op, "for type", e.Type, "at", e.Loc)
		}
	}
}
