package maqui

import (
	"fmt"
	"strings"
)

type SemanticAnalyser interface {
	Define(*SymbolTable)
	Do() *AST
}

type ContextAnalyzer struct {
	filename string
	parser   SyntacticAnalyzer

	cache   []Expr
	live    bool
	started bool
	index   int
}

func NewContextAnalyser(parser SyntacticAnalyzer) *ContextAnalyzer {
	return &ContextAnalyzer{
		filename: parser.GetFilename(),
		parser:   parser,
		live:     true,
	}
}

func (c *ContextAnalyzer) Define(scope *SymbolTable) {
	c.reset()

	for {
		expr := c.get()
		if expr == nil {
			break
		}

		if e, isVarDef := expr.(*VariableDecl); isVarDef {
			scope.Add(e.Name, c.resolve(scope, e.Value))
		}

		if e, isFuncDef := expr.(*FuncDecl); isFuncDef {
			c.addFunction(scope, e)
		}
	}
}

func (c *ContextAnalyzer) Do(global *SymbolTable) *AST {
	c.reset()

	ast := &AST{
		Global:   global,
		Filename: c.filename,
	}

	for {
		expr := c.get()
		if expr == nil {
			break
		}

		stab := c.analyze(*ast.Global.Copy(), expr)
		ast.Statements = append(ast.Statements, &AnnotatedExpr{
			Stab: stab.Copy(),
			Expr: expr,
		})

		for _, err := range stab.Errors {
			isDuplicate := false
			for _, err2 := range ast.Errors {
				if err == err2 {
					isDuplicate = true
					break
				}
			}

			if !isDuplicate {
				ast.Errors = append(ast.Errors, err)
			}
		}
	}

	return ast
}

func (c *ContextAnalyzer) get() Expr {
	if c.live {
		if !c.started {
			go c.parser.Do()
			c.started = true
		}

		expr := c.parser.Get()
		_, ok := expr.(*EOS)
		if ok {
			c.live = false
			return nil
		}

		c.cache = append(c.cache, expr)
		return expr
	}

	if c.index >= len(c.cache) {
		return nil
	}

	expr := c.cache[c.index]
	c.index++
	return expr
}

func (c *ContextAnalyzer) reset() {
	c.index = 0
}

func (c *ContextAnalyzer) analyze(stab SymbolTable, expr Expr) SymbolTable {
	switch e := expr.(type) {
	case *BadExpr:
		stab.AddError(&BadExprError{
			Loc:  e.GetLocation(),
			Expr: e,
		})
		return stab
	case *FuncDecl:
		c.addFunction(&stab, e)
		for _, child := range e.Body {
			stab.Merge(c.analyze(stab, child))
		}

		return stab
	case *VariableDecl:
		t := c.resolve(&stab, e.Value)
		stab.Add(e.Name, t)
		e.ResolvedType = t
	case *FuncCall:
		if stab.Get(e.Name) == nil {
			stab.AddError(&UndefinedError{
				Loc:  e.GetLocation(),
				Name: e.Name,
			})

			break
		}

		for _, arg := range e.Args {
			e.ResolvedTypes = append(e.ResolvedTypes, c.resolve(&stab, arg))
			// TODO See if arguments match
		}
	case *Identifier:
		if stab.Get(e.Name) == nil {
			stab.AddError(&UndefinedError{
				Loc:  e.GetLocation(),
				Name: e.Name,
			})
		}
	case *BinaryExpr:
		c.resolve(&stab, e)

	case *UnaryExpr:
		c.resolve(&stab, e)
	}

	return stab
}

func (c *ContextAnalyzer) resolve(stab *SymbolTable, expr Expr) Type {
	switch e := expr.(type) {
	case *BadExpr:
		stab.AddError(&BadExprError{
			Loc:  e.GetLocation(),
			Expr: e,
		})
		return &TypeErr{TypeErrBadExpression}
	case *Identifier:
		if t := stab.Get(e.Name); t != nil {
			return t
		}

		stab.AddError(&UndefinedError{
			Loc:  e.GetLocation(),
			Name: e.Name,
		})

		return &TypeErr{TypeErrUndefined}
	case *BinaryExpr:
		t1 := c.resolve(stab, e.Op1)
		t2 := c.resolve(stab, e.Op2)

		if c.isErrorType(t1) {
			// Error already logged by the type resolution
			return t1
		}

		if c.isErrorType(t2) {
			// Error already logged by the type resolution
			return t2
		}

		if !t1.Equals(t2) {
			stab.AddError(&IncompatibleTypesError{
				Loc:   e.GetLocation(),
				Type1: t1,
				Type2: t2,
			})

			return &TypeErr{TypeErrIncompatible}
		}

		if !c.isOpDefined(t1, e.Operation) {
			stab.AddError(&UndefinedOperationError{
				Loc:  e.GetLocation(),
				Type: t1,
				Op:   e.Operation,
			})

			return &TypeErr{TypeErrBadOp}
		}

		return t1
	case *UnaryExpr:
		if t, isBasicType := c.resolve(stab, e.Operand).(*BasicType); isBasicType && t.Typ != "int" {
			stab.AddError(&UndefinedUnitaryError{
				Loc:  e.GetLocation(),
				Type: t,
				Op:   e.Operation,
			})

			return &TypeErr{TypeErrBadOp}
		} else {
			return t
		}

	case *LiteralExpr:
		switch e.Typ {
		case LiteralString:
			return &BasicType{"string"}
		case LiteralNumber:
			return &BasicType{"int"}
		default:
			return &TypeErr{"unimplemented"} // TODO Log error
		}
	}

	return &TypeErr{"unknown"}
}

