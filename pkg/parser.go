package maqui

import "fmt"

type Parser struct {
	lexer *Lexer
	ast   *AST
	buf   *Token
}

func NewParser(lexer *Lexer) *Parser {
	return &Parser{
		lexer: lexer,
		ast: &AST{
			Filename: lexer.filename,
		},
	}
}

func (p *Parser) Run() (*AST, error) {
	go p.lexer.Run()

	for p.peek().Typ != TokenEOF {
		p.ast.Statements = append(p.ast.Statements, p.statement())
	}

	return p.ast, nil
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

	tok := <-p.lexer.Chan()
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
	return BadExpr{l, fmt.Sprintf(format, args...)}
}

// TODO Temporary, only for testing
func (p *Parser) variableDecl() *VariableDecl {
	name := p.expect(TokenIdentifier)
	if name == nil {
		p.errorf(nil, "expected function name")
	}

	if !p.consume(TokenDeclaration) {
		p.errorf(name.Loc, "bad variable declaration")
	}

	return &VariableDecl{
		Name:  name.Value,
		Value: p.expr(),
	}
}

// TODO Temporary, only for testing
func (p *Parser) funcCall() *FuncCall {
	name := p.expect(TokenIdentifier)
	if name == nil {
		p.errorf(nil, "expected function name")
	}

	if !p.consume(TokenOpenParentheses) {
		p.errorf(name.Loc, "bad function call")
	}

	id := p.expect(TokenIdentifier)
	if id == nil {
		p.errorf(nil, "bad function call")
	}

	if !p.consume(TokenCloseParentheses) {
		p.errorf(name.Loc, "bad function call")
	}

	return &FuncCall{
		Name: name.Value,
		Args: []Expr{Identifier{Name: id.Value}},
	}
}

func (p *Parser) statement() Expr {
	switch tok := p.peek(); tok.Typ {
	case TokenFunc:
		return p.funcDecl()
	case TokenIdentifier:
		// TODO Move to recursive descent; only for testing
		if tok.Value == "print" {
			return p.funcCall()
		}

		return p.variableDecl()

	default:
		return p.expr()
	}
}

func (p *Parser) funcDecl() *FuncDecl {
	start := p.next().Loc // func keyword

	name := p.expect(TokenIdentifier)
	if name == nil {
		p.errorf(start, "expected function name")
	}

	// TODO: Allow arguments
	if !p.consume(TokenOpenParentheses) || !p.consume(TokenCloseParentheses) {
		p.errorf(start, "bad function declaration")
	}

	return &FuncDecl{
		Name: name.Value,
		Body: p.blockStmt(),
	}
}

func (p *Parser) blockStmt() []Expr {
	if tok := p.expect(TokenOpenCurly); tok == nil {
		return []Expr{p.errorf(tok.Loc, "invalid block statement")}
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
	return p.additiveExpr()
}

func (p *Parser) additiveExpr() Expr {
	lhs := p.multiplicativeExpr()

	for true {
		if tok := p.peek(); tok.Typ == TokenPlus || tok.Typ == TokenMinus {
			// Chained operands (for example 1 - 3 + 1). Go over the operand and nest
			p.next()

			rhs := p.additiveExpr()
			lhs = BinaryExpr{
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
	if p.check(TokenOpenParentheses) {
		p.next()

		exp := p.expr()

		if tok := p.next(); tok.Typ != TokenCloseParentheses {
			return p.errorf(tok.Loc, "expected closing parenthesis")
		}

		return exp
	}

	return p.literal()
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
		return p.errorf(tok.Loc, "invalid literal")
	}
}
