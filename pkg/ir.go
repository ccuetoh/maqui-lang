package maqui

import (
	"fmt"
	"strconv"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

// ValueLookup aliases a map[string]value.Value. ValueLookup is used to store the IR value references for the IDs while
// building the IR code.
type ValueLookup map[string]value.Value

// NewValueLookup creates a new empty ValueLookup
func NewValueLookup() ValueLookup {
	return make(map[string]value.Value)
}

// Inherit sets all keys and values from a ValueLookup to this ValueLookup. Priority is given to incoming keys, so if
// a key already exists in t1, it will be replaced by the value of that key in t2.
func (l ValueLookup) Inherit(t2 ValueLookup) {
	for k, v := range t2 {
		l.Set(k, v)
	}
}

// Get fetches the value mapped to the ID. If the ID is not present, it will panic.
func (l ValueLookup) Get(id string) value.Value {
	if val, ok := l[id]; ok {
		return val
	}

	// TODO: Handle gracefully
	// The semantic analyser should make sure this doesn't happen
	panic("undefined identifier: " + id)
}

// Set sets the value associated to an ID. If the ID already has a value set, it will be overriden.
func (l ValueLookup) Set(id string, val value.Value) {
	l[id] = val
}

// IRGenerator defines a single method Do, that creates an IR that turns a Maqui program with an immediate
// representation.
type IRGenerator interface {
	Do() IR
}

// IR is an immediate representation of a Maqui program. Currently, it just requires that the program is stringable.
type IR interface {
	// TODO
	fmt.Stringer
}

// LLVMGenerator is an IR generator that parses a Maqui AST into an LLVM compatible immediate representation.
type LLVMGenerator struct {
	// ast is the source for the IR. It's assumed valid, and will panic if not.
	ast *AST
}

// NewLLVMGenerator creates a new generator with the given AST.
func NewLLVMGenerator(ast *AST) *LLVMGenerator {
	return &LLVMGenerator{
		ast: ast,
	}
}

// Do builds the LLVM IR by recursively visiting all the nodes inside the AST. It assumes the AST is valid, and will
// panic if an unexpected statement is encountered.
func (g LLVMGenerator) Do() IR {
	builder := NewLLVMIRBuilder()
	for _, stmt := range g.ast.Statements {
		g.visit(builder, stmt)
	}

	return builder.mod
}

// visit takes an expression and decides what should be done to generate IR based on that expression's type.
func (g LLVMGenerator) visit(b *LLVMIRBuilder, expr Expr) {
	switch e := expr.(type) {
	case *AnnotatedExpr:
		g.visit(b, e.Expr)
	case *FuncDecl:
		b.function(e)
	}
}

// LLVMIRBuilder is a helper structure that simplifies the process of moving the IR module and values around. It
// implements some methods that take expressions and modify in-place the module based on the created IR.
type LLVMIRBuilder struct {
	mod    *ir.Module
	values ValueLookup
}

// NewLLVMIRBuilder creates a new builder with a module containing the builtin functions and empty values
func NewLLVMIRBuilder() *LLVMIRBuilder {
	builder := &LLVMIRBuilder{
		mod:    ir.NewModule(),
		values: NewValueLookup(),
	}

	defineBuiltins(builder)
	return builder
}

// function defines a function in the body. It will recursively parse the expressions inside the function. The function
// will be defined in the value table.
func (b *LLVMIRBuilder) function(expr *FuncDecl) {
	// TODO: Allow arguments and returns
	f := b.mod.NewFunc(expr.Name, types.Void)
	b.values.Set(expr.Name, f)

	block := f.NewBlock("")

	prevVals := b.values
	b.values = NewValueLookup()
	b.values.Inherit(prevVals)

	defer func() {
		b.values = prevVals
	}()

	for _, stmt := range expr.Body {
		if isBlockExpr(stmt) {
			continueBlock := ir.NewBlock("")

			blocks := b.blocks(stmt, continueBlock)
			block.NewBr(blocks[0])

			f.Blocks = append(f.Blocks, blocks...)
			f.Blocks = append(f.Blocks, continueBlock)

			block = continueBlock
			continue
		}

		block.Insts = append(block.Insts, b.instructions(stmt)...)
	}

	// TODO: Allow returns
	block.NewRet(nil)
}

// isBlockExpr returns true if the expression is a block expression (if, for, etc.).
func isBlockExpr(expr Expr) bool {
	switch expr.(type) {
	case *IfExpr:
		return true
	default:
		return false
	}
}

// blocks parses a block statement and returns a *ir.Block slice containing the processed block. A block statement
// might generate a multi-block IR since a block statement refers to a semantically constrained block (delimited by {})
// and not an execution branch, as the IR block does.
func (b *LLVMIRBuilder) blocks(expr Expr, exit *ir.Block) []*ir.Block {
	switch e := expr.(type) {
	case *IfExpr:
		return b.ifBranch(e, exit)
	}

	return nil
}

// instructions parses a statement into their corresponding sequence of instructions
func (b *LLVMIRBuilder) instructions(expr Expr) []ir.Instruction {
	switch e := expr.(type) {
	case *BinaryExpr:
		_, ins := b.binaryExpression(e)
		return ins
	case *VariableDecl:
		_, ins := b.variableDecl(e)
		return ins
	case *FuncCall:
		_, ins := b.functionCall(e)
		return ins
	}

	return []ir.Instruction{}
}

// ifBranch takes in an if expression and parses recursively it's content. As a product it will generate an IR block
// slice containing one block for each branch.
func (b *LLVMIRBuilder) ifBranch(expr *IfExpr, exit *ir.Block) []*ir.Block {
	block := ir.NewBlock("")

	condVal, condIns := b.recursiveLoad(expr.Condition)
	block.Insts = append(block.Insts, condIns...)

	trueBlock := ir.NewBlock("")
	for _, cExpr := range expr.Consequent {
		trueBlock.Insts = append(trueBlock.Insts, b.instructions(cExpr)...)
	}

	trueBlock.NewBr(exit)

	if len(expr.Else) == 0 {
		block.NewCondBr(condVal, trueBlock, exit)
		return []*ir.Block{block, trueBlock}
	}

	falseBlock := ir.NewBlock("")
	for _, eExpr := range expr.Else {
		falseBlock.Insts = append(falseBlock.Insts, b.instructions(eExpr)...)
	}

	falseBlock.NewBr(exit)

	block.NewCondBr(condVal, trueBlock, falseBlock)
	return []*ir.Block{block, trueBlock, falseBlock}
}

// recursiveLoad will load the value and instructions associated with an instruction expression. Blocks and other
// types of complex expressions are not parsable by recursiveLoad and will fail.
func (b *LLVMIRBuilder) recursiveLoad(expr Expr) (value.Value, []ir.Instruction) {
	switch e := expr.(type) {
	case *LiteralExpr:
		return b.loadLiteral(e)
	case *BinaryExpr:
		return b.binaryExpression(e)
	case *BooleanExpr:
		return b.booleanExpression(e)
	case *UnaryExpr:
		return b.unaryExpression(e)
	case *Identifier:
		return b.values.Get(e.Name), []ir.Instruction{}
	case *FuncCall:
		return b.functionCall(e)
	default:
		// TODO: Handle gracefully
		panic("not implemented")
	}
}

// binaryExpression loads a binary expression recursively, and returns its value and instructions
func (b *LLVMIRBuilder) binaryExpression(expr *BinaryExpr) (value.Value, []ir.Instruction) {
	v1, i1 := b.recursiveLoad(expr.Op1)
	v2, i2 := b.recursiveLoad(expr.Op2)
	ins := append(i1, i2...)

	switch expr.Operation {
	case BinaryAddition:
		op := ir.NewAdd(v1, v2)
		return op, append(ins, op)
	case BinarySubtraction:
		op := ir.NewSub(v1, v2)
		return op, append(ins, op)
	case BinaryMultiplication:
		op := ir.NewMul(v1, v2)
		return op, append(ins, op)
	case BinaryDivision:
		// TODO: Use fdiv and udiv when appropriate
		op := ir.NewSDiv(v1, v2)
		return op, append(ins, op)
	default:
		// TODO: Handle gracefully
		panic("unexpected binary op: " + expr.Operation)
	}
}

// booleanExpression loads a boolean expression recursively, and returns its value and instructions
func (b *LLVMIRBuilder) booleanExpression(expr *BooleanExpr) (value.Value, []ir.Instruction) {
	v1, i1 := b.recursiveLoad(expr.Op1)
	v2, i2 := b.recursiveLoad(expr.Op2)
	ins := append(i1, i2...)

	switch expr.Operation {
	case BooleanEquals:
		// TODO Add more data types
		op := ir.NewICmp(enum.IPredEQ, v1, v2)
		return op, append(ins, op)
	default:
		// TODO: Handle gracefully
		panic("unexpected binary op: " + expr.Operation)
	}
}

// unaryExpression loads a unary expression recursively, and returns its value and instructions
func (b *LLVMIRBuilder) unaryExpression(expr *UnaryExpr) (value.Value, []ir.Instruction) {
	v, ins := b.recursiveLoad(expr.Operand)

	switch expr.Operation {
	case UnaryNegative:
		minusOne := constant.NewInt(types.I32, -1)
		op := ir.NewMul(v, minusOne)
		return op, append(ins, op)
	default:
		// TODO: Handle gracefully
		panic("unexpected unary op: " + expr.Operation)
	}
}

// variableDecl loads a variable declaration expression recursively, and returns its value and instructions
func (b *LLVMIRBuilder) variableDecl(expr *VariableDecl) (value.Value, []ir.Instruction) {
	v, ins := b.recursiveLoad(expr.Value)
	b.values.Set(expr.Name, v)

	return v, ins
}

// loadLiteral loads a literal declaration, and returns its value and instructions
func (b *LLVMIRBuilder) loadLiteral(expr *LiteralExpr) (value.Value, []ir.Instruction) {
	switch expr.Typ {
	case LiteralString:
		// TODO: Implement
		panic("not implemented")
	case LiteralNumber:
		return b.loadLiteralInt(expr)
	default:
		// TODO: Handle gracefully
		panic("unknown type")
	}
}

// loadLiteralInt loads a literal integer expression and returns its value and instructions
func (b *LLVMIRBuilder) loadLiteralInt(expr *LiteralExpr) (value.Value, []ir.Instruction) {
	v, err := strconv.ParseInt(expr.Value, 10, 32)
	if err != nil {
		// TODO: Handle gracefully
		panic(err)
	}

	c := constant.NewInt(types.I32, v)
	return c, []ir.Instruction{}
}

// functionCall loads a function call expression and returns its value and instructions
func (b *LLVMIRBuilder) functionCall(expr *FuncCall) (value.Value, []ir.Instruction) {
	var ins []ir.Instruction
	var callVals []value.Value
	for _, arg := range expr.Args {
		argVal, argIns := b.recursiveLoad(arg)

		ins = append(ins, argIns...)
		callVals = append(callVals, argVal)
	}

	call := ir.NewCall(b.values.Get(expr.Name), callVals...)
	ins = append(ins, call)

	// TODO: Implement function call returns
	return nil, ins
}
