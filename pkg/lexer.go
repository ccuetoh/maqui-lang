package maqui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TokenType is an ID that correlates to the symbol this tokens signifies.
type TokenType uint64

// lexerState A lexer states is function that, given the lexer, might emmit a [Token], and set the next state
// by returning it. A state can return a null-state to end the lexing.
type lexerState func(l *Lexer) lexerState

//go:generate stringer -type=TokenType -trimprefix=Token
const (
	// EOF is returned by the lexer when a new rune is fetched from the stream, but it was already exhausted.
	EOF rune = 0

	// TokenError denotes a lexing error. The value of the token should contain further error details.
	TokenError TokenType = iota
	// TokenEOF denotes the end of the lexing process. Its emitted once all symbols of the stream are exhausted.
	TokenEOF

	// TokenNumber denotes a numeric value, held inside the value of the [Token]. The token does not commit to any
	// specific type of number, and can hold any of decimal, integer or complex numbers.
	TokenNumber
	// TokenString denotes a [Token] which holds a string value. The surrounding double-quotes (") are removed, and only
	// the inner value of the string should be found inside the [Token].
	TokenString

	// TokenIdentifier holds any identifier, that is, any non double-quoted (") text. An identifier might be a function,
	// variable, type and so on. No assumptions are made over the identifier, and it might be invalid or undeclared. Any
	// built-in keywords will first be matched to their respective token types, so the func keyword will be a [TokenFunc]
	// and not a [TokenIdentifier].
	TokenIdentifier
	// TokenFunc denotes the 'func' keyword.
	TokenFunc

	// TokenPlus denotes the plus (+) symbol.
	TokenPlus
	// TokenMinus denotes the minus (-) symbol.
	TokenMinus
	// TokenMulti denotes the asterisk or multiplication (*) symbol.
	TokenMulti
	// TokenDiv denotes the forward-slash or division (/) symbol.
	TokenDiv

	// TokenDeclaration denotes the declaration (:=) symbol.
	TokenDeclaration
	// TokenLineComment matches the line comment symbol (//) and held the value of the following comment until a
	// new-line is found.
	TokenLineComment
	// TokenOpenParentheses matches the opening parenthesis symbol.
	TokenOpenParentheses
	// TokenCloseParentheses matches the closing parenthesis symbol.
	TokenCloseParentheses
	// TokenOpenCurly matches the opening curly bracket (opening brace) symbol ('{').
	TokenOpenCurly
	// TokenCloseCurly matches the closing curly bracket (closing brace) symbol ('}').
	TokenCloseCurly

	// TokenComma denotes the comma symbol (',').
	TokenComma
)

// keywordTable holds all the defined keywords and their respective token. It's used to lookup if an identifier
// corresponds to a keyword.
var keywordTable = map[string]TokenType{
	"func": TokenFunc,
}

// operatorTable holds a map between operator symbols and their token. It's used to check if a given string corresponds
// to an operator token.
var operatorTable = map[string]TokenType{
	"+":  TokenPlus,
	"-":  TokenMinus,
	":=": TokenDeclaration,
	"//": TokenLineComment,
	"(":  TokenOpenParentheses,
	")":  TokenCloseParentheses,
	"{":  TokenOpenCurly,
	"}":  TokenCloseCurly,
	",":  TokenComma,
}

// Token contains a lexicographical token parsed from the input stream. A Token contains its type, an optional semantic
// value and information regarding the location on which the token was found.
//
// If a token has type [TokenError] its value should contain a description of the error. If a token is of type
// [TokenEOF] it marks the end of the stream.
type Token struct {
	// Typ holds the type of this Token.
	Typ TokenType

	// Value is an optional semantic value that holds the characters relevant to this token, or additional information
	// if the token is an error.
	Value string

	// Loc holds location data pointing to the file and position where the token was found.
	Loc *Location
}

