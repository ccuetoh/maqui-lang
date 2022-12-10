package maqui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type LexerMocker struct {
	buf []Token
	pos int
}

func NewLexerMocker(toks []Token) *LexerMocker {
	return &LexerMocker{
		buf: toks,
		pos: 0,
	}
}

func (b *LexerMocker) Do() {
	return
}

func (b *LexerMocker) Get() Token {
	if len(b.buf) <= b.pos {
		return Token{Typ: TokenEOF}
	}

	tok := b.buf[b.pos]
	b.pos++

	return tok
}

func (b *LexerMocker) GetFilename() string {
	return "testing"
}

func TestParser(t *testing.T) {
	cases := []struct {
		name   string
		data   []Token
		fail   bool
		expect []Expr
	}{
		{
			"FunctionDefinition",
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
			"Comment",
			[]Token{
				{TokenLineComment, "this is a comment", nil},
			},
			false,
			nil,
		},
		{
			"FunctionDefinitionWithComment",
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
			"UnicodeIdentifier",
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
			"FunctionDefinitionMissingArgs",
			[]Token{
				{TokenFunc, "func", nil},
				{TokenOpenCurly, "{", nil},
				{TokenCloseCurly, "}", nil},
			},
			true,
			nil,
		},
		{
			"VarString",
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
			"FunctionCall",
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
			"FunctionCallWithArgs",
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
						&LiteralExpr{Typ: LiteralString, Value: "arg1"},
						&LiteralExpr{Typ: LiteralNumber, Value: "2"},
					},
				},
			},
		},
		{
			"FunctionCallWithExpression",
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
							Operation: BinaryAddition,
							Op1:       &LiteralExpr{Typ: LiteralNumber, Value: "1"},
							Op2:       &LiteralExpr{Typ: LiteralNumber, Value: "2"},
						},
					},
				},
			},
		},
		{
			"FunctionCallInvalidExpression",
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
			"ThreeWaySum",
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
					Op1:       &LiteralExpr{Typ: LiteralNumber, Value: "1"},
					Op2: &BinaryExpr{
						Operation: BinaryMultiplication,
						Op1:       &LiteralExpr{Typ: LiteralNumber, Value: "2"},
						Op2:       &LiteralExpr{Typ: LiteralNumber, Value: "3"},
					},
				},
			},
		},
		{
			"MixedOperators",
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
					Op1:       &LiteralExpr{Typ: LiteralNumber, Value: "1"},
					Op2: &BinaryExpr{
						Operation: BinaryMultiplication,
						Op1:       &LiteralExpr{Typ: LiteralNumber, Value: "3"},
						Op2:       &LiteralExpr{Typ: LiteralNumber, Value: "2"},
					},
				},
			},
		},
		{
			"ParenthesisedExpression",
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
						Op1:       &LiteralExpr{Typ: LiteralNumber, Value: "1"},
						Op2:       &LiteralExpr{Typ: LiteralNumber, Value: "3"},
					},
					Op2: &LiteralExpr{Typ: LiteralNumber, Value: "2"},
				},
			},
		},
		{
			"UnaryNegative",
			[]Token{
				{TokenMinus, "-", nil},
				{TokenNumber, "2", nil},
			},
			false,
			[]Expr{
				&UnaryExpr{
					Operation: UnaryNegative,
					Operand:   &LiteralExpr{Typ: LiteralNumber, Value: "2"},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tokenizer := NewLexerMocker(c.data)
			p := NewParser(tokenizer)

			got := p.Run()
			expect := &AST{Filename: p.GetFilename()}

			for _, e := range c.expect {
				expect.Statements = append(expect.Statements, &AnnotatedExpr{
					Expr: e,
				})
			}

			if c.fail {
				failed := false
				for _, expr := range got.Statements {
					if _, ok := expr.Expr.(*BadExpr); ok {
						failed = true
						break
					}
				}

				if !failed {
					assert.Fail(t, "expected parsing to fail, but succeeded")
				}

				return
			}

			assert.Equal(t, expect, got)
		})
	}
}
