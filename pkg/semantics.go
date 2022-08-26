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

	ast := &AST{}
	global := NewSymbolTable()
	for {
		expr := c.parser.Get()
		_, ok := expr.(*EOS)
		if ok {
			// End-of-stream
			break
		}

		stab := c.analyze(global, expr)
		if e, isVarDef := expr.(*VariableDecl); isVarDef {
			global.add(e.Name, stab.get(e.Name))
		}

		if e, isFuncDef := expr.(*FuncDecl); isFuncDef {
			global.add(e.Name, stab.get(e.Name))
		}

		ast.Statements = append(ast.Statements, expr)
		for _, err := range stab.errors {
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
		stab.addError(&BadExprError{e})
		return stab
	case *FuncDecl:
		c.addFunction(&stab, e)
		for _, child := range e.Body {
			stab.merge(c.analyze(stab, child))
		}

		return stab
	case *VariableDecl:
		t := c.resolve(&stab, e.Value)

		stab.add(e.Name, t)
		e.ResolvedType = t
	case *FuncCall:
		if stab.get(e.Name) == nil {
			stab.addError(&UndefinedError{
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
		if stab.get(e.Name) == nil {
			stab.addError(&UndefinedError{
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
		stab.addError(&BadExprError{e})
		return nil
	case *Identifier:
		if t := stab.get(e.Name); t != nil {
			return t
		}

		stab.addError(&UndefinedError{
			Name: e.Name,
			Loc:  nil, // TODO
		})

		return nil
	case *BinaryExpr:
		t1 := c.resolve(stab, e.Op1)
		t2 := c.resolve(stab, e.Op2)

		if t1 == nil || t2 == nil {
			// Error already logged by the type resolution
			break
		}

		if !t1.Equals(t2) {
			stab.addError(&IncompatibleTypesError{
				Type1: t1,
				Type2: t2,
				Loc:   nil, // TODO
			})

			break
		}

		if !c.isOpDefined(t1, e.Operation) {
			stab.addError(&UndefinedOperationError{
				Type: t1,
				Op:   e.Operation,
				Loc:  nil, // TODO
			})
		}

		return t1
	case *UnaryExpr:
		if t, isBasicType := c.resolve(stab, e.Operand).(*BasicType); isBasicType && t.Typ != "int" {
			stab.addError(&UndefinedUnitaryError{
				Type: t,
				Op:   e.Operation,
				Loc:  nil, // TODO
			})
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
			return nil
		}
	}

	return nil
}

func (c *ContextAnalyzer) addFunction(stab *SymbolTable, e *FuncDecl) {
	entry := &FuncType{}
	// TODO Add arguments and returns

	stab.add(e.Name, entry)
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

type TypeInfo interface {
	String() string
	Equals(t2 TypeInfo) bool
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
	entries map[string]TypeInfo
	errors  []CompileError
}

func NewSymbolTable() SymbolTable {
	return SymbolTable{
		entries: make(map[string]TypeInfo),
	}
}

func (t *SymbolTable) add(name string, typ TypeInfo) {
	t.entries[name] = typ
}

func (t *SymbolTable) get(name string) TypeInfo {
	typ, contains := t.entries[name]
	if !contains {
		return nil
	}

	return typ
}

func (t *SymbolTable) merge(t2 SymbolTable) {
	for key, typ2 := range t2.entries {
		t.entries[key] = typ2
	}

	for _, err := range t2.errors {
		t.errors = append(t.errors, err)
	}
}

func (t *SymbolTable) addError(err CompileError) {
	t.errors = append(t.errors, err)
}
