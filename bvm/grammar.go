package bvm

// Main grammar
type Program struct {
	Stmts []*Stmt `@@*`
}

type Stmt struct {
	FuncDef *FuncDef `  @@`
	Assign  *Assign  `| @@`
	If      *IfStmt  `| @@`
}

type FuncDef struct {
	Func   string    `"func"`
	Name   string    `@Ident`
	Params []*Param  `"(" [ @@ { "," @@ } ] ")"`
	Ret    string    `"int"`
	Body   *FuncBody `"{" @@ "}"`
}

type Param struct {
	Name string `@Ident`
	Type string `@Ident`
}

type FuncBody struct {
	Return *ReturnStmt `@@`
}

type ReturnStmt struct {
	Return string `"return"`
	Left   *Expr  `@@`
}

type Assign struct {
	Var   string `@Ident`
	Equal string `@AssignOp`
	Expr  *Expr  `@@`
}

type Expr struct {
	Left  Term    `@@`
	Op    *string `[ @( "&&" | "||" | "==" | "!=" | "<=" | ">=" | "<" | ">" | "+" | "-" | "*" | "/" | "^" | "%" )`
	Right Term    `  @@ ]`
}
type Term interface {
	isTerm()
}

type NotTerm struct {
	Not  string `"!"`
	Term Term   `@@`
}

func (*NotTerm) isTerm() {}

type BoolLit struct {
	Value string `@Bool`
}

func (*BoolLit) isTerm() {}

type ArrayLit struct {
	Elements []*Expr `"[" [ @@ { "," @@ } ] "]"`
}

func (*ArrayLit) isTerm() {}

type FuncCall struct {
	Name string  `@Ident`
	Args []*Expr `"(" [ @@ { "," @@ } ] ")"`
}

func (*FuncCall) isTerm() {}

type Number struct {
	Value int `@Int`
}

func (*Number) isTerm() {}

type Variable struct {
	Name string `@Ident`
}

func (*Variable) isTerm() {}

type StringLit struct {
	Value string `@String`
}

func (*StringLit) isTerm() {}

type IfStmt struct {
	If        string   `"if"`
	Condition *Expr    `@@`
	Then      []*Stmt  `"{" @@* "}"`
	Else      *ElseBlk `[ @@ ]`
}

type ElseBlk struct {
	Else string  `"else"`
	Body []*Stmt `"{" @@* "}"`
}
