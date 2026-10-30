package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const etagPortPackage = "go/net/http/etag"
const etagReportPackage = "app/strong-validator-facts"
const etagScanActivity = "ScanETag"
const etagStrongActivity = "ETagStrongMatch"
const etagReportActivity = "Report"
const etagPortSource = "ports/net/http/etag.gooo"
const etagReportSource = "examples/strong-validator-facts/main.gooo"
const etagScanActivityID = "gooo://go/net/http/scan-etag"
const etagStrongActivityID = "gooo://go/net/http/etag-strong-match"
const etagReportActivityID = "gooo://go-ports/examples/strong-validator-facts/report"
const etagScanEntity = "gooo://go/net/http/etag-scan-result/v1"
const etagFactsEntity = "gooo://go-ports/examples/strong-validator-facts"

var etagScanNames = []string{"etag", "remain"}
var etagFactsNames = []string{"raw", "tag", "remainder", "header_valid", "present", "weak", "complete", "strong", "single_strong", "input_bytes", "tag_bytes", "remainder_bytes", "reason"}
var etagFactsKinds = []string{"string", "string", "string", "boolean", "boolean", "boolean", "boolean", "boolean", "boolean", "integer", "integer", "integer", "string"}

func isETagSuite(s suite) bool {
	return s.Package == etagPortPackage && (s.Activity == etagScanActivity || s.Activity == etagStrongActivity) || s.Package == etagReportPackage && s.Activity == etagReportActivity
}
func etagReportConsumer(s suite) bool {
	return s.Package == etagReportPackage && s.Activity == etagReportActivity
}
func etagGoToolPath() string {
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(runtime.GOROOT(), "bin", name)
}
func etagResultContract(s suite) resultContract {
	if !isHTTPConditionalSuite(s) {
		return resultContract{}
	}
	if isIfMatchSuite(s) {
		return resultContract{Entity: ifMatchEntity, Fields: []recordFieldContract{{"condition", ifMatchEntity + "/condition"}}}
	}
	if s.Activity == etagStrongActivity {
		return resultContract{Entity: "gooo://std/boolean"}
	}
	id, names := etagScanEntity, etagScanNames
	if etagReportConsumer(s) {
		id, names = etagFactsEntity, etagFactsNames
	}
	c := resultContract{Entity: id}
	for _, name := range names {
		c.Fields = append(c.Fields, recordFieldContract{name, id + "/" + name})
	}
	return c
}
func etagRecords(consumer bool) []any {
	r := []any{headerAdditionOwnRecord("ETagScanResult", etagScanEntity, etagScanNames, []string{"string", "string"})}
	if consumer {
		r = append(r, headerAdditionOwnRecord("StrongValidatorFacts", etagFactsEntity, etagFactsNames, etagFactsKinds))
	}
	return r
}

type etagDefinition struct {
	Package, Source, Namespace, Lowered string
	Declaration                         decimalPayloadBody
}
type etagPackageSpec struct {
	path, source, name, namespace string
	entities                      map[string]string
	activities                    []decimalPayloadBody
	imports                       []string
}

func etagSpecs(s suite) []etagPackageSpec {
	if isIfMatchSuite(s) {
		return []etagPackageSpec{{ifMatchPackage, ifMatchSource, "httpcondition", "httpcondition", map[string]string{"Text": textEntity, "IfMatchResult": ifMatchEntity}, []decimalPayloadBody{{Name: ifMatchActivity, ID: ifMatchActivityID, Inputs: []string{"Text", "Text"}, Output: "IfMatchResult"}}, nil}}
	}
	core := etagPackageSpec{etagPortPackage, etagPortSource, "httpetag", "httpetag", map[string]string{"Text": textEntity, "Boolean": "gooo://std/boolean", "ETagScanResult": etagScanEntity}, []decimalPayloadBody{
		{Name: etagScanActivity, ID: etagScanActivityID, Inputs: []string{"Text"}, Output: "ETagScanResult"},
		{Name: etagStrongActivity, ID: etagStrongActivityID, Inputs: []string{"Text", "Text"}, Output: "Boolean"}}, nil}
	if !etagReportConsumer(s) {
		return []etagPackageSpec{core}
	}
	return []etagPackageSpec{core,
		{headerPortPackage, headerPortSource, "httpguts", "gooo_ports_x_net_httpguts", map[string]string{"Text": textEntity, "Boolean": "gooo://std/boolean"}, []decimalPayloadBody{
			{Name: headerNameActivity, ID: "gooo://golang.org/x/net/http/httpguts/ValidHeaderFieldName", Inputs: []string{"Text"}, Output: "Boolean"},
			{Name: headerValueActivity, ID: "gooo://golang.org/x/net/http/httpguts/ValidHeaderFieldValue", Inputs: []string{"Text"}, Output: "Boolean"}}, nil},
		{etagReportPackage, etagReportSource, "strongvalidator", "strongvalidator", map[string]string{"Text": textEntity, "StrongValidatorFacts": etagFactsEntity}, []decimalPayloadBody{{Name: etagReportActivity, ID: etagReportActivityID, Inputs: []string{"Text"}, Output: "StrongValidatorFacts"}}, []string{`import etags "` + etagPortPackage + `"`, `import headers "` + headerPortPackage + `"`}}}
}

