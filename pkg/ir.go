package maqui

import (
	"fmt"
	"github.com/llir/llvm/ir/enum"
	"strconv"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type ValueLookup map[string]value.Value

func NewValueLookup() ValueLookup {
	return make(map[string]value.Value)
}

func (l ValueLookup) Inherit(t2 ValueLookup) {
	for k, v := range t2 {
		l.Set(k, v)
	}
}

func (l ValueLookup) Get(id string) value.Value {
	if val, ok := l[id]; ok {
		return val
	}

	// TODO: Handle gracefully
	// The semantic analyser should make sure this doesn't happen
	panic("undefined identifier: " + id)
}

func (l ValueLookup) Set(id string, val value.Value) {
	l[id] = val
}

type IRGenerator interface {
	Do() IR
}

type IR interface {
	// TODO
	fmt.Stringer
}

type LLVMIRBuilder struct {
	mod    *ir.Module
	values ValueLookup
}

func NewLLVMIRBuilder() *LLVMIRBuilder {
	builder := &LLVMIRBuilder{
		mod:    ir.NewModule(),
		values: NewValueLookup(),
	}

	defineBuiltins(builder)
	return builder
}

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

func isBlockExpr(expr Expr) bool {
	switch expr.(type) {
	case *IfExpr:
		return true
	default:
		return false
	}
}

func (b *LLVMIRBuilder) blocks(expr Expr, exit *ir.Block) []*ir.Block {
	switch e := expr.(type) {
	case *IfExpr:
		return b.ifBranch(e, exit)
	}

	return nil
}

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

func (b *LLVMIRBuilder) variableDecl(expr *VariableDecl) (value.Value, []ir.Instruction) {
	v, ins := b.recursiveLoad(expr.Value)
	b.values.Set(expr.Name, v)

	return v, ins
}

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

func (b *LLVMIRBuilder) loadLiteralInt(expr *LiteralExpr) (value.Value, []ir.Instruction) {
	v, err := strconv.ParseInt(expr.Value, 10, 32)
	if err != nil {
		// TODO: Handle gracefully
		panic(err)
	}

	c := constant.NewInt(types.I32, v)
	return c, []ir.Instruction{}
}

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

type LLVMGenerator struct {
	ast *AST
}

func NewLLVMGenerator(ast *AST) *LLVMGenerator {
	return &LLVMGenerator{
		ast: ast,
	}
}

func (g LLVMGenerator) Do() IR {
	builder := NewLLVMIRBuilder()
	for _, stmt := range g.ast.Statements {
		g.visit(builder, stmt)
	}

	return builder.mod
}

func (g LLVMGenerator) visit(b *LLVMIRBuilder, expr Expr) {
	switch e := expr.(type) {
	case *AnnotatedExpr:
		g.visit(b, e.Expr)
	case *FuncDecl:
		b.function(e)
	}
}
