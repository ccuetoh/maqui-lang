package maqui

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type ParserMocker struct {
	buf []Expr
	pos int
}

func NewParserMocker(exprs []Expr) *ParserMocker {
	return &ParserMocker{
		buf: exprs,
		pos: 0,
	}
}

func (b *ParserMocker) Do() {
	return
}

func (b *ParserMocker) Get() Expr {
	if len(b.buf) <= b.pos {
		return &EOS{}
	}

	expr := b.buf[b.pos]
	b.pos++

	return expr
}

func (b *ParserMocker) GetFilename() string {
	return "testing"
}

func TestContextAnalyzer(t *testing.T) {
	cases := []struct {
		data   []Expr
		expect *AST
	}{
		{
			[]Expr{
				&FuncDecl{
					Name: "main",
					Body: []Expr{
						&VariableDecl{
							Name: "x",
							Value: &BinaryExpr{
								Operation: BinaryAddition,
								Op1: &LiteralExpr{
									Typ:   LiteralNumber,
									Value: "1",
								},
								Op2: &LiteralExpr{
									Typ:   LiteralNumber,
									Value: "1",
								},
							},
						},
					},
				},
			},
			&AST{
				Statements: []Expr{
					&FuncDecl{
						Name: "main",
						Body: []Expr{
							&VariableDecl{
								Name: "x",
								Value: &BinaryExpr{
									Operation: BinaryAddition,
									Op1: &LiteralExpr{
										Typ:   LiteralNumber,
										Value: "1",
									},
									Op2: &LiteralExpr{
										Typ:   LiteralNumber,
										Value: "1",
									},
								},
								ResolvedType: &BasicType{
									Typ: "int",
								},
							},
						},
					},
				},
				Errors: nil,
			},
		},
		{
			[]Expr{
				&VariableDecl{
					Name: "x",
					Value: &BinaryExpr{
						Operation: BinaryAddition,
						Op1: &LiteralExpr{
							Typ:   LiteralNumber,
							Value: "1",
						},
						Op2: &LiteralExpr{
							Typ:   LiteralString,
							Value: "text",
						},
					},
				},
			},
			&AST{
				Statements: []Expr{
					&VariableDecl{
						Name: "x",
						Value: &BinaryExpr{
							Operation: BinaryAddition,
							Op1: &LiteralExpr{
								Typ:   LiteralNumber,
								Value: "1",
							},
							Op2: &LiteralExpr{
								Typ:   LiteralString,
								Value: "text",
							},
						},
					},
				},
				Errors: []CompileError{
					&IncompatibleTypesError{
						Type1: &BasicType{Typ: "int"},
						Type2: &BasicType{Typ: "string"},
					},
				},
			},
		},
		{
			[]Expr{
				&FuncDecl{
					Name: "foo",
					Body: []Expr{},
				},
				&FuncCall{
					Name: "foo",
					Args: []Expr{},
				},
			},
			&AST{
				Statements: []Expr{
					&FuncDecl{
						Name: "foo",
						Body: []Expr{},
					},
					&FuncCall{
						Name: "foo",
						Args: []Expr{},
					},
				},
				Errors: nil,
			},
		},
		{
			[]Expr{
				&FuncCall{
					Name: "foo",
					Args: []Expr{},
				},
			},
			&AST{
				Statements: []Expr{
					&FuncCall{
						Name: "foo",
						Args: []Expr{},
					},
				},
				Errors: []CompileError{
					&UndefinedError{
						Name: "foo",
					},
				},
			},
		},
		{
			[]Expr{
				&Identifier{
					Name: "x",
				},
			},
			&AST{
				Statements: []Expr{
					&Identifier{
						Name: "x",
					},
				},
				Errors: []CompileError{
					&UndefinedError{
						Name: "x",
					},
				},
			},
		},
		{
			[]Expr{
				&UnaryExpr{
					Operation: UnaryNegative,
					Operand: &LiteralExpr{
						Typ:   LiteralNumber,
						Value: "1",
					},
				},
			},
			&AST{
				Statements: []Expr{
					&UnaryExpr{
						Operation: UnaryNegative,
						Operand: &LiteralExpr{
							Typ:   LiteralNumber,
							Value: "1",
						},
					},
				},
				Errors: nil,
			},
		},
		{
			[]Expr{
				&UnaryExpr{
					Operation: UnaryNegative,
					Operand: &LiteralExpr{
						Typ:   LiteralString,
						Value: "foo",
					},
				},
			},
			&AST{
				Statements: []Expr{
					&UnaryExpr{
						Operation: UnaryNegative,
						Operand: &LiteralExpr{
							Typ:   LiteralString,
							Value: "foo",
						},
					},
				},
				Errors: []CompileError{
					&UndefinedUnitaryError{
						Type: &BasicType{"string"},
						Op:   UnaryNegative,
					},
				},
			},
		},
		{
			[]Expr{
				&BinaryExpr{
					Operation: BinarySubtraction,
					Op1: &LiteralExpr{
						Typ:   LiteralString,
						Value: "foo",
					},
					Op2: &LiteralExpr{
						Typ:   LiteralString,
						Value: "bar",
					},
				},
			},
			&AST{
				Statements: []Expr{
					&BinaryExpr{
						Operation: BinarySubtraction,
						Op1: &LiteralExpr{
							Typ:   LiteralString,
							Value: "foo",
						},
						Op2: &LiteralExpr{
							Typ:   LiteralString,
							Value: "bar",
						},
					},
				},
				Errors: []CompileError{
					&UndefinedOperationError{
						Type: &BasicType{"string"},
						Op:   BinarySubtraction,
					},
				},
			},
		},
		{
			[]Expr{
				&VariableDecl{
					Name: "x",
					Value: &BinaryExpr{
						Operation: BinaryAddition,
						Op1: &LiteralExpr{
							Typ:   LiteralNumber,
							Value: "1",
						},
						Op2: &LiteralExpr{
							Typ:   LiteralNumber,
							Value: "1",
						},
					},
				},
				&VariableDecl{
					Name: "y",
					Value: &BinaryExpr{
						Operation: BinaryAddition,
						Op1: &LiteralExpr{
							Typ:   LiteralNumber,
							Value: "1",
						},
						Op2: &Identifier{
							Name: "x",
						},
					},
				},
			},
			&AST{
				Statements: []Expr{
					&VariableDecl{
						Name: "x",
						Value: &BinaryExpr{
							Operation: BinaryAddition,
							Op1: &LiteralExpr{
								Typ:   LiteralNumber,
								Value: "1",
							},
							Op2: &LiteralExpr{
								Typ:   LiteralNumber,
								Value: "1",
							},
						},
						ResolvedType: &BasicType{"int"},
					},
					&VariableDecl{
						Name: "y",
						Value: &BinaryExpr{
							Operation: BinaryAddition,
							Op1: &LiteralExpr{
								Typ:   LiteralNumber,
								Value: "1",
							},
							Op2: &Identifier{
								Name: "x",
							},
						},
						ResolvedType: &BasicType{"int"},
					},
				},
				Errors: nil,
			},
		},
		{
			[]Expr{
				&VariableDecl{
					Name: "y",
					Value: &BinaryExpr{
						Operation: BinaryAddition,
						Op1: &LiteralExpr{
							Typ:   LiteralNumber,
							Value: "1",
						},
						Op2: &Identifier{
							Name: "x",
						},
					},
				},
			},
			&AST{
				Statements: []Expr{
					&VariableDecl{
						Name: "y",
						Value: &BinaryExpr{
							Operation: BinaryAddition,
							Op1: &LiteralExpr{
								Typ:   LiteralNumber,
								Value: "1",
							},
							Op2: &Identifier{
								Name: "x",
							},
						},
					},
				},
				Errors: []CompileError{
					&UndefinedError{
						Name: "x",
					},
				},
			},
		},
	}

	for _, c := range cases {
		parser := NewParserMocker(c.data)
		analyzer := NewContextAnalyser(parser)

		got := analyzer.Do()
		assert.Equal(t, c.expect, got)
	}
}

