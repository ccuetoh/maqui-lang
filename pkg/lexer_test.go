package maqui

import (
	"go.maqui.dev/internal/test"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLexer(t *testing.T) {
	cases := []struct {
		data   string
		fail   bool
		expect []Token
	}{
		{
			"func main () {}",
			false,
			[]Token{
				{TokenFunc, "func"},
				{TokenIdentifier, "main"},
				{TokenOpenParentheses, "("},
				{TokenCloseParentheses, ")"},
				{TokenOpenCurly, "{"},
				{TokenCloseCurly, "}"},
			},
		},
		{
			"//this is a comment\n",
			false,
			[]Token{
				{TokenLineComment, "this is a comment"},
			},
		},
		{
			"func main () {\n// this is a comment \n}",
			false,
			[]Token{
				{TokenFunc, "func"},
				{TokenIdentifier, "main"},
				{TokenOpenParentheses, "("},
				{TokenCloseParentheses, ")"},
				{TokenOpenCurly, "{"},
				{TokenLineComment, " this is a comment "},
				{TokenCloseCurly, "}"},
			},
		},
		{
			"únicódeShouldBeVàlid := 1",
			false,
			[]Token{
				{TokenIdentifier, "únicódeShouldBeVàlid"},
				{TokenDeclaration, ":="},
				{TokenNumber, "1"},
			},
		},
		{
			"identifier := \"string\"",
			false,
			[]Token{
				{TokenIdentifier, "identifier"},
				{TokenDeclaration, ":="},
				{TokenString, "string"},
			},
		},
		{
			"\"\"",
			false,
			[]Token{
				{TokenString, ""},
			},
		},
		{
			"\"unclosed string",
			true,
			nil,
		},
		{
			"@",
			true,
			nil,
		},
	}

	for _, c := range cases {
		r := strings.NewReader(c.data)
		l := NewLexer(r)

		toks, err := l.RunBlocking()
		if c.fail {
			assert.Error(t, err)
		}

		assert.Equal(t, c.expect, toks)

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
		l := NewLexer(r)

		var err error
		b.StartTimer()

		benchResult, err = l.RunBlocking()
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