var etagSourceSHA = map[string]string{
	ifMatchSource:    "9924cde4a2e01222658929a87007d6e34db85f2d4f49c057d110bb8d1bf05aeb",
	etagPortSource:   "2fc5b2c19acf8c040173859309dd687be37bca1c2c56b49efa293a8df98d42c4",
	headerPortSource: "a033b96ab0adb394a9d79137715d7a4170e22061e4ae9aa282f8ab3671da9f76",
	etagReportSource: "a7002b9db59ff68e732b66ef27c86426b8fff325cfb15f2665a0d0cf04f17a01",
}

// Source identities and authored declarations are independent of receipt names.
// No init order or lowered numeric prefix is inferred from manifest order.
func etagDefinitions(s suite, identity expectedIdentity) (map[string]etagDefinition, error) {
	if !isHTTPConditionalSuite(s) || len(identity.SourceBytes) != len(etagSpecs(s)) || len(identity.Sources) != len(identity.SourceBytes) {
		return nil, fmt.Errorf("etag exact selected source roster")
	}
	defs := map[string]etagDefinition{}
	var recordSource []byte
	for _, spec := range etagSpecs(s) {
		raw, ok := identity.SourceBytes[spec.source]
		encoded, err := json.Marshal(string(raw))
		if !ok || err != nil || contentDigest(raw) != "sha256:"+etagSourceSHA[spec.source] || identity.Sources[spec.source] != (sourceIdentity{spec.path, contentDigest(encoded)}) {
			return nil, fmt.Errorf("etag current raw/image source binding")
		}
		if !strings.Contains(string(raw), "package "+spec.name+"\n") || !strings.Contains(string(raw), "namespace "+spec.namespace+"\n") || len(catalogImport.FindAll(raw, -1)) != len(spec.imports) {
			return nil, fmt.Errorf("etag authored package/namespace/imports")
		}
		previous := -1
		for _, line := range spec.imports {
			at := strings.Index(string(raw), line+"\n")
			if at <= previous {
				return nil, fmt.Errorf("etag authored import order/alias")
			}
			previous = at
		}
		bodies, err := decimalPayloadBodies(raw)
		decls := decimalPayloadDeclaration.FindAllSubmatch(raw, -1)
		if err != nil || len(bodies) != len(spec.activities) || len(decls) != len(spec.activities) {
			return nil, fmt.Errorf("etag authored activity roster")
		}
		for i, want := range spec.activities {
			body := bodies[want.Name]
			if string(decls[i][1]) != want.Name || body.ID != want.ID || body.Output != want.Output || !reflect.DeepEqual(body.Inputs, want.Inputs) {
				return nil, fmt.Errorf("etag authored signature/ID")
			}
			defs[spec.path+":"+want.Name] = etagDefinition{spec.path, spec.source, spec.namespace, "", body}
		}
		recordSource = append(recordSource, raw...)
	}
	records, err := etagSourceRecords(recordSource, etagRecordsFor(s))
	if err != nil || len(records) != len(etagRecordsFor(s)) {
		return nil, fmt.Errorf("etag authored exact record declarations")
	}
	return defs, nil
}
func etagSourceRecords(raw []byte, allowed []any) ([]any, error) {
	known := map[string]map[string]any{}
	for _, x := range allowed {
		r := x.(map[string]any)
		known[r["name"].(string)] = r
	}
	seen := map[string]bool{}
	records := []any{}
	for _, m := range headerAdditionRecordPattern.FindAllSubmatch(raw, -1) {
		r, ok := known[string(m[1])]
		if !ok || r["id"] != string(m[2]) || seen[string(m[1])] {
			return nil, fmt.Errorf("etag unknown/duplicate record")
		}
		seen[string(m[1])] = true
		fields := r["fields"].([]any)
		got := headerFieldDeclaration.FindAllSubmatch(m[3], -1)
		if len(got) != len(fields) || len(headerAnyFieldDeclaration.FindAll(m[3], -1)) != len(fields) {
			return nil, fmt.Errorf("etag record field arity")
		}
		for i, v := range fields {
			f := v.(map[string]any)
			if string(got[i][1]) != f["name"] || string(got[i][2]) != f["id"] || "urn:gooo:type:"+string(got[i][3]) != f["type_id"] {
				return nil, fmt.Errorf("etag record field ID/type/order")
			}
		}
		records = append(records, r)
	}
	return records, nil
}
func etagSameRecords(value any, records []any) bool {
	actual, ok := value.([]any)
	if value == nil {
		actual, ok = []any{}, true
	}
	if !ok || len(actual) != len(records) {
		return false
	}
	want := map[string]any{}
	for _, v := range records {
		r := v.(map[string]any)
		want[r["id"].(string)] = r
	}
	seen := map[string]bool{}
	for _, v := range actual {
		r, ok := v.(map[string]any)
		if !ok {
			return false
		}
		id, ok := r["id"].(string)
		if !ok || seen[id] || !decimalPayloadSame(r, want[id]) {
			return false
		}
		seen[id] = true
	}
	return true
}
func etagCallees(s suite) []string {
	if !etagReportConsumer(s) {
		return nil
	}
	return []string{etagPortPackage + ":" + etagScanActivity, headerPortPackage + ":" + headerValueActivity, etagPortPackage + ":" + etagStrongActivity}
}
func etagToken(key string) string {
	if key == headerPortPackage+":"+headerValueActivity {
		return "headers." + headerValueActivity
	}
	return "etags." + strings.TrimPrefix(key, etagPortPackage+":")
}
func etagReachable(s suite) []string {
	return append([]string{s.Package + ":" + s.Activity}, etagCallees(s)...)
}
func etagLowerBody(d etagDefinition, defs map[string]etagDefinition, s suite) string {
	body := d.Declaration.Body
	if d.Package == etagReportPackage {
		for _, key := range etagCallees(s) {
			body = strings.ReplaceAll(body, etagToken(key), defs[key].Lowered)
		}
	}
	return body
}
func etagBindLowered(text string, s suite, defs map[string]etagDefinition) (map[string]etagDefinition, []any, error) {
	actual, err := decimalPayloadBodies([]byte(text))
	keys := etagReachable(s)
	if err != nil || len(actual) != len(keys) {
		return nil, nil, fmt.Errorf("etag reachable lowered activity count")
	}
	byID := map[string]decimalPayloadBody{}
	for _, d := range actual {
		if byID[d.ID].Name != "" {
			return nil, nil, fmt.Errorf("etag duplicate lowered semantic ID")
		}
		byID[d.ID] = d
	}
	bound := map[string]etagDefinition{}
	for _, key := range keys {
		d := defs[key]
		a, ok := byID[d.Declaration.ID]
		if !ok || !regexp.MustCompile(`^GoooPackage[0-9]+Activity`+regexp.QuoteMeta(d.Declaration.Name)+`$`).MatchString(a.Name) || a.Output != d.Declaration.Output || !reflect.DeepEqual(a.Inputs, d.Declaration.Inputs) {
			return nil, nil, fmt.Errorf("etag lowered exact semantic signature")
		}
		d.Lowered = a.Name
		bound[key] = d
	}
	for key, d := range bound {
		if byID[d.Declaration.ID].Body != etagLowerBody(d, bound, s) {
			return nil, nil, fmt.Errorf("etag lowered authored body/callee binding: %s", key)
		}
	}
	records, err := etagSourceRecords([]byte(text), etagRecordsFor(s))
	if err != nil {
		return nil, nil, err
	}
	for _, d := range bound {
		if d.Declaration.Output != "ETagScanResult" && d.Declaration.Output != "StrongValidatorFacts" && d.Declaration.Output != "IfMatchResult" {
			continue
		}
		found := false
		for _, r := range records {
			if r.(map[string]any)["name"] == d.Declaration.Output {
				found = true
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("etag reachable output record missing")
		}
	}
	return bound, records, nil
}

func etagInspectImage(image map[string]any, s suite, defs map[string]etagDefinition) error {
	packages, ok := image["packages"].([]any)
	order, ordered := image["init_order"].([]any)
	specs := etagSpecs(s)
	if !ok || !ordered || len(packages) != len(specs) || len(order) != len(specs) || image["schema"] != "gooo/package-runtime-image/v1" {
		return fmt.Errorf("etag image package/init roster")
	}
	byPath := map[string]map[string]any{}
	positions := map[string]int{}
	for i, v := range order {
		path, ok := v.(string)
		if !ok {
			return fmt.Errorf("etag init path")
		}
		if _, exists := positions[path]; exists {
			return fmt.Errorf("etag duplicate init path")
		}
		positions[path] = i
	}
	for _, v := range packages {
		p, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("etag image package object")
		}
		path, ok := p["path"].(string)
		if !ok || byPath[path] != nil {
			return fmt.Errorf("etag duplicate image package")
		}
		byPath[path] = p
	}
	for _, spec := range specs {
		p, ok := byPath[spec.path]
		at, exists := positions[spec.path]
		if !ok || !exists || p["name"] != spec.name || p["namespace"] != spec.namespace {
			return fmt.Errorf("etag image package/namespace")
		}
		if etagReportConsumer(s) && spec.path != etagReportPackage && at >= positions[etagReportPackage] {
			return fmt.Errorf("etag dependency must precede Report")
		}
		imports := []any{}
		if spec.path == etagReportPackage {
			imports = []any{etagPortPackage, headerPortPackage}
		}
		actualImports, ok := p["imports"].([]any)
		if p["imports"] == nil {
			actualImports, ok = []any{}, true
		}
		if !ok || len(actualImports) != len(imports) {
			return fmt.Errorf("etag image imports")
		}
		seenImport := map[any]bool{}
		for _, v := range actualImports {
			path, ok := v.(string)
			if !ok || seenImport[path] || (path != etagPortPackage && path != headerPortPackage) {
				return fmt.Errorf("etag image import path")
			}
			seenImport[path] = true
		}
		for _, v := range imports {
			if !seenImport[v] {
				return fmt.Errorf("etag missing image import")
			}
		}
		exports := []any{}
		for _, a := range spec.activities {
			inputs := []string{}
			for _, t := range a.Inputs {
				inputs = append(inputs, spec.entities[t])
			}
			exports = append(exports, map[string]any{"name": a.Name, "kind": "activity", "id": a.ID, "input_types": inputs, "output_type": spec.entities[a.Output]})
		}
		for name, id := range spec.entities {
			exports = append(exports, map[string]any{"name": name, "kind": "entity", "id": id})
		}
		actualExports, ok := p["exports"].([]any)
		if !ok || len(actualExports) != len(exports) {
			return fmt.Errorf("etag full declared export roster")
		}
		want := map[string]any{}
		for _, v := range exports {
			e := v.(map[string]any)
			want[e["kind"].(string)+":"+e["name"].(string)] = e
		}
		seen := map[string]bool{}
		for _, v := range actualExports {
			e, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("etag export object")
			}
			key := fmt.Sprint(e["kind"]) + ":" + fmt.Sprint(e["name"])
			if seen[key] || !decimalPayloadSame(e, want[key]) {
				return fmt.Errorf("etag exact export ID/type")
			}
			seen[key] = true
		}
	}
	root := defs[s.Package+":"+s.Activity]
	if !decimalPayloadSame(image["entry"], map[string]any{"package_path": s.Package, "source": root.Source, "activity": s.Activity, "inputs": root.Declaration.Inputs, "output": root.Declaration.Output}) {
		return fmt.Errorf("etag image entry")
	}
	return nil
}

