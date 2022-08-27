package maqui

import "strings"

type SemanticAnalyser interface {
	Do() *AST
}

type ContextAnalyzer struct {
	filename string
	parser   SyntacticAnalyzer
}

func NewContextAnalyser(parser SyntacticAnalyzer) *ContextAnalyzer {
	return &ContextAnalyzer{
		filename: parser.GetFilename(),
		parser:   parser,
	}
}

func (c *ContextAnalyzer) Do() *AST {
	go c.parser.Do()

	ast := &AST{Global: NewGlobalSymbolTable()}
	for {
		expr := c.parser.Get()
		_, ok := expr.(*EOS)
		if ok {
			// End-of-stream
			break
		}

		stab := c.analyze(*ast.Global.Copy(), expr)
		if e, isVarDef := expr.(*VariableDecl); isVarDef {
			ast.Global.Add(e.Name, stab.Get(e.Name))
		}

		if e, isFuncDef := expr.(*FuncDecl); isFuncDef {
			ast.Global.Add(e.Name, stab.Get(e.Name))
		}

		ast.Statements = append(ast.Statements, AnnotatedExpr{
			Stab: &stab,
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

func (c *ContextAnalyzer) analyze(stab SymbolTable, expr Expr) SymbolTable {
	switch e := expr.(type) {
	case *BadExpr:
		stab.AddError(&BadExprError{e})
		return stab
	case *FuncDecl:
		c.addFunction(&stab, e)
		for _, child := range e.Body {
			stab.Merge(c.analyze(stab, child))
		}

		return stab
	case *VariableDecl:
		t := c.resolve(&stab, e.Value)
		if t == nil {
			// Error already logged by the type resolution
			break
		}

		stab.Add(e.Name, t)
		e.ResolvedType = t
	case *FuncCall:
		if stab.Get(e.Name) == nil {
			stab.AddError(&UndefinedError{
				Name: e.Name,
				Loc:  nil, // TODO
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
				Name: e.Name,
				Loc:  nil, // TODO
			})
		}
	case *BinaryExpr:
		c.resolve(&stab, e)

	case *UnaryExpr:
		c.resolve(&stab, e)
	}

	return stab
}

func (c *ContextAnalyzer) resolve(stab *SymbolTable, expr Expr) TypeInfo {
	switch e := expr.(type) {
	case *BadExpr:
		stab.AddError(&BadExprError{e})
		return &ErrorType{}
	case *Identifier:
		if t := stab.Get(e.Name); t != nil {
			return t
		}

		stab.AddError(&UndefinedError{
			Name: e.Name,
			Loc:  nil, // TODO
		})

		return &ErrorType{}
	case *BinaryExpr:
		t1 := c.resolve(stab, e.Op1)
		t2 := c.resolve(stab, e.Op2)

		if t1 == nil || t2 == nil || c.isErrorType(t1) || c.isErrorType(t2) {
			// Error already logged by the type resolution
			return &ErrorType{}
		}

		if !t1.Equals(t2) {
			stab.AddError(&IncompatibleTypesError{
				Type1: t1,
				Type2: t2,
				Loc:   nil, // TODO
			})

			return &ErrorType{}
		}

		if !c.isOpDefined(t1, e.Operation) {
			stab.AddError(&UndefinedOperationError{
				Type: t1,
				Op:   e.Operation,
				Loc:  nil, // TODO
			})

			return &ErrorType{}
		}

		return t1
	case *UnaryExpr:
		if t, isBasicType := c.resolve(stab, e.Operand).(*BasicType); isBasicType && t.Typ != "int" {
			stab.AddError(&UndefinedUnitaryError{
				Type: t,
				Op:   e.Operation,
				Loc:  nil, // TODO
			})

			return &ErrorType{}
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
			return nil // TODO Log error
		}
	}

	return &ErrorType{}
}

func (c *ContextAnalyzer) addFunction(stab *SymbolTable, e *FuncDecl) {
	entry := &FuncType{}
	// TODO Add arguments and returns

	stab.Add(e.Name, entry)
}

func (c *ContextAnalyzer) isOpDefined(t TypeInfo, op BinaryOp) bool {
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

func (c *ContextAnalyzer) isErrorType(t TypeInfo) bool {
	if _, isErr := t.(*ErrorType); isErr {
		return true
	}

	return false
}

type TypeInfo interface {
	String() string
	Equals(t2 TypeInfo) bool
}
type ErrorType struct{}

func (t *ErrorType) String() string {
	return "~error"
}

func (t *ErrorType) Equals(_ TypeInfo) bool {
	return false
}

type BasicType struct {
	Typ string
}

func (t *BasicType) String() string {
	return t.Typ
}

func (t *BasicType) Equals(t2 TypeInfo) bool {
	if typ, ok := t2.(*BasicType); ok {
		return t.Typ == typ.Typ
	}

	return false
}

type ArgumentType struct {
	Name string
	Type *BasicType
}

func (t *ArgumentType) String() string {
	return t.Type.String()
}

func (t *ArgumentType) Equals(t2 TypeInfo) bool {
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

func (t *FuncType) Equals(t2 TypeInfo) bool {
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

type CompileError interface{}

type BadExprError struct {
	Expr *BadExpr
}

type UndefinedError struct {
	Name string
	Loc  *Location
}

type IncompatibleTypesError struct {
	Type1 TypeInfo
	Type2 TypeInfo
	Loc   *Location
}

type UndefinedOperationError struct {
	Type TypeInfo
	Op   BinaryOp
	Loc  *Location
}

type UndefinedUnitaryError struct {
	Type TypeInfo
	Op   UnaryOp
	Loc  *Location
}

type SymbolTable struct {
	Entries map[string]TypeInfo
	Errors  []CompileError
}

func NewGlobalSymbolTable() *SymbolTable {
	return &SymbolTable{
		Entries: make(map[string]TypeInfo),
	}
}

func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		Entries: make(map[string]TypeInfo),
	}
}

func (t *SymbolTable) Add(name string, typ TypeInfo) {
	t.Entries[name] = typ
}

func (t *SymbolTable) Get(name string) TypeInfo {
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
	copy(t.Errors, t2.Errors)

	for k, v := range t.Entries {
		t2.Entries[k] = v
	}

	return t2
}

func (t *SymbolTable) AddError(err CompileError) {
	t.Errors = append(t.Errors, err)
}
