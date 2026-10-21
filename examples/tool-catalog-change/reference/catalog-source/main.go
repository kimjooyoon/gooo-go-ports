// Private, closed-shape metadata extraction. Never executes upstream code.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
)

type Tool struct {
	Name       string `json:"name"`
	Service    string `json:"service"`
	Risk       string `json:"risk"`
	Capability string `json:"capability"`
}

func need(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func funcs(path string) map[string]*ast.FuncDecl {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		panic(err)
	}
	result := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok {
			need(result[f.Name.Name] == nil, "duplicate function")
			result[f.Name.Name] = f
		}
	}
	return result
}
func returned(f *ast.FuncDecl) ast.Expr {
	need(f != nil && f.Body != nil && len(f.Body.List) == 1, "expected one return statement")
	r, ok := f.Body.List[0].(*ast.ReturnStmt)
	need(ok && len(r.Results) == 1, "expected one returned value")
	return r.Results[0]
}
func stringValue(expr ast.Expr) string {
	v, ok := expr.(*ast.BasicLit)
	need(ok && v.Kind == token.STRING, "nonliteral metadata")
	s, err := strconv.Unquote(v.Value)
	if err != nil {
		panic(err)
	}
	return s
}
func symbolic(expr ast.Expr, constants map[string]string) string {
	id, ok := expr.(*ast.Ident)
	need(ok, "nonsymbolic constant")
	s, ok := constants[id.Name]
	need(ok, "unknown metadata constant")
	return s
}
func spec(expr ast.Expr, constants map[string]string) Tool {
	c, ok := expr.(*ast.CompositeLit)
	need(ok, "nonliteral tool spec")
	fields := map[string]ast.Expr{}
	for _, elt := range c.Elts {
		k, ok := elt.(*ast.KeyValueExpr)
		need(ok, "non-keyed tool field")
		id, ok := k.Key.(*ast.Ident)
		need(ok && fields[id.Name] == nil, "invalid/duplicate field")
		fields[id.Name] = k.Value
	}
	t := Tool{Name: stringValue(fields["Name"]), Service: stringValue(fields["Service"]), Risk: symbolic(fields["Risk"], constants)}
	if c, ok := fields["Capability"]; ok {
		t.Capability = symbolic(c, constants)
	}
	return t
}
func factoryList(expr ast.Expr, functions map[string]*ast.FuncDecl, constants map[string]string) ([]Tool, []string) {
	list, ok := expr.(*ast.CompositeLit)
	need(ok, "catalog is not a literal list")
	tools := []Tool{}
	names := []string{}
	for _, elt := range list.Elts {
		call, ok := elt.(*ast.CallExpr)
		need(ok && len(call.Args) == 0, "base catalog entry is not zero-arg factory")
		id, ok := call.Fun.(*ast.Ident)
		need(ok, "unknown factory syntax")
		tools = append(tools, spec(returned(functions[id.Name]), constants))
		names = append(names, id.Name)
	}
	return tools, names
}
func reviewedFactoryHashes(path string) map[string]string {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		panic(err)
	}
	result := map[string]string{}
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok {
			result[f.Name.Name] = fmt.Sprintf("%x", sha256.Sum256(raw[fset.Position(f.Pos()).Offset:fset.Position(f.End()).Offset]))
		}
	}
	return result
}
func resolvedMutation(expr ast.Expr, functions map[string]*ast.FuncDecl, constants map[string]string, reviewed map[string]string) Tool {
	if _, ok := expr.(*ast.CompositeLit); ok {
		return spec(expr, constants)
	}
	call, ok := expr.(*ast.CallExpr)
	need(ok, "unknown mutation expression")
	id, ok := call.Fun.(*ast.Ident)
	need(ok, "unknown mutation factory")
	switch id.Name {
	case "mcpGmailListDraftsTool":
		need(len(call.Args) == 0, "list-drafts args changed")
		return spec(returned(functions[id.Name]), constants)
	case "mcpGmailComposeTool":
		need(reviewed[id.Name] == "75dd5eae7bcdaacab8947639684cd75f59aa2436d384b317fbcf13b61e612007", "compose factory preimage changed")
		need(len(call.Args) == 1, "compose args changed")
		action := stringValue(call.Args[0])
		need(action == "create" || action == "update" || action == "send", "unknown compose action")
		t := Tool{Name: "gmail_" + action + "_draft", Service: "gmail", Risk: "write"}
		if action == "send" {
			t.Name = "gmail_send_message"
			t.Capability = "gmail_send"
		}
		return t
	case "mcpGmailModifyTool":
		need(reviewed[id.Name] == "dd4d519c905043c81cfd7e581d9e8b9d03c6c7e6e5560edf1868b944814d0057", "modify factory preimage changed")
		need(len(call.Args) == 1, "modify args changed")
		thread, ok := call.Args[0].(*ast.Ident)
		need(ok && (thread.Name == "true" || thread.Name == "false"), "nonliteral thread selector")
		name := "gmail_modify_messages"
		if thread.Name == "true" {
			name = "gmail_modify_thread"
		}
		return Tool{Name: name, Service: "gmail", Risk: "write"}
	case "mcpGmailMessagesTool":
		need(reviewed[id.Name] == "13c9c168138cac5d839715b2203dedc28301be79d486e46e5e7ccf1f87a31e09", "messages factory preimage changed")
		need(len(call.Args) == 4, "messages args changed")
		name := stringValue(call.Args[0])
		_ = stringValue(call.Args[1])
		command, ok := call.Args[2].(*ast.CompositeLit)
		need(ok, "nonliteral command fixture")
		for _, v := range command.Elts {
			_ = stringValue(v)
		}
		capability := ""
		if _, ok := call.Args[3].(*ast.BasicLit); ok {
			capability = stringValue(call.Args[3])
		} else {
			capability = symbolic(call.Args[3], constants)
		}
		return Tool{Name: name, Service: "gmail", Risk: "write", Capability: capability}
	default:
		panic("unregistered factory: " + id.Name)
	}
}
func main() {
	need(len(os.Args) == 2, "expected evidence directory")
	root := os.Args[1]
	constants := map[string]string{"mcpRiskRead": "read", "mcpRiskWrite": "write", "mcpCapabilityGmailSend": "gmail_send", "mcpCapabilityGmailDelete": "gmail_delete"}
	beforeFunctions := funcs(filepath.Join(root, "before/internal/cmd/mcp_tools.go.txt"))
	before, names := factoryList(returned(beforeFunctions["mcpAllTools"]), beforeFunctions, constants)
	need(len(before) == 10, "unreviewed base catalog size")
	afterFunctions := funcs(filepath.Join(root, "after/internal/cmd/mcp_tools.go.txt"))
	appendCall, ok := returned(afterFunctions["mcpAllTools"]).(*ast.CallExpr)
	need(ok, "after catalog must append")
	id, ok := appendCall.Fun.(*ast.Ident)
	need(ok && id.Name == "append" && len(appendCall.Args) == 2 && appendCall.Ellipsis.IsValid(), "unreviewed after catalog composition")
	base, afterNames := factoryList(appendCall.Args[0], afterFunctions, constants)
	need(len(base) == len(before), "base catalog size changed")
	for i, t := range base {
		need(t == before[i] && afterNames[i] == names[i], "base metadata/factory changed")
	}
	mutationCall, ok := appendCall.Args[1].(*ast.CallExpr)
	need(ok && len(mutationCall.Args) == 0, "unreviewed appended catalog")
	mutationID, ok := mutationCall.Fun.(*ast.Ident)
	need(ok && mutationID.Name == "mcpGmailMutationTools", "unknown appended catalog")
	path := filepath.Join(root, "after/internal/cmd/mcp_gmail.go.txt")
	mutationFunctions := funcs(path)
	reviewed := reviewedFactoryHashes(path)
	mutationList, ok := returned(mutationFunctions[mutationID.Name]).(*ast.CompositeLit)
	need(ok, "mutation catalog is not literal")
	after := append([]Tool{}, base...)
	for _, elt := range mutationList.Elts {
		after = append(after, resolvedMutation(elt, mutationFunctions, constants, reviewed))
	}
	seen := map[string]bool{}
	for _, t := range after {
		need(!seen[t.Name], "duplicate after tool")
		seen[t.Name] = true
	}
	result := map[string]any{"schema": "gooo/gogcli-closed-source-catalog/v1", "before_metadata": before, "after_metadata": after, "unresolved_reachable_factory_entries": 0, "reviewed_factory_hashes": reviewed, "scope": "Complete metadata of the two reviewed mcpAllTools expressions; not runtime registration, account/config validation, invocation behavior or authority. Three nonliteral factories are projected only under reviewed exact function preimages."}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		panic(err)
	}
}
