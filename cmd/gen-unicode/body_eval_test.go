package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// Interpret the emitted syntax, rather than the generator's input table or
// tree builder. The independent oracle uses Go's original classification code.
func parseGeneratedBody(t *testing.T, source []byte, name string) *ast.BlockStmt {
	t.Helper()
	declaration := strings.SplitN(string(source), "activity "+name+"(", 2)
	if len(declaration) != 2 {
		t.Fatalf("missing generated activity %s", name)
	}
	quoted := strings.SplitN(declaration[1], "`", 3)
	if len(quoted) != 3 {
		t.Fatalf("missing generated body %s", name)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "generated.go",
		"package generated\nfunc Check(input int64) bool {\n"+quoted[1]+"\n}", 0)
	if err != nil {
		t.Fatal(err)
	}
	return file.Decls[0].(*ast.FuncDecl).Body
}

func evaluateGeneratedBody(t *testing.T, body *ast.BlockStmt, input int64) bool {
	t.Helper()
	for _, statement := range body.List {
		switch value := statement.(type) {
		case *ast.ReturnStmt:
			return generatedExpression(t, value.Results[0], input).(bool)
		case *ast.IfStmt:
			if generatedExpression(t, value.Cond, input).(bool) {
				return evaluateGeneratedBody(t, value.Body, input)
			}
			if value.Else != nil {
				return evaluateGeneratedBody(t, value.Else.(*ast.BlockStmt), input)
			}
		default:
			t.Fatalf("unexpected generated statement %T", statement)
		}
	}
	t.Fatal("generated branch does not return")
	return false
}

func generatedExpression(t *testing.T, expression ast.Expr, input int64) any {
	t.Helper()
	switch value := expression.(type) {
	case *ast.Ident:
		switch value.Name {
		case "input":
			return input
		case "false":
			return false
		case "true":
			return true
		}
	case *ast.BasicLit:
		result, err := strconv.ParseInt(value.Value, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return result
	case *ast.ParenExpr:
		return generatedExpression(t, value.X, input)
	case *ast.UnaryExpr:
		if value.Op == token.SUB {
			return -generatedExpression(t, value.X, input).(int64)
		}
	case *ast.BinaryExpr:
		x := generatedExpression(t, value.X, input)
		if value.Op == token.LAND {
			return x.(bool) && generatedExpression(t, value.Y, input).(bool)
		}
		if value.Op == token.LOR {
			return x.(bool) || generatedExpression(t, value.Y, input).(bool)
		}
		a, b := x.(int64), generatedExpression(t, value.Y, input).(int64)
		switch value.Op {
		case token.SUB:
			return a - b
		case token.REM:
			return a % b
		case token.LSS:
			return a < b
		case token.LEQ:
			return a <= b
		case token.GTR:
			return a > b
		case token.GEQ:
			return a >= b
		case token.EQL:
			return a == b
		}
	}
	t.Fatalf("unexpected generated expression %T", expression)
	return nil
}
