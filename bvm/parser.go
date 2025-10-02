package bvm

import (
	"blvchain/core/db"
	"blvchain/core/utils"
)

type Context struct {
	Functions map[string]*FuncDef
	VarStack  []map[string]any
	Variables map[string]interface{}
}

func (ctx *Context) pushScope() {
	ctx.VarStack = append(ctx.VarStack, map[string]any{})
}

func (ctx *Context) popScope() {
	ctx.VarStack = ctx.VarStack[:len(ctx.VarStack)-1]
}

func (ctx *Context) setVar(name string, val any) {
	ctx.VarStack[len(ctx.VarStack)-1][name] = val
}

func (ctx *Context) getVar(name string) (any, bool) {
	for i := len(ctx.VarStack) - 1; i >= 0; i-- {
		if v, ok := ctx.VarStack[i][name]; ok {
			return v, true
		}
	}
	return nil, false
}

func EvalStmts(stmts []*Stmt, ctx *Context) {
	for _, stmt := range stmts {
		switch {
		case stmt.FuncDef != nil:
			ctx.Functions[stmt.FuncDef.Name] = stmt.FuncDef

		case stmt.Assign != nil:
			val := EvalExpr(stmt.Assign.Expr, ctx)
			if stmt.Assign.VarKw != nil {
				// Declaration: always in current scope
				ctx.VarStack[len(ctx.VarStack)-1][stmt.Assign.Name] = val
			} else {
				// Re-assignment: must exist in any scope
				found := false
				for i := len(ctx.VarStack) - 1; i >= 0; i-- {
					if _, ok := ctx.VarStack[i][stmt.Assign.Name]; ok {
						ctx.VarStack[i][stmt.Assign.Name] = val
						found = true
						break
					}
				}
				if !found {
					panic("variable not declared: " + stmt.Assign.Name)
				}
			}

		case stmt.If != nil:
			ctx.pushScope()
			cond := EvalExpr(stmt.If.Condition, ctx)
			if cond.(bool) {
				EvalStmts(stmt.If.Then, ctx)
			} else if stmt.If.Else != nil {
				EvalStmts(stmt.If.Else.Body, ctx)
			}
			ctx.popScope()

		case stmt.For != nil:
			// Do NOT pushScope/popScope here!
			if stmt.For.Init != nil && stmt.For.Cond != nil && stmt.For.Post != nil {
				EvalStmts([]*Stmt{{Assign: stmt.For.Init}}, ctx)
				for {
					cond := EvalExpr(stmt.For.Cond, ctx)
					if !cond.(bool) {
						break
					}
					EvalStmts(stmt.For.Body, ctx) // use the same context!
					EvalStmts([]*Stmt{{Assign: stmt.For.Post}}, ctx)
				}
			}
		}
	}
}

func EvalProgram(prog *Program) *Context {
	ctx := &Context{
		Functions: map[string]*FuncDef{},
		VarStack:  []map[string]any{{}},
	}
	EvalStmts(prog.Stmts, ctx)
	return ctx
}