// Location records a position inside a file.
type Location struct {
	Start uint64
	End   uint64
	File  string
}

// Tokenizer defines a lexer that transforms a given stream of text into a sequential series of Tokens.
type Tokenizer interface {
	// Do starts lexing on a goroutine, and sends the completed tokens to the results channel.
	Do()

	// Get fetches the next available token. If no token is available it blocks until one is ready.
	Get() Token

	// GetFilename returns the name of the current working file.
	GetFilename() string
}

// Lexer implements the Tokenizer interface and acts as the default tokenizer for the Maqui language. Internally, the
// lexer keeps the stream of which to read from and its state. A lexer should never be reused, and it's not thread-safe.
type Lexer struct {
	// filename is the location of the original file in disk. The provided path might be relative or absolute.
	filename string

	// reader is the current stream.
	reader *bufio.Reader

	// output is the result channel of the lexer. Once a [Token] is ready its immediately placed on the channel.
	output chan Token

	// start represents the start position of the lexer once a state begun. It's used to provide error locations for
	// error management, and not as a marker for the stream. Once a token is emitted start is set to equal pos.
	start uint64

	// pos is the current position of the lexer. It gets incremented every time a new rune is fetched from the stream
	pos uint64
}

// NewLexer creates a lexer and sets the stream to the file at the provided path.
func NewLexer(filename string) (*Lexer, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	l := NewLexerFromReader(f)
	l.filename = filename

	return l, nil
}

// NewLexerFromReader creates a lexer and sets the stream to the provided reader.
func NewLexerFromReader(reader io.Reader) *Lexer {
	return &Lexer{
		reader: bufio.NewReader(reader),
		output: make(chan Token, 2),
	}
}

// Chan gets the result channel
func (l *Lexer) Chan() chan Token {
	return l.output
}

// Get fetches the next available token. If no token is available it blocks until one is ready.
func (l *Lexer) Get() Token {
	// Comply with the Tokenizer interface.
	return <-l.Chan()
}

// GetFilename returns the name of the current working file.
func (l *Lexer) GetFilename() string {
	// Comply with the Tokenizer interface.
	return l.filename
}

// Do starts lexing on a goroutine, and sends the completed tokens to the results channel.
func (l *Lexer) Do() {
	for state := startState; state != nil; {
		state = state(l)
	}

	close(l.output)
}

