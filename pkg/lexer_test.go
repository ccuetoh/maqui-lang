package maqui

import (
	"strings"
	"testing"

	"go.maqui.dev/internal/test"

	"github.com/stretchr/testify/assert"
)

func TestLexer(t *testing.T) {
	cases := []struct {
		name   string
		data   string
		fail   bool
		expect []Token
	}{
		{
			"EmptyMain",
			"func main () {}",
			false,
			[]Token{
				{TokenFunc, "func", nil},
				{TokenIdentifier, "main", nil},
				{TokenOpenParentheses, "(", nil},
				{TokenCloseParentheses, ")", nil},
				{TokenOpenCurly, "{", nil},
				{TokenCloseCurly, "}", nil},
			},
		},
		{
			"SingleLineComment",
			"//this is a comment\n",
			false,
			[]Token{
				{TokenLineComment, "this is a comment", nil},
			},
		},
		{
			"MainWithSingleLineComment",
			"func main () {\n// this is a comment \n}",
			false,
			[]Token{
				{TokenFunc, "func", nil},
				{TokenIdentifier, "main", nil},
				{TokenOpenParentheses, "(", nil},
				{TokenCloseParentheses, ")", nil},
				{TokenOpenCurly, "{", nil},
				{TokenLineComment, " this is a comment ", nil},
				{TokenCloseCurly, "}", nil},
			},
		},
		{
			"UnicodeVarDeclaration",
			"únicódeShouldBeVàlid := 1",
			false,
			[]Token{
				{TokenIdentifier, "únicódeShouldBeVàlid", nil},
				{TokenDeclaration, ":=", nil},
				{TokenNumber, "1", nil},
			},
		},
		{
			"StringVarDeclaration",
			"varDeclExpr := \"string\"",
			false,
			[]Token{
				{TokenIdentifier, "varDeclExpr", nil},
				{TokenDeclaration, ":=", nil},
				{TokenString, "string", nil},
			},
		},
		{
			"EmptyString",
			"\"\"",
			false,
			[]Token{
				{TokenString, "", nil},
			},
		},
		{
			"UnclosedString",
			"\"unclosed string",
			true,
			nil,
		},
		{
			"BadCharacter",
			"@",
			true,
			nil,
		},
		{
			"EmptyIfElse",
			"if {} else {} ",
			false,
			[]Token{
				{TokenIf, "if", nil},
				{TokenOpenCurly, "{", nil},
				{TokenCloseCurly, "}", nil},
				{TokenElse, "else", nil},
				{TokenOpenCurly, "{", nil},
				{TokenCloseCurly, "}", nil},
			},
		},
		{
			"IfExprCond",
			"if 1+1 {} else {} ",
			false,
			[]Token{
				{TokenIf, "if", nil},
				{TokenNumber, "1", nil},
				{TokenPlus, "+", nil},
				{TokenNumber, "1", nil},
				{TokenOpenCurly, "{", nil},
				{TokenCloseCurly, "}", nil},
				{TokenElse, "else", nil},
				{TokenOpenCurly, "{", nil},
				{TokenCloseCurly, "}", nil},
			},
		},
		{
			"IfWithBody",
			"if 1+1 {\n2 - 3 \n 2 + 3\n} else {\n1 - 2 \n 1 + 2\n} ",
			false,
			[]Token{
				{TokenIf, "if", nil},
				{TokenNumber, "1", nil},
				{TokenPlus, "+", nil},
				{TokenNumber, "1", nil},
				{TokenOpenCurly, "{", nil},
				{TokenNumber, "2", nil},
				{TokenMinus, "-", nil},
				{TokenNumber, "3", nil},
				{TokenNumber, "2", nil},
				{TokenPlus, "+", nil},
				{TokenNumber, "3", nil},
				{TokenCloseCurly, "}", nil},
				{TokenElse, "else", nil},
				{TokenOpenCurly, "{", nil},
				{TokenNumber, "1", nil},
				{TokenMinus, "-", nil},
				{TokenNumber, "2", nil},
				{TokenNumber, "1", nil},
				{TokenPlus, "+", nil},
				{TokenNumber, "2", nil},
				{TokenCloseCurly, "}", nil},
			},
		},
		{
			"SimpleEquals",
			"1 == 1",
			false,
			[]Token{
				{TokenNumber, "1", nil},
				{TokenBooleanEquals, "==", nil},
				{TokenNumber, "1", nil},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := strings.NewReader(c.data)
			l := NewLexerFromReader(r)

			toks, err := l.Run()
			if c.fail {
				assert.Error(t, err)
			}

			for i := 0; i < len(toks); i++ {
				toks[i].Loc = nil // ignore meta
			}

			assert.Equal(t, c.expect, toks)
		})
	}
}

// Use a package-level variable to avoid compiler optimisation
var benchResult []Token

func benchmarkLexer(size int, b *testing.B) {
	for n := 0; n < b.N; n++ {
		// Setup
		b.StopTimer()
		data := test.GetRandomTokens(size)
		r := strings.NewReader(data)
		l := NewLexerFromReader(r)

		var err error
		b.StartTimer()

		benchResult, err = l.Run()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLexer100(b *testing.B) {
	benchmarkLexer(100, b)
}

func BenchmarkLexer1000(b *testing.B) {
	benchmarkLexer(1000, b)
}

func BenchmarkLexer10000(b *testing.B) {
	benchmarkLexer(10000, b)
}

func BenchmarkLexer100000(b *testing.B) {
	benchmarkLexer(100000, b)
}

func BenchmarkLexer1000000(b *testing.B) {
	benchmarkLexer(1000000, b)
}