func EvalExpr(expr *Expr, ctx *Context) any {
	left := EvalTerm(expr.Left, ctx)
	if expr.Op != nil && expr.Right != nil {
		right := EvalExpr(expr.Right, ctx)
		switch *expr.Op {
		case "&&":
			return left.(bool) && right.(bool)
		case "||":
			return left.(bool) || right.(bool)
		case "+":
			switch l := left.(type) {
			case int:
				return l + right.(int)
			case string:
				return l + right.(string)
			default:
				panic("unsupported type for +")
			}
		case "-":
			switch l := left.(type) {
			case int:
				return l - right.(int)
			default:
				panic("unsupported type for -")
			}
		case "*":
			switch l := left.(type) {
			case int:
				return l * right.(int)
			case string:
				// Repeat string n times if right is int
				return repeatString(l, right.(int))
			default:
				panic("unsupported type for *")
			}
		case "/":
			switch l := left.(type) {
			case int:
				return l / right.(int)
			default:
				panic("unsupported type for /")
			}
		case "==":
			return left == right
		case "!=":
			return left != right
		case "<":
			switch l := left.(type) {
			case int:
				return l < right.(int)
			case string:
				return l < right.(string)
			default:
				panic("unsupported type for <")
			}
		case "<=":
			switch l := left.(type) {
			case int:
				return l <= right.(int)
			case string:
				return l <= right.(string)
			default:
				panic("unsupported type for <=")
			}
		case ">":
			switch l := left.(type) {
			case int:
				return l > right.(int)
			case string:
				return l > right.(string)
			default:
				panic("unsupported type for >")
			}
		case ">=":
			switch l := left.(type) {
			case int:
				return l >= right.(int)
			case string:
				return l >= right.(string)
			default:
				panic("unsupported type for >=")
			}
		case "^":
			switch l := left.(type) {
			case int:
				return l ^ right.(int)
			default:
				panic("unsupported type for ^")
			}
		case "%":
			switch l := left.(type) {
			case int:
				return l % right.(int)
			default:
				panic("unsupported type for %")
			}
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
		val, ok := ctx.getVar(t.Name)
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
	case *ObjectLit:
		obj := map[string]any{}
		for _, pair := range t.Pairs {
			// Remove quotes from key if needed
			key := pair.Key
			if len(key) >= 2 && key[0] == '"' && key[len(key)-1] == '"' {
				key = key[1 : len(key)-1]
			}
			obj[key] = EvalExpr(pair.Value, ctx)
		}
		return obj
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
	if fc.Name == "len" {
		arg := EvalExpr(fc.Args[0], ctx)
		switch v := arg.(type) {
		case string:
			return len(v)
		case []any:
			return len(v)
		case map[string]any:
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

	// ## Get from object with key
	if fc.Name == "getFromObjWithKey" {
		obj := EvalExpr(fc.Args[0], ctx)
		key := EvalExpr(fc.Args[1], ctx)
		switch o := obj.(type) {
		case map[string]any:
			return o[key.(string)]
		default:
			panic("getFromObjWithKey: unsupported type")
		}
	}

	// # User-defined
	fn := ctx.Functions[fc.Name]
	if fn == nil {
		panic("undefined function: " + fc.Name)
	}
	args := make(map[string]any)
	for i, param := range fn.Params {
		args[param.Name] = EvalExpr(fc.Args[i], ctx)
	}
	return EvalFuncBody(fn.Body, args, ctx)
}

func EvalFuncBody(body *FuncBody, args map[string]any, parentCtx *Context) any {
	// Create a local context for the function with the provided args as the only scope
	var functions map[string]*FuncDef
	if parentCtx != nil {
		functions = parentCtx.Functions
	} else {
		functions = map[string]*FuncDef{}
	}
	ctx := &Context{Functions: functions, VarStack: []map[string]any{args}}

	// helper to evaluate a statement within the function and capture returns
	var evalStmt func(*Stmt) (any, bool)
	evalStmt = func(stmt *Stmt) (any, bool) {
		switch {
		case stmt.Assign != nil:
			val := EvalExpr(stmt.Assign.Expr, ctx)
			if stmt.Assign.VarKw != nil {
				ctx.VarStack[len(ctx.VarStack)-1][stmt.Assign.Name] = val
			} else {
				found := false
				for i := len(ctx.VarStack) - 1; i >= 0; i-- {
					if _, ok := ctx.VarStack[i][stmt.Assign.Name]; ok {
						ctx.VarStack[i][stmt.Assign.Name] = val
						found = true
						break
					}
				}
				if !found {
					panic("variable not declared: " + stmt.Assign.Name)
				}
			}
			return nil, false

		case stmt.If != nil:
			// new scope
			ctx.VarStack = append(ctx.VarStack, map[string]any{})
			cond := EvalExpr(stmt.If.Condition, ctx)
			if cond.(bool) {
				for _, s := range stmt.If.Then {
					if val, ok := evalStmt(s); ok {
						ctx.VarStack = ctx.VarStack[:len(ctx.VarStack)-1]
						return val, true
					}
				}
			} else if stmt.If.Else != nil {
				for _, s := range stmt.If.Else.Body {
					if val, ok := evalStmt(s); ok {
						ctx.VarStack = ctx.VarStack[:len(ctx.VarStack)-1]
						return val, true
					}
				}
			}
			ctx.VarStack = ctx.VarStack[:len(ctx.VarStack)-1]
			return nil, false

		case stmt.For != nil:
			if stmt.For.Init != nil && stmt.For.Cond != nil && stmt.For.Post != nil {
				// init
				if stmt.For.Init != nil {
					// reuse assign logic
					init := stmt.For.Init
					val := EvalExpr(init.Expr, ctx)
					if init.VarKw != nil {
						ctx.VarStack[len(ctx.VarStack)-1][init.Name] = val
					} else {
						found := false
						for i := len(ctx.VarStack) - 1; i >= 0; i-- {
							if _, ok := ctx.VarStack[i][init.Name]; ok {
								ctx.VarStack[i][init.Name] = val
								found = true
								break
							}
						}
						if !found {
							panic("variable not declared: " + init.Name)
						}
					}
				}

				for {
					cond := EvalExpr(stmt.For.Cond, ctx)
					if !cond.(bool) {
						break
					}
					for _, s := range stmt.For.Body {
						if val, ok := evalStmt(s); ok {
							return val, true
						}
					}
					// post
					post := stmt.For.Post
					val := EvalExpr(post.Expr, ctx)
					if post.VarKw != nil {
						ctx.VarStack[len(ctx.VarStack)-1][post.Name] = val
					} else {
						found := false
						for i := len(ctx.VarStack) - 1; i >= 0; i-- {
							if _, ok := ctx.VarStack[i][post.Name]; ok {
								ctx.VarStack[i][post.Name] = val
								found = true
								break
							}
						}
						if !found {
							panic("variable not declared: " + post.Name)
						}
					}
				}
			}
			return nil, false

		case stmt.Return != nil:
			val := EvalExpr(stmt.Return.Left, ctx)
			return val, true
		}
		return nil, false
	}

	for _, stmt := range body.Stmts {
		if val, ok := evalStmt(stmt); ok {
			return val
		}
	}
	panic("no return statement in function body")
}

// repeatString repeats the string s n times.
func repeatString(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
