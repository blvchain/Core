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
			&BoolLit{},
			&ArrayLit{},
		),
	)
	if err != nil {
		panic(err)
	}

	source := `
		arr := [1, 2, 3, 4]
		l := getFromArrWithIndex(arr, 1)
	`

	ast, err := parser.ParseString("", source)
	if err != nil {
		log.Fatal(err)
	}

	ctx := EvalProgram(ast)
	fmt.Println("l =", ctx.Variables["l"])
}
