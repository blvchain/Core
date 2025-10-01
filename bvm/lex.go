package bvm

import (
	"github.com/alecthomas/participle/v2/lexer"
)

var MainLex = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `//[^\n]*`},
	{Name: "LBrace", Pattern: `{`}, {Name: "RBrace", Pattern: `}`},
	{Name: "AssignOp", Pattern: `=`},
	{Name: "Operators", Pattern: `&&|\|\||==|!=|<=|>=|[-+*/%^=;:()<>!,\[\]]`},
	{Name: "Bool", Pattern: `true|false`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "String", Pattern: `"(\\"|[^"])*"`},
	{Name: "Int", Pattern: `[0-9]+`},
	{Name: "Whitespace", Pattern: `[ \t\n\r]+`},
})
