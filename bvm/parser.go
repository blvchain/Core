package bvm

import (
	"blvchain/core/db"
	"blvchain/core/utils"
)

type Context struct {
	Functions map[string]*FuncDef
	Variables map[string]any
}

func EvalStmts(stmts []*Stmt, ctx *Context) {
	for _, stmt := range stmts {
		if stmt.FuncDef != nil {
			ctx.Functions[stmt.FuncDef.Name] = stmt.FuncDef
		} else if stmt.Assign != nil {
			val := EvalExpr(stmt.Assign.Expr, ctx)
			ctx.Variables[stmt.Assign.Var] = val
		} else if stmt.If != nil {
			cond := EvalExpr(stmt.If.Condition, ctx)
			if cond.(bool) {
				EvalStmts(stmt.If.Then, ctx)
			} else if stmt.If.Else != nil {
				EvalStmts(stmt.If.Else.Body, ctx)
			}
		}
	}
}

func EvalProgram(prog *Program) *Context {
	ctx := &Context{
		Functions: map[string]*FuncDef{},
		Variables: map[string]any{},
	}
	EvalStmts(prog.Stmts, ctx)
	return ctx
}

func EvalExpr(expr *Expr, ctx *Context) any {
	left := EvalTerm(expr.Left, ctx)
	if expr.Op != nil && expr.Right != nil {
		right := EvalTerm(expr.Right, ctx)
		switch *expr.Op {
		case "&&":
			return left.(bool) && right.(bool)
		case "||":
			return left.(bool) || right.(bool)
		case "+":
			return left.(int) + right.(int)
		case "-":
			return left.(int) - right.(int)
		case "*":
			return left.(int) * right.(int)
		case "/":
			return left.(int) / right.(int)
		case "==":
			return left == right
		case "!=":
			return left != right
		case "<":
			return left.(int) < right.(int)
		case "<=":
			return left.(int) <= right.(int)
		case ">":
			return left.(int) > right.(int)
		case ">=":
			return left.(int) >= right.(int)
		default:
			panic("unsupported operator: " + *expr.Op)
		}
	}
	return left
}

func EvalTerm(term Term, ctx *Context) any {
	switch t := term.(type) {
	case *Number:
		return t.Value
	case *StringLit:
		return t.Value[1 : len(t.Value)-1] // remove quotes
	case *FuncCall:
		return EvalFuncCall(t, ctx)
	case *NotTerm:
		return !EvalTerm(t.Term, ctx).(bool)
	case *BoolLit:
		return t.Value == "true"
	case *Variable:
		val, ok := ctx.Variables[t.Name]
		if !ok {
			panic("undefined variable: " + t.Name)
		}
		return val
	case *ArrayLit:
		arr := make([]any, len(t.Elements))
		for i, e := range t.Elements {
			arr[i] = EvalExpr(e, ctx)
		}
		return arr
	default:
		panic("unknown term")
	}
}

func EvalFuncCall(fc *FuncCall, ctx *Context) any {
	// # Built-in
	// ## Delium
	if fc.Name == "D256" {
		str := EvalExpr(fc.Args[0], ctx).(string)
		step := EvalExpr(fc.Args[1], ctx).(int)
		repeat := EvalExpr(fc.Args[2], ctx).(int)
		res := utils.D256(str, step, repeat)
		return res.String
	}

	if fc.Name == "D512" {
		str := EvalExpr(fc.Args[0], ctx).(string)
		step := EvalExpr(fc.Args[1], ctx).(int)
		repeat := EvalExpr(fc.Args[2], ctx).(int)
		res := utils.D512(str, step, repeat)
		return res.String
	}

	if fc.Name == "D256C" {
		str := EvalExpr(fc.Args[0], ctx).(string)
		path := EvalExpr(fc.Args[1], ctx).(string)
		res := utils.D256C(str, path)
		return res.String
	}

	if fc.Name == "D512C" {
		str := EvalExpr(fc.Args[0], ctx).(string)
		path := EvalExpr(fc.Args[1], ctx).(string)
		res := utils.D512C(str, path)
		return res.String
	}

	// ## Verification
	if fc.Name == "VerifySignature" {
		hexPublicKey := EvalExpr(fc.Args[0], ctx).(string)
		uid := EvalExpr(fc.Args[1], ctx).(string)
		message := EvalExpr(fc.Args[2], ctx).(string)
		hexSignature := EvalExpr(fc.Args[3], ctx).(string)
		res, _ := utils.Verify(hexPublicKey, uid, message, hexSignature)
		return res
	}

	// ## uid maker
	if fc.Name == "MakeUID" {
		pubkey_str := EvalExpr(fc.Args[0], ctx).(string)
		return utils.Make_UID(pubkey_str)
	}

	// ## get block data
	if fc.Name == "GetOneBlockDataWithBlockHash" {
		blockHash := EvalExpr(fc.Args[0], ctx).(string)
		var result db.Block
		err := db.FindOneBlock(blockHash, &result)
		if err != nil {
			return err
		}
		return result.BlockData
	}

	// # Helpers
	// ## Length
	if fc.Name == "length" {
		arg := EvalExpr(fc.Args[0], ctx)
		switch v := arg.(type) {
		case string:
			return len(v)
		case []any:
			return len(v)
		default:
			panic("length: unsupported type")
		}
	}

	// ## Get from array with index
	if fc.Name == "getFromArrWithIndex" {
		arr := EvalExpr(fc.Args[0], ctx)
		index := EvalExpr(fc.Args[1], ctx)
		switch a := arr.(type) {
		case []any:
			return a[index.(int)]
		case string:
			return string(a[index.(int)])
		default:
			panic("getFromArrWithIndex: unsupported type")
		}
	}

	// # User-defined
	fn := ctx.Functions[fc.Name]
	args := make(map[string]int)
	for i, param := range fn.Params {
		args[param.Name] = EvalExpr(fc.Args[i], ctx).(int)
	}
	return EvalFuncBody(fn.Body, args)
}

func EvalFuncBody(body *FuncBody, args map[string]int) int {
	left := args[body.Return.Left.Left.(*Variable).Name]
	right := args[body.Return.Left.Right.(*Variable).Name]
	switch *body.Return.Left.Op {
	case "+":
		return left + right
	case "-":
		return left - right
	case "*":
		return left * right
	case "/":
		return left / right
	case "^":
		return left ^ right
	case "%":
		return left % right
	default:
		panic("unsupported operator")
	}
}