// Run lexes the stream sequentially and blocks until the full output is ready or an error is encountered. [Do] and
// [Get] should always be preferred for parallelizable workloads. Internally [Run] wraps these methods in a blocking
// manner.
func (l *Lexer) Run() ([]Token, error) {
	go l.Do()

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

// startState is the default state of the lexer. Once a state has been depleted, [startState] should be used to pick the
// next one.
func startState(l *Lexer) lexerState {
	for {
		switch r := l.peek(); {
		case unicode.IsSpace(r):
			l.next()
			continue
		case r == EOF:
			return endState
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

// numberState is entered once a digit is found in the stream. The state concatenates the numeric value found
// until the next token is no longer numeric. A [Token] is then emitted as a [TokenNumber] with its value set to the
// parsed number.
func numberState(l *Lexer) lexerState {
	var num strings.Builder
	for r := l.peek(); '0' <= r && r <= '9'; r = l.peek() {
		num.WriteRune(l.next())
	}

	return l.emmitValue(TokenNumber, num.String())
}

// stringState is entered once a leading double-quote (") is found. The state builds a string, concatenating characters
// from the stream until a closing double-quote (") is found. A token is then emitted of type [TokenString] and value
// set to the parsed text. It might emmit an error if an unclosed string is found, in this case no [TokenString] is
// generated.
func stringState(l *Lexer) lexerState {
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

// identifierState is entered when a non-escaped string is found in the stream. The state builds the identifier by
// consuming from the stream up to the moment a not valid identifier character is found. If the identifier does not
// match a keyword the state emits a Token of type [TokenIdentifier] and the value set to the identifier. If the
// identifier is a keyword the keyword's type is emitted, based on the [keywordTable].
func identifierState(l *Lexer) lexerState {
	var id strings.Builder
	for r := l.peek(); unicode.IsLetter(r); r = l.peek() {
		id.WriteRune(l.next())
	}

	if t, ok := keywordTable[id.String()]; ok {
		return l.emmitValue(t, id.String())
	}

	return l.emmitValue(TokenIdentifier, id.String())
}

// operatorState is entered once a symbol matching an operator is found. If the operator starts a comment ("//" or "/*")
// the leading operator is consumed and a comment state is returned. If the operator is valid (present in the
// [operatorTable]), the corresponding token type is emitted, otherwise an error will be emitted.
func operatorState(l *Lexer) lexerState {
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

// lineCommentState is entered when a leading "//" is found. It's expected that the "//" operator is already
// consumed when this state is entered. The state builds the comment by reading all runes from the stream until
// the rune matches a new-line ("/n") or the end-of-file is reached. The emitted token is of type [TokenLineComment]
// and holds the comment as a value.
func lineCommentState(l *Lexer) lexerState {
	var id strings.Builder
	for r := l.peek(); r != '\n' && r != EOF; r = l.peek() {
		id.WriteRune(l.next())
	}

	return l.emmitValue(TokenLineComment, id.String())
}

// endState emits an end-of-file token and finishes the execution by returning a nil state as a result.
func endState(l *Lexer) lexerState {
	l.emmitNext(TokenEOF)
	return nil
}

// errorf is a shorthand for emitting a [TokenError] token with its value set to formatted string.
func (l *Lexer) errorf(format string, args ...interface{}) lexerState {
	l.output <- Token{
		Typ:   TokenError,
		Value: fmt.Sprintf(format, args...),
	}

	return endState
}

// emmitNext is a shorthand for emitting a token of the t type, and setting its value to the next token in the stream.
// The stream is advanced one position.
func (l *Lexer) emmitNext(t TokenType) lexerState {
	return l.emmitValue(t, string(l.next()))
}

// emmitValue emits a value of type t and value val. The location of the emitted token is resolved by the lexer's
// position. A [startState] is always returned.
func (l *Lexer) emmitValue(t TokenType, val string) lexerState {
	l.output <- Token{
		Typ:   t,
		Value: val,
		Loc:   l.location(),
	}

	l.start = l.pos

	return startState
}

// peek returns the next rune on the stream without advancing its position.
func (l *Lexer) peek() rune {
	r := l.next()
	if r != EOF && r != utf8.RuneError {
		l.pos-- // Revert position incrementer
	}

	_ = l.reader.UnreadRune()

	return r
}

// next fetches the next rune in the stream and consumes it by advancing one position.
func (l *Lexer) next() rune {
	r, _, err := l.reader.ReadRune()
	if err != nil {
		if err == io.EOF {
			return EOF
		}

		return utf8.RuneError
	}

	l.pos++
	return r
}

// location returns the current location data of the lexer.
func (l *Lexer) location() *Location {
	return &Location{
		File:  l.filename,
		Start: l.start,
		End:   l.pos,
	}
}

// String pretty formats the location data.
func (m *Location) String() string {
	return fmt.Sprintf("%s:[%d:%d]", path.Base(m.File), m.Start, m.End)
}

// isValid will return false if the token is of type [TokenEOF] or [TokenError], and true otherwise
func (t Token) isValid() bool {
	return t.Typ != TokenEOF && t.Typ != TokenError
}

// isEmpty returns true if the token is empty, and false otherwise
func (t Token) isEmpty() bool {
	return t.Typ != TokenEOF && t.Typ != TokenError
}

// isComment will return true only if the token is of type [TokenLineComment]
func (t Token) isComment() bool {
	return t.Typ == TokenLineComment
}
