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
		participle.Elide("Whitespace", "Comment"),
		participle.Union[Term](
			&FuncCall{},
			&Number{},
			&Variable{},
			&StringLit{},
			&BoolLit{},
			&ArrayLit{},
			&ObjectLit{},
			&NotTerm{},
		),
	)
	if err != nil {
		panic(err)
	}

	source := `
		func myFunc(a array) string {
			var output = 0

			for var i = 0; i < len(a); i = i + 1  {
				output = output + i
			}

			return output
		}
		var sum = myFunc([1, 2, 3])
	`
	ast, err := parser.ParseString("", source)
	if err != nil {
		log.Fatal(err)
	}

	ctx := EvalProgram(ast)
	sum, _ := ctx.Get("sum")
	fmt.Println("sum =", sum)
}