// Signature-unique roles identify composed helper names without guessing CallN.
func inspectETagNativeABI(source string, defs map[string]etagDefinition, s suite, records []any, composed bool, rootFunction string) error {
	f, err := parser.ParseFile(token.NewFileSet(), "etag-generated.go", source, 0)
	if err != nil {
		return err
	}
	kinds := map[string]string{"Text": "string", "Boolean": "bool"}
	remaining := map[string]map[string]any{}
	for _, v := range records {
		r := v.(map[string]any)
		kinds[r["name"].(string)] = r["go_name"].(string)
		remaining[r["go_name"].(string)] = r
	}
	functions := map[string]*ast.FuncDecl{}
	names := map[string]string{}
	for _, decl := range f.Decls {
		if _, function := decl.(*ast.FuncDecl); !function {
			gd, declaration := decl.(*ast.GenDecl)
			if !declaration || gd.Tok != token.TYPE {
				return fmt.Errorf("etag native only type/function declarations")
			}
		}
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			for _, v := range gd.Specs {
				t, ok := v.(*ast.TypeSpec)
				if !ok || t.Assign.IsValid() || t.TypeParams != nil {
					return fmt.Errorf("etag native type alias")
				}
				r, ok := remaining[t.Name.Name]
				if !ok {
					return fmt.Errorf("etag native unknown record")
				}
				delete(remaining, t.Name.Name)
				st, ok := t.Type.(*ast.StructType)
				fields := r["fields"].([]any)
				if !ok || len(st.Fields.List) != len(fields) {
					return fmt.Errorf("etag native record arity")
				}
				for i, v := range fields {
					want := v.(map[string]any)
					a := st.Fields.List[i]
					k, ok := a.Type.(*ast.Ident)
					tag := ""
					if a.Tag != nil {
						tag, err = strconv.Unquote(a.Tag.Value)
					}
					kind := map[string]string{"urn:gooo:type:string": "string", "urn:gooo:type:boolean": "bool", "urn:gooo:type:integer": "int64"}[want["type_id"].(string)]
					if err != nil || !ok || len(a.Names) != 1 || a.Names[0].Name != want["go_name"] || k.Name != kind || tag != `json:"`+want["name"].(string)+`"` {
						return fmt.Errorf("etag native field ID/type/order/tag")
					}
				}
			}
		}
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if functions[fn.Name.Name] != nil || fn.Body == nil || fn.Recv != nil || fn.Type.TypeParams != nil || fn.Type.Params == nil || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
			return fmt.Errorf("etag native function shape")
		}
		functions[fn.Name.Name] = fn
		matches := []string{}
		for key, d := range defs {
			if !composed && fn.Name.Name != d.Lowered {
				continue
			}
			if len(fn.Type.Params.List) != len(d.Declaration.Inputs) {
				continue
			}
			output, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || output.Name != kinds[d.Declaration.Output] || len(fn.Type.Results.List[0].Names) != 0 {
				continue
			}
			valid := true
			for i, a := range fn.Type.Params.List {
				k, ok := a.Type.(*ast.Ident)
				name := "input"
				if len(d.Declaration.Inputs) > 1 {
					name = fmt.Sprintf("input%d", i)
				}
				if !ok || k.Name != kinds[d.Declaration.Inputs[i]] || len(a.Names) != 1 || a.Names[0].Name != name {
					valid = false
				}
			}
			if valid {
				matches = append(matches, key)
			}
		}
		if len(matches) != 1 || names[matches[0]] != "" {
			return fmt.Errorf("etag native unique role/signature")
		}
		names[matches[0]] = fn.Name.Name
	}
	if len(remaining) != 0 || len(functions) != len(defs) || len(names) != len(defs) {
		return fmt.Errorf("etag native exact reached function/record roster")
	}
	rootKey := s.Package + ":" + s.Activity
	if composed && names[rootKey] != rootFunction {
		return fmt.Errorf("etag composed root plan connection")
	}
	for key, name := range names {
		calls := []string{}
		invalid := false
		ast.Inspect(functions[name].Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok {
				invalid = true
				return true
			}
			if functions[id.Name] != nil {
				calls = append(calls, id.Name)
			} else if id.Name != "len" && id.Name != "int64" {
				invalid = true
			}
			return true
		})
		wanted := []string{}
		if key == rootKey {
			for _, callee := range etagCallees(s) {
				wanted = append(wanted, names[callee])
			}
		}
		if invalid || !reflect.DeepEqual(calls, wanted) {
			return fmt.Errorf("etag native exact imported call order")
		}
	}
	return nil
}