func (c *ContextAnalyzer) addFunction(stab *SymbolTable, e *FuncDecl) {
	entry := &FuncType{}
	// TODO Add arguments and returns

	stab.Add(e.Name, entry)
}

func (c *ContextAnalyzer) isOpDefined(t Type, op BinaryOp) bool {
	if _, isFunc := t.(*FuncType); isFunc {
		return false
	}

	if t, isBasic := t.(*BasicType); isBasic {
		if t.Typ == "string" && op != BinaryAddition {
			return false
		}
	}

	return true
}

func (c *ContextAnalyzer) isErrorType(t Type) bool {
	if _, isErr := t.(*TypeErr); isErr {
		return true
	}

	return false
}

type Type interface {
	String() string
	Equals(t2 Type) bool
}

type TypeErr struct {
	Reason string
}

const (
	TypeErrUndefined     = "undefined"
	TypeErrBadExpression = "bad expr"
	TypeErrIncompatible  = "incompatible"
	TypeErrBadOp         = "bad op"
)

func (t *TypeErr) String() string {
	return "~error:" + t.Reason
}

func (t *TypeErr) Equals(_ Type) bool {
	return false
}

type AnyType struct{}

func (t *AnyType) String() string {
	return "~any"
}

func (t *AnyType) Equals(t2 Type) bool {
	if _, isErr := t2.(*TypeErr); isErr {
		return false
	}

	return true
}

type BasicType struct {
	Typ string
}

func (t *BasicType) String() string {
	return t.Typ
}

func (t *BasicType) Equals(t2 Type) bool {
	if typ, ok := t2.(*BasicType); ok {
		return t.Typ == typ.Typ
	}

	return false
}

type ArgumentType struct {
	Name string
	Type Type
}

func (t *ArgumentType) String() string {
	return t.Type.String()
}

func (t *ArgumentType) Equals(t2 Type) bool {
	if typ, ok := t2.(*ArgumentType); ok {
		return t.Name == typ.Name && t.Type.Equals(typ.Type)
	}

	return false
}

type FuncType struct {
	Args    []*ArgumentType
	Returns []*BasicType
}

func (t *FuncType) String() string {
	var str strings.Builder
	str.WriteString("func(")

	for i, arg := range t.Args {
		str.WriteString(arg.String())

		if i != len(t.Args)-1 {
			str.WriteString(", ")
		}
	}
	str.WriteString(") ")

	for i, ret := range t.Returns {
		str.WriteString(ret.String())

		if i != len(t.Returns)-1 {
			str.WriteString(", ")
		}
	}

	return str.String()
}

func (t *FuncType) Equals(t2 Type) bool {
	if typ, ok := t2.(*FuncType); ok {
		for i, arg := range t.Args {
			if i >= len(typ.Args) {
				return false
			}

			if !arg.Equals(typ.Args[i]) {
				return false
			}
		}

		for i, ret := range t.Returns {
			if i >= len(typ.Returns) {
				return false
			}

			if !ret.Equals(typ.Returns[i]) {
				return false
			}
		}

		return true
	}

	return false
}

type CompileError interface {
	fmt.Stringer
}

type BadExprError struct {
	Loc  *Location
	Expr *BadExpr
}

func (e BadExprError) String() string {
	return fmt.Sprintf("%s bad expression: %s", e.Loc, e.Expr.Error)
}

type UndefinedError struct {
	Loc  *Location
	Name string
}

func (e UndefinedError) String() string {
	return fmt.Sprintf("%s undefined: %s", e.Loc, e.Name)
}

type IncompatibleTypesError struct {
	Loc   *Location
	Type1 Type
	Type2 Type
}

func (e IncompatibleTypesError) String() string {
	return fmt.Sprintf("%s incompatible types: '%s' and '%s'", e.Loc, e.Type1, e.Type2)
}

type UndefinedOperationError struct {
	Loc  *Location
	Type Type
	Op   BinaryOp
}

func (e UndefinedOperationError) String() string {
	return fmt.Sprintf("%s undefined operation: '%s' has no operand '%s'", e.Loc, e.Type, e.Op)
}

type UndefinedUnitaryError struct {
	Loc  *Location
	Type Type
	Op   UnaryOp
}

func (e UndefinedUnitaryError) String() string {
	return fmt.Sprintf("%s undefined operation: '%s' has no operand '%s'", e.Loc, e.Type, e.Op)
}

type SymbolTable struct {
	Entries map[string]Type
	Errors  []CompileError
}

func NewGlobalSymbolTable() *SymbolTable {
	return &SymbolTable{
		Entries: map[string]Type{
			"print": &FuncType{
				Args: []*ArgumentType{
					{
						Name: "v",
						Type: &AnyType{},
					},
				},
				Returns: nil,
			},
		},
	}
}

func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		Entries: make(map[string]Type),
	}
}

func (t *SymbolTable) Add(name string, typ Type) {
	t.Entries[name] = typ
}

func (t *SymbolTable) Get(name string) Type {
	typ, contains := t.Entries[name]
	if !contains {
		return nil
	}

	return typ
}

func (t *SymbolTable) Merge(t2 SymbolTable) {
	for key, typ2 := range t2.Entries {
		t.Entries[key] = typ2
	}

	for _, err := range t2.Errors {
		t.Errors = append(t.Errors, err)
	}
}

func (t *SymbolTable) Copy() *SymbolTable {
	t2 := NewSymbolTable()

	if t.Errors != nil {
		t2.Errors = make([]CompileError, len(t.Errors))
		copy(t2.Errors, t.Errors)
	}

	for k, v := range t.Entries {
		t2.Entries[k] = v
	}

	return t2
}

func (t *SymbolTable) AddError(err CompileError) {
	t.Errors = append(t.Errors, err)
}
