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
							ResolvedType: nil,
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
	}

	for _, c := range cases {
		parser := NewParserMocker(c.data)
		analyzer := NewContextAnalyser(parser)

		got := analyzer.Do()
		assert.Equal(t, c.expect, got)
	}
}
