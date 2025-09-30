package bvm

import (
	"fmt"
	"log"

	"github.com/alecthomas/participle/v2"
)

func (ctx *Context) Get(name string) (any, bool) {
	for i := len(ctx.VarStack) - 1; i >= 0; i-- {
		if v, ok := ctx.VarStack[i][name]; ok {
			return v, true
		}
	}
	return nil, false
}

func BVM() {
	parser, err := participle.Build[Program](
		participle.Lexer(MainLex),
		participle.UseLookahead(2),
		participle.Elide("Whitespace"),
		participle.Union[Term](
			&FuncCall{},
			&Number{},
			&Variable{},
			&StringLit{},
			&BoolLit{},
			&ArrayLit{},
			&ObjectLit{},
		),
	)
	if err != nil {
		panic(err)
	}

	source := `
		func myFunc(a string, b string, c string) string {
			return a + b + c
		}
		var sum = myFunc("1", "2", "3")
	`

	ast, err := parser.ParseString("", source)
	if err != nil {
		log.Fatal(err)
	}

	ctx := EvalProgram(ast)
	sum, _ := ctx.Get("sum")
	fmt.Println("sum =", sum)
}
