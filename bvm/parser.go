package bvm

import "blvchain/core/utils"

type Context struct {
	Functions map[string]*FuncDef
	Variables map[string]any
}

func EvalProgram(prog *Program) *Context {
	ctx := &Context{
		Functions: map[string]*FuncDef{},
		Variables: map[string]any{},
	}
	for _, stmt := range prog.Stmts {
		if stmt.FuncDef != nil {
			ctx.Functions[stmt.FuncDef.Name] = stmt.FuncDef
		} else if stmt.Assign != nil {
			val := EvalExpr(stmt.Assign.Expr, ctx)
			ctx.Variables[stmt.Assign.Var] = val
		}
	}
	return ctx
}

func EvalExpr(expr *Expr, ctx *Context) any {
	left := EvalTerm(expr.Left, ctx)
	if expr.Op != nil && expr.Right != nil {
		right := EvalTerm(expr.Right, ctx)
		return left.(int) + right.(int)
	}
	return left
}

func EvalTerm(term Term, ctx *Context) any {
	switch t := term.(type) {
	case *Number:
		return t.Value
	case *Variable:
		return ctx.Variables[t.Name]
	case *StringLit:
		return t.Value[1 : len(t.Value)-1] // remove quotes
	case *FuncCall:
		return EvalFuncCall(t, ctx)
	default:
		panic("unknown term")
	}
}

func EvalFuncCall(fc *FuncCall, ctx *Context) any {
	// Built-in
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

	if fc.Name == "D512C" {
		str := EvalExpr(fc.Args[0], ctx).(string)
		path := EvalExpr(fc.Args[1], ctx).(string)
		res := utils.D512C(str, path)
		return res.String
	}

	if fc.Name == "D256C" {
		str := EvalExpr(fc.Args[0], ctx).(string)
		path := EvalExpr(fc.Args[1], ctx).(string)
		res := utils.D256C(str, path)
		return res.String
	}

	// User-defined
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
	default:
		panic("unsupported operator")
	}
}
