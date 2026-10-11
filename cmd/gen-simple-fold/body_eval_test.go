package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

type integerExpression func(int64) int64
type booleanExpression func(int64) bool
type statementBlock func(int64) (int64, bool)

// Compile the emitted AST into a small test interpreter once. This checks the
// emitted branches over all Unicode code points without invoking the compiler
// or mistaking a Go translation for actual Gooo native execution evidence.
func compileGeneratedBody(t *testing.T, source []byte) integerExpression {
	t.Helper()
	text := string(source)
	start, end := strings.Index(text, " computes `"), strings.LastIndex(text, "`")
	if start < 0 || end <= start+len(" computes `") {
		t.Fatal("missing generated computes body")
	}
	body := text[start+len(" computes `") : end]
	if body != strings.TrimSpace(body) {
		t.Fatal("noncanonical computes whitespace")
	}
	file, err := parser.ParseFile(token.NewFileSet(), "emitted-body.go", "package generated\nfunc Fold(input int64) int64 {\n"+body+"\n}", 0)
	if err != nil {
		t.Fatal(err)
	}
	evaluate := compileBlock(t, file.Decls[0].(*ast.FuncDecl).Body)
	return func(input int64) int64 {
		value, returned := evaluate(input)
		if !returned {
			panic("generated body failed to return")
		}
		return value
	}
}

func compileBlock(t *testing.T, block *ast.BlockStmt) statementBlock {
	t.Helper()
	var statements []statementBlock
	for _, statement := range block.List {
		switch value := statement.(type) {
		case *ast.ReturnStmt:
			if len(value.Results) != 1 {
				t.Fatal("invalid generated return")
			}
			result := compileInteger(t, value.Results[0])
			statements = append(statements, func(input int64) (int64, bool) { return result(input), true })
		case *ast.IfStmt:
			if value.Init != nil {
				t.Fatal("unexpected generated if initializer")
			}
			condition, then := compileBoolean(t, value.Cond), compileBlock(t, value.Body)
			var otherwise statementBlock
			if value.Else != nil {
				block, ok := value.Else.(*ast.BlockStmt)
				if !ok {
					t.Fatal("unexpected generated else statement")
				}
				otherwise = compileBlock(t, block)
			}
			statements = append(statements, func(input int64) (int64, bool) {
				if condition(input) {
					return then(input)
				}
				if otherwise != nil {
					return otherwise(input)
				}
				return 0, false
			})
		default:
			t.Fatalf("unsupported emitted statement %T", statement)
		}
	}
	return func(input int64) (int64, bool) {
		for _, statement := range statements {
			if result, returned := statement(input); returned {
				return result, true
			}
		}
		return 0, false
	}
}

func compileInteger(t *testing.T, expression ast.Expr) integerExpression {
	t.Helper()
	switch value := expression.(type) {
	case *ast.Ident:
		if value.Name == "input" {
			return func(input int64) int64 { return input }
		}
	case *ast.BasicLit:
		n, err := strconv.ParseInt(value.Value, 10, 64)
		if value.Kind != token.INT || err != nil {
			t.Fatal("invalid generated integer", value.Value, err)
		}
		return func(int64) int64 { return n }
	case *ast.ParenExpr:
		return compileInteger(t, value.X)
	case *ast.UnaryExpr:
		if value.Op == token.SUB {
			x := compileInteger(t, value.X)
			return func(input int64) int64 { return -x(input) }
		}
	case *ast.BinaryExpr:
		x, y := compileInteger(t, value.X), compileInteger(t, value.Y)
		switch value.Op {
		case token.ADD:
			return func(input int64) int64 { return x(input) + y(input) }
		case token.SUB:
			return func(input int64) int64 { return x(input) - y(input) }
		case token.REM:
			return func(input int64) int64 { return x(input) % y(input) }
		}
	}
	t.Fatalf("unsupported emitted integer expression %T", expression)
	return nil
}

func compileBoolean(t *testing.T, expression ast.Expr) booleanExpression {
	t.Helper()
	if value, ok := expression.(*ast.ParenExpr); ok {
		return compileBoolean(t, value.X)
	}
	value, ok := expression.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("unsupported emitted Boolean expression %T", expression)
	}
	if value.Op == token.LAND || value.Op == token.LOR {
		x, y := compileBoolean(t, value.X), compileBoolean(t, value.Y)
		if value.Op == token.LAND {
			return func(input int64) bool { return x(input) && y(input) }
		}
		return func(input int64) bool { return x(input) || y(input) }
	}
	x, y := compileInteger(t, value.X), compileInteger(t, value.Y)
	switch value.Op {
	case token.LSS:
		return func(input int64) bool { return x(input) < y(input) }
	case token.GTR:
		return func(input int64) bool { return x(input) > y(input) }
	case token.EQL:
		return func(input int64) bool { return x(input) == y(input) }
	}
	t.Fatalf("unsupported emitted comparison %s", value.Op)
	return nil
}
