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
			&ObjectLit{},
		),
	)
	if err != nil {
		panic(err)
	}

	source := `
		obj := {"name": "Alice", "age": 30}
val := getFromObjWithKey(obj, "name")
	`

	ast, err := parser.ParseString("", source)
	if err != nil {
		log.Fatal(err)
	}

	ctx := EvalProgram(ast)
	fmt.Println("val =", ctx.Variables["val"])
}
