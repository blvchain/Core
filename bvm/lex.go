package bvm

import (
	"github.com/alecthomas/participle/v2/lexer"
)

var MainLex = lexer.MustSimple([]lexer.SimpleRule{
	{"Comment", `//[^\n]*`},
	{"AssignOp", `=`},
	{"Operators", `&&|\|\||==|!=|<=|>=|[-+*/%^=;:(){}<>!,\[\]]`},
	{"Bool", `true|false`},
	{"Ident", `[a-zA-Z_][a-zA-Z0-9_]*`},
	{"String", `"(\\"|[^"])*"`},
	{"Int", `[0-9]+`},
	{"Whitespace", `[ \t\n\r]+`},
})
