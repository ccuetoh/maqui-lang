package maqui

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
	analyzer.Define(global)
	ast := analyzer.Do(global)

	return ast.Errors, nil
}
