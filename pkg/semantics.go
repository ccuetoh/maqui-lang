package maqui

import (
	"fmt"
	"strings"
)

// SemanticAnalyser defines the expected behavior of a semantic analyzer. The semantic analyzer should use context-aware
// strategies to build a symbol table and resolve reference in the code.
type SemanticAnalyser interface {
	Define(*SymbolTable)
	Do() *AST
}

// ContextAnalyzer is the default Maqui semantic analyser. It goes over the expressions provided by the Parses to build
// an AST annotated with type data and symbol tables. A ContextAnalyzer is stateful and shouldn't be used for more than
// one file.
type ContextAnalyzer struct {
	// filename name of the file that provided the source for the expressions
	filename string
	// parser is the expression provider
	parser SyntacticAnalyzer

	// cache hold expressions already visited to be able to go over them again when needed
	cache []Expr
	// live is true if the parser is providing expressions as the ContextAnalyzer goes forward. It starts as true
	// and is set to false once the end of the end of the stream is reached. If live is set to false the ContextAnalyzer
	// will feed from the cached expressions.
	live bool
	// started is set to true once the underlying parser is ran.
	started bool
	// index holds the current position of the ContextAnalyzer, but will only be used once live is set to false and the
	// ContextAnalyzer is working offline.
	index int
}

// NewContextAnalyser creates a *ContextAnalyzer that takes expressions from the parser.
func NewContextAnalyser(parser SyntacticAnalyzer) *ContextAnalyzer {
	return &ContextAnalyzer{
		filename: parser.GetFilename(),
		parser:   parser,
		live:     true,
	}
}

// DefineInto does a full but shallow pass over the expressions and brings the file definitions inside the provided scope.
// It won't delve into nested definitions like functions.
func (c *ContextAnalyzer) DefineInto(scope *SymbolTable) {
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

// Do takes in a global symbol table and builds an annotated *AST. It delves into nested definitions and builds the
// corresponding symbol tables as well.
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

		// Prevent duplicated entries caused by cascading errors
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

// get fetches the next available expression. If the ContextAnalyzer is running on live mode (that is, the first run) it
// will fetch the expressions directly from the parser and store them in cache. Once the parser stream is exhausted the
// ContextAnalyzer can be reset to use the cache in an offline way to go over the expressions again.
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

// reset sets the position (index) to the start
func (c *ContextAnalyzer) reset() {
	c.index = 0
}

// analyze takes in a symbol table and an expression and returns an updated symbol table with the new definition when
// appropriate. If the expression is a nested expression it will recursively analyze the expressions therein. If one or
// more invalid definitions are encountered they will be added to the SymbolTable's error slice.
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
			stab.Import(c.analyze(stab, child))
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

// resolve will try to resolve the type of an expression. It takes in the context's symbol table and it might be used
// to get other definition's types. If an error or an unexpected expression is encountered, an error will be added to
// the symbol table and a *TypeErr will be returned.
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

// addFunction is a shorthand to create a *FuncType entry inside the system table
func (c *ContextAnalyzer) addFunction(stab *SymbolTable, e *FuncDecl) {
	entry := &FuncType{}
	// TODO Add arguments and returns

	stab.Add(e.Name, entry)
}

// isOpDefined returns true if an operation is defined for the type. For example, subtraction is defined for numbers
// (1-2), but not for strings ("foo"-"bar").
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

// isErrorType returns true if the provided type is a *TypeErr, and false otherwise
func (c *ContextAnalyzer) isErrorType(t Type) bool {
	if _, isErr := t.(*TypeErr); isErr {
		return true
	}

	return false
}

// Type defines the behavior of type. It should at a minimum be stringable and comparable.
type Type interface {
	fmt.Stringer
	Equals(t2 Type) bool
}

// TypeErr is special type that represents an error while resolving a type.
type TypeErr struct {
	// Reason contains an explanatory message of the error
	Reason string
}

const (
	// TypeErrUndefined is used when an identifier was used but not defined
	TypeErrUndefined = "undefined"
	// TypeErrBadExpression occurs when a bad expression tries to get type-resolved
	TypeErrBadExpression = "bad expr"
	// TypeErrIncompatible occurs when a binary operations is attempted between two non-similar types
	TypeErrIncompatible = "incompatible"
	// TypeErrBadOp occurs when a binary operation is attempted between operands of same type that have an undefined
	// operation. For example "foo"-"bar".
	TypeErrBadOp = "bad op"
)

func (t *TypeErr) String() string {
	return "~error:" + t.Reason
}

func (t *TypeErr) Equals(_ Type) bool {
	return false
}

func (t *TypeErr) Error() string {
	return t.Reason
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

// SymbolTable keeps a list of definitions and types inside a code context. It also hold all related errors generated
// during its creation.
type SymbolTable struct {
	// Entries maps an identifier to its Type
	Entries map[string]Type
	// Errors hold all errors produced while creating the symbol table.
	Errors []CompileError
}

// NewGlobalSymbolTable crates a new symbol table with global definitions prepopulated
func NewGlobalSymbolTable() *SymbolTable {
	// TODO: Move the creation of global definitions elsewhere
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

// NewSymbolTable creates a new empty symbol table
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		Entries: make(map[string]Type),
	}
}

// Add adds an entry to the symbol table. If an entry with the same name already exists, it will be replaced.
func (t *SymbolTable) Add(name string, typ Type) {
	t.Entries[name] = typ
}

// Get fetches the Type of the entry. If the entry is not present nil will be returned.
func (t *SymbolTable) Get(name string) Type {
	typ, contains := t.Entries[name]
	if !contains {
		return nil
	}

	return typ
}

// Import merges the provided symbol table into the current table. It copies entries and errors. If an entry with the
// same name already exists, it will be replaced. Priority is given to the incoming entry.
func (t *SymbolTable) Import(t2 SymbolTable) {
	for key, typ2 := range t2.Entries {
		t.Entries[key] = typ2
	}

	for _, err := range t2.Errors {
		t.Errors = append(t.Errors, err)
	}
}

// Copy creates a new table and copies all entries and errors into it
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

// AddError adds a new error to the table's error list
func (t *SymbolTable) AddError(err CompileError) {
	t.Errors = append(t.Errors, err)
}
