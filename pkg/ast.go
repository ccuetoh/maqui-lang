package maqui

type AST struct {
	Filename   string
	Statements []Expr
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
	Name  string
	Value Expr
}

type FuncCall struct {
	Name string
	Args []Expr
}

type Identifier struct {
	Name string
}

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