func TestTypeEquals(t *testing.T) {
	tInt1 := &BasicType{"int"}
	tInt2 := &BasicType{"int"}
	tStr := &BasicType{"string"}

	tFunc1 := &FuncType{
		Args: []*ArgumentType{
			{
				Name: "arg1",
				Type: tInt1,
			},
		},
		Returns: []*BasicType{tStr},
	}

	tFunc2 := &FuncType{
		Args: []*ArgumentType{
			{
				Name: "arg1",
				Type: tInt1,
			},
		},
		Returns: []*BasicType{tStr},
	}

	tFunc3 := &FuncType{
		Args: []*ArgumentType{
			{
				Name: "arg1",
				Type: tInt1,
			},
		},
		Returns: []*BasicType{tInt1},
	}

	assert.True(t, tInt1.Equals(tInt2))
	assert.True(t, tInt2.Equals(tInt1))
	assert.False(t, tStr.Equals(tInt1))
	assert.False(t, tInt1.Equals(tStr))
	assert.False(t, tFunc1.Equals(tStr))
	assert.True(t, tFunc1.Equals(tFunc2))
	assert.True(t, tFunc2.Equals(tFunc1))
	assert.False(t, tFunc2.Equals(tFunc3))
	assert.False(t, tFunc1.Equals(tFunc3))
}

func TestTypeString(t *testing.T) {
	tInt := &BasicType{"int"}
	tFunc := &FuncType{
		Args: []*ArgumentType{
			{
				Name: "arg1",
				Type: &BasicType{"string"},
			},
			{
				Name: "arg2",
				Type: &BasicType{"int"},
			},
		},
		Returns: []*BasicType{
			{"string"},
			{"int"},
		},
	}

	assert.Equal(t, "int", tInt.String())
	assert.Equal(t, "func(string, int) string, int", tFunc.String())
}
