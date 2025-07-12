package bvm

import (
	"fmt"
	"log"

	"github.com/alecthomas/participle/v2"
)

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
		),
	)
	if err != nil {
		panic(err)
	}

	source := `
		result := 5
		hash := D256("mydata", 2, 5)
	`

	ast, err := parser.ParseString("", source)
	if err != nil {
		log.Fatal(err)
	}

	ctx := EvalProgram(ast)
	fmt.Println("result =", ctx.Variables["result"])
	fmt.Println("hash =", ctx.Variables["hash"])
}
