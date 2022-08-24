package maqui

import "fmt"

type AST struct {
	Statements []Expr
	Errors     []CompileError
}

type Expr interface{}

type BadExpr struct {
	Location *Location
	Error    string
}

type FuncDecl struct {
	Name string
	Body []Expr
}

type VariableDecl struct {
	Name         string
	Value        Expr
	ResolvedType TypeInfo
}

type FuncCall struct {
	Name          string
	Args          []Expr
	ResolvedTypes []TypeInfo
}

type Identifier struct {
	Name string
}

type EOS struct{}

type BinaryOp string

const (
	BinaryAddition       BinaryOp = "+"
	BinarySubtraction    BinaryOp = "-"
	BinaryMultiplication BinaryOp = "*"
	BinaryDivision       BinaryOp = "/"
)

type BinaryExpr struct {
	Operation BinaryOp
	Op1       Expr
	Op2       Expr
}

type UnaryOp string

const (
	UnaryNegative UnaryOp = "-"
)

type UnaryExpr struct {
	Operation UnaryOp
	Operand   Expr
}

type LiteralType int

const (
	LiteralNumber LiteralType = iota
	LiteralString
)

type LiteralExpr struct {
	Typ   LiteralType
	Value string
}

type SyntacticAnalyzer interface {
	Do()
	Get() Expr
	GetFilename() string
}

type Parser struct {
	filename  string
	tokenizer Tokenizer
	output    chan Expr
	buf       *Token
}

func NewParser(tokenizer Tokenizer) *Parser {
	return &Parser{
		tokenizer: tokenizer,
		filename:  tokenizer.GetFilename(),
		output:    make(chan Expr, 2),
	}
}
func (p *Parser) Get() Expr {
	return <-p.Chan()
}

func (p *Parser) Chan() chan Expr {
	return p.output
}

func (p *Parser) GetFilename() string {
	return p.filename
}

func (p *Parser) Do() {
	go p.tokenizer.Do()

	for p.peek().Typ != TokenEOF {
		p.output <- p.statement()
	}

	p.output <- &EOS{}
	close(p.output)
}

func (p *Parser) Run() *AST {
	go p.tokenizer.Do()

	ast := &AST{}

	for p.peek().Typ != TokenEOF {
		ast.Statements = append(ast.Statements, p.statement())
	}

	return ast
}

func (p *Parser) peek() Token {
	if p.buf == nil {
		temp := p.next()
		p.buf = &temp
	}

	return *p.buf
}

func (p *Parser) next() Token {
	if p.buf != nil {
		if !p.buf.isValid() {
			// If an invalid token is buffered, don't try to get more tokens
			return *p.buf
		}

		temp := p.buf
		p.buf = nil

		return *temp
	}

	tok := p.tokenizer.Get()
	if !tok.isValid() {
		// If a token is invalid (such as Error or EOF) keep it buffered since no more valid tokens are expected
		p.buf = &tok
	}

	if tok.isComment() {
		return p.next()
	}

	return tok
}

func (p *Parser) expect(typ TokenType) *Token {
	tok := p.next()
	if tok.Typ != typ {
		return nil
	}

	return &tok
}

func (p *Parser) check(typ TokenType) bool {
	return p.peek().Typ == typ
}

func (p *Parser) consume(typ TokenType) bool {
	tok := p.next()
	if tok.Typ != typ {
		return false
	}

	return true
}

func (p *Parser) errorf(l *Location, format string, args ...interface{}) Expr {
	return &BadExpr{l, fmt.Sprintf(format, args...)}
}

func (p *Parser) statement() Expr {
	switch tok := p.peek(); tok.Typ {
	case TokenFunc:
		return p.funcDecl()
	default:
		return p.expr()
	}
}

func (p *Parser) funcDecl() Expr {
	start := p.next().Loc // func keyword

	name := p.expect(TokenIdentifier)
	if name == nil {
		return p.errorf(start, "expected function name")
	}

	// TODO: Allow arguments
	if !p.consume(TokenOpenParentheses) || !p.consume(TokenCloseParentheses) {
		return p.errorf(start, "bad function declaration")
	}

	return &FuncDecl{
		Name: name.Value,
		Body: p.blockStmt(),
	}
}

