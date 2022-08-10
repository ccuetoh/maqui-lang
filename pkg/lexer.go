package maqui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type TokenType uint64
type stateFunc func(l *Lexer) stateFunc

//go:generate stringer -type=TokenType -trimprefix=Token
const (
	EOF rune = 0

	TokenError TokenType = iota
	TokenEOF
	TokenNumber
	TokenString

	TokenIdentifier
	TokenFunc

	TokenPlus
	TokenMinus
	TokenDeclaration
	TokenLineComment
	TokenOpenParentheses
	TokenCloseParentheses
	TokenOpenCurly
	TokenCloseCurly
)

var keywordTable = map[string]TokenType{
	"func": TokenFunc,
}

var operatorTable = map[string]TokenType{
	"+":  TokenPlus,
	"-":  TokenMinus,
	":=": TokenDeclaration,
	"//": TokenLineComment,
	"(":  TokenOpenParentheses,
	")":  TokenCloseParentheses,
	"{":  TokenOpenCurly,
	"}":  TokenCloseCurly,
}

type Token struct {
	Typ   TokenType
	Value string
}

type Lexer struct {
	reader *bufio.Reader
	done   chan Token
}

func NewLexer(reader io.Reader) *Lexer {
	return &Lexer{
		reader: bufio.NewReader(reader),
		done:   make(chan Token),
	}
}

func (l *Lexer) Chan() chan Token {
	return l.done
}

func (l *Lexer) Run() {
	for state := defaultState; state != nil; {
		state = state(l)
	}

	close(l.done)
}

func (l *Lexer) RunBlocking() ([]Token, error) {
	go l.Run()

	var tokens []Token
	for {
		select {
		case t := <-l.Chan():
			if t.Typ == TokenEOF {
				return tokens, nil
			}

			if t.Typ == TokenError {
				return nil, errors.New(t.Value)
			}

			tokens = append(tokens, t)
		}
	}
}

func defaultState(l *Lexer) stateFunc {
	for {
		switch r := l.peek(); {
		case r == EOF:
			l.emmitNext(TokenEOF)
			return nil
		case unicode.IsSpace(r):
			l.next()
			continue
		case '0' <= r && r <= '9':
			return numberState
		case r == '"':
			return stringState
		case unicode.IsLetter(r):
			return identifierState
		default:
			return operatorState
		}
	}
}

func numberState(l *Lexer) stateFunc {
	var num strings.Builder
	for r := l.peek(); '0' <= r && r <= '9'; r = l.peek() {
		num.WriteRune(l.next())
	}

	return l.emmitValue(TokenNumber, num.String())
}

func stringState(l *Lexer) stateFunc {
	l.next() // Skip the leading double-quote

	var str strings.Builder
	for r := l.next(); r != '"'; r = l.next() {
		if r == EOF {
			return l.errorf("unclosed string: %s", str.String())
		}

		str.WriteRune(r)
	}

	return l.emmitValue(TokenString, str.String())
}

func identifierState(l *Lexer) stateFunc {
	var id strings.Builder
	for r := l.peek(); unicode.IsLetter(r); r = l.peek() {
		id.WriteRune(l.next())
	}

	if t, ok := keywordTable[id.String()]; ok {
		return l.emmitValue(t, id.String())
	}

	return l.emmitValue(TokenIdentifier, id.String())
}

func operatorState(l *Lexer) stateFunc {
	r := l.next()
	if r == ':' || r == '/' { // Some operators can be two runes
		op := string(r) + string(l.peek())
		if tok, ok := operatorTable[string(r)+string(l.peek())]; ok {
			l.next() // Skip

			if tok == TokenLineComment {
				return lineCommentState
			}

			return l.emmitValue(tok, op)
		}
	}

	if tok, ok := operatorTable[string(r)]; ok {
		return l.emmitValue(tok, string(r))
	}

	return l.errorf("invalid symbol '%c'", r)
}

func lineCommentState(l *Lexer) stateFunc {
	var id strings.Builder
	for r := l.peek(); r != '\n' && r != EOF; r = l.peek() {
		id.WriteRune(l.next())
	}

	return l.emmitValue(TokenLineComment, id.String())
}

func (l *Lexer) errorf(format string, args ...interface{}) stateFunc {
	l.done <- Token{
		Typ:   TokenError,
		Value: fmt.Sprintf(format, args...),
	}

	return nil
}

func (l *Lexer) emmitNext(t TokenType) stateFunc {
	l.done <- Token{
		Typ:   t,
		Value: string(l.next()),
	}

	return defaultState
}

func (l *Lexer) emmitValue(t TokenType, val string) stateFunc {
	l.done <- Token{
		Typ:   t,
		Value: val,
	}

	return defaultState
}

func (l *Lexer) peek() rune {
	r := l.next()
	_ = l.reader.UnreadRune()

	return r
}

func (l *Lexer) next() rune {
	r, _, err := l.reader.ReadRune()
	if err != nil {
		if err == io.EOF {
			return EOF
		}

		return utf8.RuneError
	}

	return r
}
