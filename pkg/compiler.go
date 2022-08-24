package maqui

import (
	"io"
)

type Compiler struct{}

func NewCompiler() *Compiler {
	return &Compiler{}
}

func (c *Compiler) Compile(filename string) (error, []CompileError) {
	lexer, err := NewLexer(filename)
	if err != nil {
		return err, nil
	}

	parser := NewParser(lexer)
	analyzer := NewContextAnalyser(parser)
	return nil, c.compile(analyzer)
}

func (c *Compiler) CompileFromReader(reader io.Reader) []CompileError {
	lexer := NewLexerFromReader(reader)
	parser := NewParser(lexer)
	analyzer := NewContextAnalyser(parser)

	return c.compile(analyzer)
}

func (c *Compiler) compile(ca *ContextAnalyzer) []CompileError {
	ast := ca.Do()
	return ast.Errors
}
