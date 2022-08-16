package maqui

import (
	"fmt"
	"io"
)

type Compiler struct{}

func NewCompiler() *Compiler {
	return &Compiler{}
}

func (c *Compiler) Compile(filename string) error {
	lexer, err := NewLexer(filename)
	if err != nil {
		return err
	}

	parser := NewParser(lexer)
	return c.compile(parser)
}

func (c *Compiler) CompileFromReader(reader io.Reader) error {
	lexer := NewLexerFromReader(reader)
	parser := NewParser(lexer)

	return c.compile(parser)
}

func (c *Compiler) compile(p *Parser) error {
	ast := p.Run()

	// TODO
	fmt.Println(ast)
	return nil

}