func etagExpectedStdout(s suite) ([]byte, error) {
	matrix := [][]json.RawMessage{}
	fields := etagResultContract(s).Fields
	for _, row := range s.Cases {
		var raw []byte
		if len(fields) == 0 {
			var err error
			raw, err = json.Marshal(row.Expected)
			if err != nil {
				return nil, err
			}
		} else {
			values, ok := row.Expected.(map[string]any)
			if !ok || len(values) != len(fields) {
				return nil, fmt.Errorf("etag oracle record shape")
			}
			var b bytes.Buffer
			b.WriteByte('{')
			for i, f := range fields {
				v, ok := values[f.Name]
				if !ok {
					return nil, fmt.Errorf("etag missing oracle field")
				}
				if i > 0 {
					b.WriteByte(',')
				}
				key, _ := json.Marshal(f.Name)
				value, err := json.Marshal(v)
				if err != nil {
					return nil, err
				}
				b.Write(key)
				b.WriteByte(':')
				b.Write(value)
			}
			b.WriteByte('}')
			raw = b.Bytes()
		}
		matrix = append(matrix, []json.RawMessage{raw})
	}
	raw, err := json.Marshal(matrix)
	return append(raw, '\n'), err
}

// Complements generic finite value/source/parent checks. Inner canonical plan,
// semantic and saved-result digests are cross-bound only, never reimplemented.
func inspectETagABI(raw []byte, s suite, identity expectedIdentity) error {
	return inspectETagABIWithProcesses(raw, s, identity, nil)
}

