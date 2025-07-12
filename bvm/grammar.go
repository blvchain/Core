package bvm

// Main grammar
type Program struct {
	Stmts []*Stmt `@@*`
}

type Stmt struct {
	FuncDef *FuncDef `  @@`
	Assign  *Assign  `| @@`
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
	Op    *string `[ @( "+" | "-" )`
	Right Term    `  @@ ]`
}

type Term interface {
	isTerm()
}

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
