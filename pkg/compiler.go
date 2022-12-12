package maqui

import (
	"io/fs"
	"log"
	"os"
	"os/exec"
	"runtime"
)

type Compiler struct{}

func NewCompiler() *Compiler {
	return &Compiler{}
}

func (c *Compiler) Compile(filename string) ([]CompileError, error) {
	lexer, err := NewLexer(filename)
	if err != nil {
		return nil, err
	}

	parser := NewParser(lexer)
	analyzer := NewContextAnalyser(parser)

	global := NewGlobalSymbolTable()
	analyzer.DefineInto(global)

	ast := analyzer.Do(global)
	if len(ast.Errors) != 0 {
		return ast.Errors, nil
	}

	gen := NewLLVMGenerator(ast)
	ir := gen.Do()

	err = os.Mkdir("./build", os.ModePerm)
	if err != nil {
		log.Println(err)
	}

	err = os.WriteFile("./build/temp.ll", []byte(ir.String()), fs.ModePerm)
	if err != nil {
		return nil, err
	}

	out := "./build/main"
	if runtime.GOOS == "Windows" {
		out += ".exe"
	}

	cmd := exec.Command("clang", "./build/temp.ll", "-o", out)
	if err = cmd.Run(); err != nil {
		return nil, err
	}

	return nil, nil
}
