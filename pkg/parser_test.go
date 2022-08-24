package maqui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type BufferedTokenizerMocker struct {
	buf []Token
	pos int
}

func NewBufferedTokenizerMocker(toks []Token) *BufferedTokenizerMocker {
	return &BufferedTokenizerMocker{
		buf: toks,
		pos: 0,
	}
}

func (b *BufferedTokenizerMocker) Do() {
	return
}

func (b *BufferedTokenizerMocker) Get() Token {
	if len(b.buf) <= b.pos {
		return Token{Typ: TokenEOF}
	}

	tok := b.buf[b.pos]
	b.pos++

	return tok
}

func (b *BufferedTokenizerMocker) GetFilename() string {
	return "testing"
}

func TestParser(t *testing.T) {
	cases := []struct {
		data   []Token
		fail   bool
		expect []Expr
	}{
		{
			[]Token{
				{TokenFunc, "func", nil},
				{TokenIdentifier, "main", nil},
				{TokenOpenParentheses, "(", nil},
				{TokenCloseParentheses, ")", nil},
				{TokenOpenCurly, "{", nil},
				{TokenCloseCurly, "}", nil},
			},
			false,
			[]Expr{
				&FuncDecl{
					Name: "main",
					Body: nil,
				},
			},
		},
		{
			[]Token{
				{TokenLineComment, "this is a comment", nil},
			},
			false,
			nil,
		},
		{
			[]Token{
				{TokenFunc, "func", nil},
				{TokenIdentifier, "main", nil},
				{TokenOpenParentheses, "(", nil},
				{TokenCloseParentheses, ")", nil},
				{TokenOpenCurly, "{", nil},
				{TokenLineComment, " this is a comment ", nil},
				{TokenCloseCurly, "}", nil},
			},
			false,
			[]Expr{
				&FuncDecl{
					Name: "main",
					Body: nil,
				},
			},
		},
		{
			[]Token{
				{TokenIdentifier, "únicódeShouldBeVàlid", nil},
				{TokenDeclaration, ":=", nil},
				{TokenNumber, "1", nil},
			},
			false,
			[]Expr{
				&VariableDecl{
					Name: "únicódeShouldBeVàlid",
					Value: &LiteralExpr{
						Typ:   LiteralNumber,
						Value: "1",
					},
				},
			},
		},
		{
			[]Token{
				{TokenFunc, "func", nil},
				{TokenOpenCurly, "{", nil},
				{TokenCloseCurly, "}", nil},
			},
			true,
			nil,
		},
		{
			[]Token{
				{TokenIdentifier, "varDeclExpr", nil},
				{TokenDeclaration, ":=", nil},
				{TokenString, "string", nil},
			},
			false,
			[]Expr{
				&VariableDecl{
					Name: "varDeclExpr",
					Value: &LiteralExpr{
						Typ:   LiteralString,
						Value: "string",
					},
				},
			},
		},
		{
			[]Token{
				{TokenIdentifier, "foo", nil},
				{TokenOpenParentheses, "(", nil},
				{TokenCloseParentheses, ")", nil},
			},
			false,
			[]Expr{
				&FuncCall{
					Name: "foo",
					Args: nil,
				},
			},
		},
		{
			[]Token{
				{TokenIdentifier, "foo", nil},
				{TokenOpenParentheses, "(", nil},
				{TokenString, "arg1", nil},
				{TokenComma, ",", nil},
				{TokenNumber, "2", nil},
				{TokenCloseParentheses, ")", nil},
			},
			false,
			[]Expr{
				&FuncCall{
					Name: "foo",
					Args: []Expr{
						&LiteralExpr{LiteralString, "arg1"},
						&LiteralExpr{LiteralNumber, "2"},
					},
				},
			},
		},
		{
			[]Token{
				{TokenIdentifier, "foo", nil},
				{TokenOpenParentheses, "(", nil},
				{TokenNumber, "1", nil},
				{TokenPlus, "+", nil},
				{TokenNumber, "2", nil},
				{TokenCloseParentheses, ")", nil},
			},
			false,
			[]Expr{
				&FuncCall{
					Name: "foo",
					Args: []Expr{
						&BinaryExpr{
							BinaryAddition,
							&LiteralExpr{LiteralNumber, "1"},
							&LiteralExpr{LiteralNumber, "2"},
						},
					},
				},
			},
		},
		{
			[]Token{
				{TokenIdentifier, "foo", nil},
				{TokenOpenParentheses, "(", nil},
				{TokenNumber, "1", nil},
				{TokenNumber, "2", nil},
				{TokenCloseParentheses, ")", nil},
			},
			true,
			nil,
		},
		{
			[]Token{
				{TokenNumber, "1", nil},
				{TokenPlus, "+", nil},
				{TokenNumber, "2", nil},
				{TokenMulti, "*", nil},
				{TokenNumber, "3", nil},
			},
			false,
			[]Expr{
				&BinaryExpr{
					Operation: BinaryAddition,
					Op1:       &LiteralExpr{LiteralNumber, "1"},
					Op2: &BinaryExpr{
						Operation: BinaryMultiplication,
						Op1:       &LiteralExpr{LiteralNumber, "2"},
						Op2:       &LiteralExpr{LiteralNumber, "3"},
					},
				},
			},
		},
		{
			[]Token{
				{TokenNumber, "1", nil},
				{TokenPlus, "+", nil},
				{TokenNumber, "3", nil},
				{TokenMulti, "*", nil},
				{TokenNumber, "2", nil},
			},
			false,
			[]Expr{
				&BinaryExpr{
					Operation: BinaryAddition,
					Op1:       &LiteralExpr{LiteralNumber, "1"},
					Op2: &BinaryExpr{
						Operation: BinaryMultiplication,
						Op1:       &LiteralExpr{LiteralNumber, "3"},
						Op2:       &LiteralExpr{LiteralNumber, "2"},
					},
				},
			},
		},
		{
			[]Token{
				{TokenOpenParentheses, "(", nil},
				{TokenNumber, "1", nil},
				{TokenPlus, "+", nil},
				{TokenNumber, "3", nil},
				{TokenCloseParentheses, ")", nil},
				{TokenMulti, "*", nil},
				{TokenNumber, "2", nil},
			},
			false,
			[]Expr{
				&BinaryExpr{
					Operation: BinaryMultiplication,
					Op1: &BinaryExpr{
						Operation: BinaryAddition,
						Op1:       &LiteralExpr{LiteralNumber, "1"},
						Op2:       &LiteralExpr{LiteralNumber, "3"},
					},
					Op2: &LiteralExpr{LiteralNumber, "2"},
				},
			},
		},
	}

	for _, c := range cases {
		tokenizer := NewBufferedTokenizerMocker(c.data)
		p := NewParser(tokenizer)

		got := p.Run()
		expect := &AST{
			Statements: c.expect,
		}

		if c.fail {
			failed := false
			for _, node := range got.Statements {
				if _, ok := node.(*BadExpr); ok {
					failed = true
					break
				}
			}

			if !failed {
				assert.Fail(t, "expected parsing to fail, but succeeded")
			}

			continue
		}

		assert.Equal(t, expect, got)
	}
}