func (p *Parser) blockStmt() []Expr {
	if tok := p.expect(TokenOpenCurly); tok == nil {
		return []Expr{p.errorf(nil, "invalid block statement")}
	}

	var exprs []Expr
	for tok := p.peek(); tok.isValid() && tok.Typ != TokenCloseCurly; tok = p.peek() {
		exprs = append(exprs, p.statement())
	}

	switch closer := p.next(); closer.Typ {
	case TokenCloseCurly:
		return exprs
	case TokenError:
		return append(exprs, p.errorf(closer.Loc, "invalid block statement"))
	case TokenEOF:
		return append(exprs, p.errorf(closer.Loc, "unclosed block statement"))
	default:
		return append(exprs, p.errorf(closer.Loc, "unexpected token in block statement"))
	}
}

func (p *Parser) expr() Expr {
	expr := p.additiveExpr()

	id, ok := expr.(*Identifier)
	if ok {
		tok := p.peek()
		if tok.Typ == TokenDeclaration {
			return p.varDeclExpr(id)
		}

		if tok.Typ == TokenOpenParentheses {
			return p.funcCall(id)
		}
	}

	return expr
}

func (p *Parser) varDeclExpr(id *Identifier) Expr {
	if p.peek().Typ != TokenDeclaration {
		return id
	}

	p.next() // Skip :=

	return &VariableDecl{
		Name:  id.Name,
		Value: p.expr(),
	}
}

func (p *Parser) funcCall(id *Identifier) *FuncCall {
	if !p.consume(TokenOpenParentheses) {
		p.errorf(nil, "bad function call")
	}

	var args []Expr
	for tok := p.peek(); tok.isValid() && tok.Typ != TokenCloseParentheses; tok = p.peek() {
		args = append(args, p.expr())

		if !p.check(TokenComma) {
			break
		}

		p.next() // Skip the comma
	}

	if !p.consume(TokenCloseParentheses) {
		p.errorf(nil, "bad function call")
	}

	return &FuncCall{
		Name: id.Name,
		Args: args,
	}
}

func (p *Parser) additiveExpr() Expr {
	lhs := p.multiplicativeExpr()

	for true {
		if tok := p.peek(); tok.Typ == TokenPlus || tok.Typ == TokenMinus {
			// Chained operands (for example 1 - 3 + 1). Go over the operand and nest
			p.next()

			rhs := p.additiveExpr()
			lhs = &BinaryExpr{
				Operation: BinaryOp(tok.Value),
				Op1:       lhs,
				Op2:       rhs,
			}

			continue
		}

		return lhs
	}

	return lhs // Unreachable
}

func (p *Parser) multiplicativeExpr() Expr {
	lhs := p.unaryExpr()

	for true {
		if tok := p.peek(); tok.Typ == TokenMulti || tok.Typ == TokenDiv {
			// Chained operands (for example 1 / 3 * 1). Go over the operand and nest
			p.next()

			rhs := p.multiplicativeExpr()
			lhs = &BinaryExpr{
				Operation: BinaryOp(tok.Value),
				Op1:       lhs,
				Op2:       rhs,
			}

			continue
		}

		return lhs
	}

	return lhs // Unreachable
}

func (p *Parser) unaryExpr() Expr {
	if p.check(TokenMinus) { // Unary negative
		p.next()

		return &UnaryExpr{
			Operation: UnaryNegative,
			Operand:   p.primary(),
		}
	}

	return p.primary()
}

func (p *Parser) primary() Expr {
	switch tok := p.peek(); tok.Typ {
	case TokenOpenParentheses:
		return p.parenthesisedExpression()
	case TokenIdentifier:
		return p.identifier()
	}

	return p.literal()
}

func (p *Parser) parenthesisedExpression() Expr {
	if tok := p.next(); tok.Typ != TokenOpenParentheses {
		return p.errorf(tok.Loc, "expected opening parenthesis")
	}

	exp := p.expr()

	if tok := p.next(); tok.Typ != TokenCloseParentheses {
		return p.errorf(tok.Loc, "expected closing parenthesis")
	}

	return exp
}

func (p *Parser) identifier() Expr {
	tok := p.next()
	if tok.Typ != TokenIdentifier {
		return p.errorf(tok.Loc, "expected an varDeclExpr")
	}

	return &Identifier{
		Name: tok.Value,
	}
}

func (p *Parser) literal() Expr {
	switch tok := p.peek(); tok.Typ {
	case TokenNumber:
		return &LiteralExpr{
			Typ:   LiteralNumber,
			Value: p.next().Value,
		}
	case TokenString:
		return &LiteralExpr{
			Typ:   LiteralString,
			Value: p.next().Value,
		}
	default:
		p.next() // Skip errored token
		return p.errorf(tok.Loc, "invalid symbol '%s'", tok.Value)
	}
}
