package bvm

// Main grammar
type Program struct {
	Stmts []*Stmt `@@*`
}

type Stmt struct {
	FuncDef *FuncDef    `  @@`
	Assign  *Assign     `| @@`
	If      *IfStmt     `| @@`
	For     *ForStmt    `| @@`
	Return  *ReturnStmt `| @@`
}

type FuncDef struct {
	Func   string    `"func"`
	Name   string    `@Ident`
	Params []*Param  `"(" [ @@ { "," @@ } ] ")"`
	Ret    string    `@Ident`
	Body   *FuncBody `"{" @@ "}"`
}

type Param struct {
	Name string `@Ident`
	Type string `@Ident`
}

type FuncBody struct {
	Stmts []*Stmt `@@*`
}

type ReturnStmt struct {
	Return string `"return"`
	Left   *Expr  `@@`
}

type Assign struct {
	VarKw *string `@"var"?`
	Name  string  `@Ident`
	Equal string  `@AssignOp`
	Expr  *Expr   `@@`
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

type ForStmt struct {
	For  string  `"for"`
	Init *Assign `@@ ";"` // initialization
	Cond *Expr   `@@ ";"` // condition
	Post *Assign `@@`     // post-expression
	Body []*Stmt `"{" @@* "}"`
}

type BoolLit struct {
	Value string `@Bool`
}

func (*BoolLit) isTerm() {}

type ArrayLit struct {
	Elements []*Expr `"[" [ @@ { "," @@ } ] "]"`
}

func (*ArrayLit) isTerm() {}

type ObjectLit struct {
	Pairs []*ObjectPair `"{" [ @@ { "," @@ } ] "}"`
}
type ObjectPair struct {
	Key   string `@String ":"`
	Value *Expr  `@@`
}

func (*ObjectLit) isTerm() {}

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