// nil retains the original stateless process contract verbatim. Only the
// explicitly selected session ABI supplies a cached-process validator.
func inspectETagABIWithProcesses(raw []byte, s suite, identity expectedIdentity, validateProcesses func([]byte) error) error {
	all, err := etagDefinitions(s, identity)
	if err != nil {
		return err
	}
	v, err := decode(raw)
	if err != nil {
		return err
	}
	top, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("etag receipt object")
	}
	r, err := decimalPayloadReceiptObject(top, "result")
	if err != nil {
		return err
	}
	p, err := decimalPayloadReceiptObject(r, "program")
	if err != nil {
		return err
	}
	c, err := decimalPayloadReceiptObject(r, "composition")
	if err != nil {
		return err
	}
	rt, err := decimalPayloadReceiptObject(r, "runtime")
	if err != nil {
		return err
	}
	image, err := decimalPayloadReceiptObject(p, "workspace_image")
	if err != nil {
		return err
	}
	text, ok := p["lowered_gooo_source"].(string)
	if !ok {
		return fmt.Errorf("etag lowered source")
	}
	defs, records, err := etagBindLowered(text, s, all)
	if err != nil {
		return err
	}
	rootKey := s.Package + ":" + s.Activity
	root := defs[rootKey]
	entry := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": root.Lowered}
	if r["schema"] != "gooo/workspace-body-execution/v1" || p["schema"] != "gooo/workspace-body-program/v1" || !decimalPayloadSame(p["entry"], entry) || !decimalPayloadSame(p["activities"], []any{entry}) || p["entity_aliases"] != nil {
		return fmt.Errorf("etag source root binding")
	}
	if err := etagInspectImage(image, s, all); err != nil {
		return err
	}
	// Generic inspectWorkspace already checks complete source-image raw bindings.
	pure := p["pure_calls"]
	if pure != nil || etagReportConsumer(s) {
		ids, err := selectPureCallIdentityProfile(identity.CompilerRevision)
		if err != nil {
			return err
		}
		pc, ok := pure.(map[string]any)
		if !ok || pc["schema"] != "gooo/workspace-pure-calls/v1" {
			return fmt.Errorf("etag pure source metadata")
		}
		acts, ok := pc["activities"].([]any)
		if !ok || len(acts) != len(defs) {
			return fmt.Errorf("etag pure reachable roster")
		}
		seen := map[string]bool{}
		for _, x := range acts {
			a, ok := x.(map[string]any)
			if !ok {
				return fmt.Errorf("etag pure activity")
			}
			ref, ok := a["activity"].(map[string]any)
			if !ok {
				return fmt.Errorf("etag pure reference")
			}
			key := fmt.Sprint(ref["package_path"]) + ":" + fmt.Sprint(ref["activity"])
			d, ok := defs[key]
			if !ok || seen[key] || !decimalPayloadSame(ref, map[string]any{"package_path": d.Package, "activity": d.Declaration.Name, "lowered_name": d.Lowered}) || a["source"] != d.Source || a["source_body_sha256"] != contentDigest([]byte(d.Declaration.Body)) || a["activity_id"] != ids.original(d.Namespace, d.Declaration.Name, d.Declaration.ID) || a["lowered_activity_id"] != ids.lowered(d.Lowered, d.Declaration.ID) {
				return fmt.Errorf("etag pure exact body/ID")
			}
			seen[key] = true
		}
		sites := []any{}
		for _, key := range etagCallees(s) {
			token := etagToken(key)
			at := strings.Index(root.Declaration.Body, token+"(")
			if at < 0 || strings.Count(root.Declaration.Body, token) != 1 {
				return fmt.Errorf("etag exact source call site")
			}
			sites = append(sites, map[string]any{"caller": rootKey, "callee": key, "surface": "computes", "start_byte": at, "end_byte": at + len(token)})
		}
		if !decimalPayloadSame(pc["sites"], sites) {
			return fmt.Errorf("etag pure source sites")
		}
	}
	sh := contentDigest([]byte(text))
	if c["schema"] != "gooo/body-composition/v1" || c["stage"] != "COMPLETE" || c["gooo_source"] != text || c["original_source_sha256"] != sh || c["selected_source_sha256"] != sh || r["execution_source_sha256"] != sh {
		return fmt.Errorf("etag lowered raw source binding")
	}
	plan, err := decimalPayloadReceiptObject(c, "plan")
	if err != nil {
		return err
	}
	activities, ok := plan["activities"].([]any)
	if !ok || len(activities) != 1 || plan["schema"] != "gooo/body-composition-plan/v1" || !decimalPayloadSame(plan["edges"], []any{}) || plan["preparations"] != nil || !etagSameRecords(plan["record_types"], records) {
		return fmt.Errorf("etag exact single typed plan/records")
	}
	if value := plan["entry_activity"]; value != nil && value != root.Lowered || etagReportConsumer(s) && value != root.Lowered {
		return fmt.Errorf("etag typed root entry")
	}
	a, ok := activities[0].(map[string]any)
	if !ok {
		return fmt.Errorf("etag plan activity")
	}
	fn, ok := a["go_function"].(string)
	if !ok || !token.IsIdentifier(fn) || a["name"] != root.Lowered || a["id"] != root.Declaration.ID || a["input_type"] != "Text" || a["output_type"] != root.Declaration.Output || a["input_entity_id"] != textEntity || a["output_entity_id"] != etagResultContract(s).Entity || !decimalPayloadSame(a["input_from"], -1) || a["assembling"] != false || a["prepared"] != nil && a["prepared"] != false {
		return fmt.Errorf("etag plan exact input/output/root function")
	}
	ports := []any{}
	parameters := []any{}
	for i := range root.Declaration.Inputs {
		ports = append(ports, map[string]any{"port": fmt.Sprintf("input%d", i), "type": "Text", "entity_id": textEntity, "from": -1})
		parameters = append(parameters, map[string]any{"name": fmt.Sprintf("input%d", i), "type": "string"})
	}
	if value := a["inputs"]; value != nil || len(ports) > 1 {
		if !decimalPayloadSame(value, ports) {
			return fmt.Errorf("etag explicit input port order")
		}
	}
	steps, ok := c["steps"].([]any)
	if !ok || len(steps) != 1 {
		return fmt.Errorf("etag one generation step")
	}
	step, ok := steps[0].(map[string]any)
	if !ok || step["input_source_sha256"] != sh {
		return fmt.Errorf("etag generation input source")
	}
	generation, err := decimalPayloadReceiptObject(step, "generation")
	if err != nil {
		return err
	}
	g, err := decimalPayloadReceiptObject(generation, "report")
	if err != nil {
		return err
	}
	generated, ok := generation["source"].(string)
	in := "string"
	if len(ports) == 2 {
		in = "(string,string)"
	}
	out := root.Declaration.Output
	if out == "Boolean" {
		out = "bool"
	}
	if !ok || generated == "" || g["schema"] != "gooo/body-codegen-report/v3" || g["decision"] != "PASS" || g["activity"] != root.Lowered || g["activity_id"] != root.Declaration.ID || g["compiler_source_sha"] != identity.CompilerRevision || g["source_digest"] != sh || g["program_digest"] != contentDigest([]byte(etagLowerBody(root, defs, s))) || g["generated_digest"] != contentDigest([]byte(generated)) || g["replay_digest"] != g["generated_digest"] || g["input_type"] != in || g["output_type"] != out || !etagSameRecords(g["record_types"], records) || g["typecheck_passed"] != true || g["deterministic_replay"] != true || !decimalPayloadSame(g["repository_writes"], 0) {
		return fmt.Errorf("etag generated current source/type/producer")
	}
	if value := g["input_parameters"]; value != nil || len(ports) > 1 {
		if !decimalPayloadSame(value, parameters) {
			return fmt.Errorf("etag native input parameter order")
		}
	}
	callees := etagCallees(s)
	closure := g["call_closure"]
	if closure != nil || len(callees) > 0 {
		cl, ok := closure.(map[string]any)
		if !ok || cl["schema"] != "gooo/pure-activity-call-closure/v1" || !decimalPayloadSame(cl["max_calls_per_invocation"], len(callees)) {
			return fmt.Errorf("etag imported call budget")
		}
		as, ok := cl["activities"].([]any)
		if !ok || len(as) != len(callees) {
			return fmt.Errorf("etag imported program roster")
		}
		wanted := map[string]any{}
		edges := []any{}
		for i, key := range callees {
			d := defs[key]
			wanted[d.Lowered] = map[string]any{"name": d.Lowered, "activity_id": d.Declaration.ID, "program_sha256": contentDigest([]byte(etagLowerBody(d, defs, s)))}
			edges = append(edges, map[string]any{"caller_id": root.Declaration.ID, "callee_id": d.Declaration.ID, "ordinal": i})
		}
		seen := map[string]bool{}
		for _, v := range as {
			x, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("etag imported program")
			}
			name, _ := x["name"].(string)
			if seen[name] || !decimalPayloadSame(x, wanted[name]) {
				return fmt.Errorf("etag imported program ID/body")
			}
			seen[name] = true
		}
		if !decimalPayloadSame(cl["edges"], edges) {
			return fmt.Errorf("etag call ordinals0/1/2")
		}
	}
	composed, ok := c["source"].(string)
	driver, driverOK := c["driver_source"].(string)
	planSHA, digestOK := plan["typed_plan_sha256"].(string)
	if !ok || composed == "" || !driverOK || driver == "" || c["generated_sha256"] != contentDigest([]byte(composed)) || rt["generated_sha256"] != c["generated_sha256"] || c["driver_sha256"] != contentDigest([]byte(driver)) || rt["driver_sha256"] != c["driver_sha256"] || !digestOK || !validDigest(planSHA) || rt["typed_plan_sha256"] != planSHA || rt["original_source_sha256"] != sh || rt["selected_source_sha256"] != sh {
		return fmt.Errorf("etag runtime actual source/driver/plan crossbinding")
	}
	goSHA, err := digestFile(etagGoToolPath())
	if err != nil {
		return err
	}
	if rt["go_tool_selection"] != "explicit" || rt["go_tool_sha256"] != "sha256:"+goSHA {
		return fmt.Errorf("etag exact explicit Go binary")
	}
	if err := inspectETagNativeABI(generated, defs, s, records, false, fn); err != nil {
		return err
	}
	if err := inspectETagNativeABI(composed, defs, s, records, true, fn); err != nil {
		return err
	}
	stdout, err := etagExpectedStdout(s)
	if err != nil {
		return err
	}
	runs, ok := rt["runs"].([]any)
	if !ok || len(runs) != 2 {
		return fmt.Errorf("etag two fresh native runs")
	}
	if validateProcesses == nil {
		processes := append([]any{rt["toolchain"], rt["build"]}, runs...)
		for _, v := range processes {
			x, ok := v.(map[string]any)
			if !ok || x["started"] != true || x["completed"] != true || x["canceled"] != false || x["timed_out"] != false || x["output_truncated"] != false || !decimalPayloadSame(x["exit_code"], 0) {
				return fmt.Errorf("etag internal native process completion")
			}
		}
	} else if err := validateProcesses(raw); err != nil {
		return err
	}
	for _, v := range runs {
		if v.(map[string]any)["stdout_sha256"] != contentDigest(stdout) {
			return fmt.Errorf("etag independent Go matrix raw stdout digest")
		}
	}
	return nil
}
