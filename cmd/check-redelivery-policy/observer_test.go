package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Every fixture below is synthetic structural evidence, not an evaluation of
// the real Gooo program or a replacement policy/oracle implementation. Positive
// fixtures prove these production gates admit their supported shape; mutations
// then exercise rejection at the same gates used by the actual observer.
func fixtureInputs() map[string]any {
	return map[string]any{"method": "GET", "delivery_state": "NOT_SENT", "body_replayable": true, "idempotency_key_present": false, "server_repeat_contract_verified": false, "request_binding_current": true, "current_access_allowed": true, "attempts_remaining": json.Number("1"), "provider_retry_guidance": "ABSENT"}
}
func fixtureRecord(ins map[string]any) map[string]any {
	r := map[string]any{}
	for k, v := range ins {
		r[k] = v
	}
	r["domain_status"] = "SYNTHETIC"
	r["action"] = "SYNTHETIC"
	r["reason"] = "SYNTHETIC"
	r["eligible_to_claim"] = false
	r["attempts_if_claimed"] = json.Number("1")
	r["dedup_required"] = false
	return r
}
func clone(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	x, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func fixtureFieldRows(record map[string]any) []any {
	a := []any{}
	for _, n := range fieldNames {
		a = append(a, map[string]any{"name": n, "id": resultEntity + "/" + n, "value": record[n]})
	}
	return a
}
func fixtureTraces() ([]any, []map[string]any, []oracleRow) {
	traces := []any{}
	rows := []map[string]any{}
	oracle := []oracleRow{}
	for i := 0; i < 8; i++ {
		ins := fixtureInputs()
		ins["attempts_remaining"] = json.Number(strconv.Itoa(i))
		r := fixtureRecord(ins)
		ports := []any{}
		for j := 0; j < 9; j++ {
			ports = append(ports, map[string]any{"port": "input" + strconv.Itoa(j), "entity_id": inputEntities[j], "value": ins[fieldNames[j]]})
		}
		traces = append(traces, map[string]any{"case_index": json.Number(strconv.Itoa(i)), "deliveries": []any{map[string]any{"activity_id": activityID, "inputs": ports, "actual": r, "actual_fields": fixtureFieldRows(r)}}})
		rows = append(rows, ins)
		oracle = append(oracle, oracleRow{Index: i, Name: "synthetic", Inputs: ins, Expected: r})
	}
	return traces, rows, oracle
}

func TestStrictTransportAndInt64(t *testing.T) {
	for _, raw := range []string{`{"a":1,"\u0061":2}`, `{"n":1} true`, `{"s":"\ud800"}`, `{"s":"\udc00"}`, `{"s":"\ud800\u0041"}`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)} {
		if _, err := decode([]byte(raw)); err == nil {
			t.Fatalf("accepted malformed/nonunique/deep transport %q", raw)
		}
	}
	if _, err := decode([]byte{'"', 0xff, '"'}); err == nil {
		t.Fatal("invalid raw UTF8 accepted")
	}
	for _, raw := range []string{`{"s":"\ud83d\ude00"}`, `{"s":"\\uD800"}`, `{"s":"한글"}`} {
		if _, err := decode([]byte(raw)); err != nil {
			t.Fatalf("valid Unicode fixture: %v", err)
		}
	}
	for _, n := range []string{"-9223372036854775808", "9223372036854775807", "0", "-1"} {
		if !scalar(json.Number(n), "int64") {
			t.Fatalf("lost exact int64 %s", n)
		}
	}
	for _, v := range []any{true, "1", 1, 1.0, json.Number("1.0"), json.Number("1e0"), json.Number("-0"), json.Number("9223372036854775808"), json.Number("-9223372036854775809")} {
		if scalar(v, "int64") {
			t.Fatalf("accepted coerced/out-of-range integer %#v", v)
		}
	}
}

func TestRecordEveryRequiredFieldTypeAndEcho(t *testing.T) {
	ins := fixtureInputs()
	r := fixtureRecord(ins)
	if err := validateRecord(r, ins); err != nil {
		t.Fatal(err)
	}
	for i, n := range fieldNames {
		t.Run(n, func(t *testing.T) {
			missing := object(clone(t, r))
			delete(missing, n)
			if validateRecord(missing, ins) == nil {
				t.Fatal("missing field admitted")
			}
			wrong := object(clone(t, r))
			wrong[n] = nil
			if validateRecord(wrong, ins) == nil {
				t.Fatal("null field admitted")
			}
			wrong = object(clone(t, r))
			if fieldTypes[i] == "string" {
				wrong[n] = false
			} else {
				wrong[n] = "false"
			}
			if validateRecord(wrong, ins) == nil {
				t.Fatal("wrong typed field admitted")
			}
			if i < 9 {
				wrong = object(clone(t, r))
				switch fieldTypes[i] {
				case "string":
					wrong[n] = "different"
				case "bool":
					wrong[n] = !r[n].(bool)
				case "int64":
					wrong[n] = json.Number("2")
				}
				if validateRecord(wrong, ins) == nil {
					t.Fatal("changed echo admitted")
				}
			}
		})
	}
	for _, n := range []string{"-9223372036854775808", "9223372036854775807"} {
		input := fixtureInputs()
		input["attempts_remaining"] = json.Number(n)
		record := fixtureRecord(input)
		record["attempts_if_claimed"] = json.Number(n)
		if err := validateRecord(record, input); err != nil {
			t.Fatal("signed extreme must remain exact:", err)
		}
	}
}

func TestEightTraceOrderPortsAndAllFifteenFields(t *testing.T) {
	base, rows, oracle := fixtureTraces()
	if _, err := inspectTraces(base, rows, oracle); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func([]any){"omit-last": func(a []any) { a[7] = nil }, "duplicate-index": func(a []any) { object(a[1])["case_index"] = json.Number("0") }, "reorder": func(a []any) { a[0], a[1] = a[1], a[0] }, "second-delivery": func(a []any) {
		tr := object(a[0])
		tr["deliveries"] = append(array(tr["deliveries"]), array(tr["deliveries"])[0])
	}, "actual-null": func(a []any) { object(array(object(a[0])["deliveries"])[0])["actual"] = nil }, "receipt-expectation": func(a []any) { object(array(object(a[0])["deliveries"])[0])["expected"] = oracle[0].Expected }, "unknown-producer": func(a []any) { object(array(object(a[0])["deliveries"])[0])["producer_id"] = "other" }, "wrong-activity": func(a []any) { object(array(object(a[0])["deliveries"])[0])["activity_id"] = "other" }, "scalar-input": func(a []any) { object(array(object(a[0])["deliveries"])[0])["input"] = rows[0] }}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			a := array(clone(t, base))
			mutate(a)
			if _, err := inspectTraces(a, rows, oracle); err == nil {
				t.Fatal("mutated trace admitted")
			}
		})
	}
	for j := 0; j < 9; j++ {
		t.Run(fmt.Sprintf("port-%d", j), func(t *testing.T) {
			a := array(clone(t, base))
			d := object(array(object(a[0])["deliveries"])[0])
			port := object(array(d["inputs"])[j])
			port["entity_id"] = resultEntity
			if _, err := inspectTraces(a, rows, oracle); err == nil {
				t.Fatal("port entity mismatch admitted")
			}
			a = array(clone(t, base))
			d = object(array(object(a[0])["deliveries"])[0])
			port = object(array(d["inputs"])[j])
			port["port"] = "input99"
			if _, err := inspectTraces(a, rows, oracle); err == nil {
				t.Fatal("port order mismatch admitted")
			}
		})
	}
	for j := 0; j < 15; j++ {
		t.Run(fmt.Sprintf("field-%d", j), func(t *testing.T) {
			a := array(clone(t, base))
			d := object(array(object(a[0])["deliveries"])[0])
			object(array(d["actual_fields"])[j])["id"] = "wrong"
			if _, err := inspectTraces(a, rows, oracle); err == nil {
				t.Fatal("field identity mismatch admitted")
			}
			a = array(clone(t, base))
			d = object(array(object(a[0])["deliveries"])[0])
			object(d["actual"])[fieldNames[j]] = nil
			if _, err := inspectTraces(a, rows, oracle); err == nil {
				t.Fatal("actual field type mismatch admitted")
			}
			a = array(clone(t, base))
			d = object(array(object(a[0])["deliveries"])[0])
			actual := object(d["actual"])
			name := fieldNames[j]
			var changed any
			switch fieldTypes[j] {
			case "string":
				changed = actual[name].(string) + "-changed"
			case "bool":
				changed = !actual[name].(bool)
			case "int64":
				changed = json.Number("99")
			}
			actual[name] = changed
			object(array(d["actual_fields"])[j])["value"] = changed
			_, err := inspectTraces(a, rows, oracle)
			if err == nil {
				t.Fatal("well-typed changed actual/oracle comparison admitted")
			}
			if j >= 9 && !strings.Contains(err.Error(), "independent Go output differs") {
				t.Fatalf("derived-field oracle comparison was not reached: %v", err)
			}
		})
	}
	if _, err := inspectTraces(base[:7], rows, oracle); err == nil {
		t.Fatal("seven trace count admitted")
	}
}

func fixtureOracle(t *testing.T) ([]byte, []byte, []map[string]any) {
	t.Helper()
	cases := []any{}
	witness := []any{}
	inputs := []map[string]any{}
	for i, n := range caseNames {
		ins := fixtureInputs()
		if i == 13 {
			ins["attempts_remaining"] = json.Number("-9223372036854775808")
		}
		if i == 14 {
			ins["attempts_remaining"] = json.Number("9223372036854775807")
		}
		r := fixtureRecord(ins)
		cases = append(cases, map[string]any{"index": i, "name": n, "inputs": ins, "expected": r})
		witness = append(witness, map[string]any{"index": i, "name": n, "inputs": ins})
		inputs = append(inputs, ins)
	}
	o, err := json.Marshal(map[string]any{"schema": "gooo/redelivery-independent-manual-oracle/v1", "contract_sha256": contractSHA, "cases": cases})
	if err != nil {
		t.Fatal(err)
	}
	w, err := json.Marshal(map[string]any{"schema": "gooo/redelivery-independent-manual-inputs/v1", "contract_sha256": contractSHA, "cases": witness})
	if err != nil {
		t.Fatal(err)
	}
	return o, w, inputs
}
func TestFreshOracleWholeSixteenRoster(t *testing.T) {
	raw, witness, inputs := fixtureOracle(t)
	if _, err := loadOracle(raw, witness, inputs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"contract", "count", "order", "name", "original-input", "expected-null", "expected-float"} {
		t.Run(name, func(t *testing.T) {
			v, err := decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			m := object(v)
			cases := array(m["cases"])
			r := object(cases[0])
			switch name {
			case "contract":
				m["contract_sha256"] = "other"
			case "count":
				m["cases"] = cases[:15]
			case "order":
				cases[0], cases[1] = cases[1], cases[0]
			case "name":
				r["name"] = "other"
			case "original-input":
				object(r["inputs"])["method"] = "POST"
			case "expected-null":
				r["expected"] = nil
			case "expected-float":
				object(r["expected"])["attempts_if_claimed"] = json.Number("1.0")
			}
			changed, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := loadOracle(changed, witness, inputs); err == nil {
				t.Fatal("mutated oracle admitted")
			}
		})
	}
}
func TestNativeBatchInputShape(t *testing.T) {
	rows := []any{}
	for i := 0; i < 8; i++ {
		r := map[string]any{}
		ins := fixtureInputs()
		for j := 0; j < 9; j++ {
			r[appPackage+":"+activityName+".input"+strconv.Itoa(j)] = ins[fieldNames[j]]
		}
		rows = append(rows, r)
	}
	base := map[string]any{"schema": "gooo/body-composition-inputs/v1", "inputs": rows}
	raw, _ := json.Marshal(base)
	if _, err := readInputs(raw, 8); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing-port", "float-integer", "numeric-bool", "extra-port", "expectation", "count"} {
		t.Run(kind, func(t *testing.T) {
			m := object(clone(t, base))
			a := array(m["inputs"])
			r := object(a[0])
			prefix := appPackage + ":" + activityName + ".input"
			switch kind {
			case "missing-port":
				delete(r, prefix+"8")
			case "float-integer":
				r[prefix+"7"] = json.Number("1.0")
			case "numeric-bool":
				r[prefix+"2"] = json.Number("1")
			case "extra-port":
				r[prefix+"9"] = false
			case "expectation":
				m["expected"] = []any{}
			case "count":
				m["inputs"] = a[:7]
			}
			raw, _ := json.Marshal(m)
			if _, err := readInputs(raw, 8); err == nil {
				t.Fatal("bad batch admitted")
			}
		})
	}
}

func fixtureRecordABI() []any {
	fields := []any{}
	for i, n := range fieldNames {
		fields = append(fields, map[string]any{"name": n, "id": resultEntity + "/" + n, "type_id": typeID(i), "go_name": fieldSymbol(i)})
	}
	return []any{map[string]any{"name": "RedeliveryDecision", "id": resultEntity, "go_name": recordSymbol(), "fields": fields}}
}
func fixturePlan() map[string]any {
	ports := []any{}
	for i := 0; i < 9; i++ {
		ports = append(ports, map[string]any{"port": "input" + strconv.Itoa(i), "type": inputTypes[i], "entity_id": inputEntities[i], "from": -1})
	}
	return map[string]any{"schema": "gooo/body-composition-plan/v1", "typed_plan_sha256": digest([]byte("synthetic-plan")), "semantic_fingerprint": strings.Repeat("a", 64), "activities": []any{map[string]any{"name": loweredName, "id": activityID, "input_type": "Text", "output_type": "RedeliveryDecision", "input_entity_id": textEntity, "output_entity_id": resultEntity, "go_function": generatedName, "input_from": -1, "assembling": false, "inputs": ports}}, "edges": []any{}, "record_types": fixtureRecordABI()}
}
func fixtureSource(pkg, name string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\ntype %s struct {\n", pkg, recordSymbol())
	for i, n := range fieldNames {
		fmt.Fprintf(&b, "%s %s `json:\"%s\"`\n", fieldSymbol(i), fieldTypes[i], n)
	}
	fmt.Fprintf(&b, "}\nfunc %s(", name)
	for i := 0; i < 9; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "input%d %s", i, fieldTypes[i])
	}
	fmt.Fprintf(&b, ") %s { return %s{", recordSymbol(), recordSymbol())
	for i := range fieldNames {
		if i > 0 {
			b.WriteString(",")
		}
		v := `""`
		if fieldTypes[i] == "bool" {
			v = "false"
		}
		if fieldTypes[i] == "int64" {
			v = "0"
		}
		if i < 9 {
			v = "input" + strconv.Itoa(i)
		}
		fmt.Fprintf(&b, "%s:%s", fieldSymbol(i), v)
	}
	b.WriteString("} }\n")
	return b.String()
}
func TestConcretePlanAndRecordMutations(t *testing.T) {
	plan := object(clone(t, fixturePlan()))
	if err := inspectPlan(plan); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"extra-entry", "wrong-input", "input-order", "record-id", "field-order", "optional-field", "fingerprint", "extra-helper"} {
		t.Run(name, func(t *testing.T) {
			p := object(clone(t, plan))
			a := object(array(p["activities"])[0])
			r := object(array(p["record_types"])[0])
			fields := array(r["fields"])
			switch name {
			case "extra-entry":
				p["entry_activity"] = loweredName
			case "wrong-input":
				object(array(a["inputs"])[7])["type"] = "Text"
			case "input-order":
				ports := array(a["inputs"])
				ports[0], ports[1] = ports[1], ports[0]
			case "record-id":
				r["id"] = "other"
			case "field-order":
				fields[0], fields[1] = fields[1], fields[0]
			case "optional-field":
				object(fields[0])["presence"] = "optional"
			case "fingerprint":
				p["semantic_fingerprint"] = "fake"
			case "extra-helper":
				p["preparations"] = []any{}
			}
			if inspectPlan(p) == nil {
				t.Fatal("mutated concrete plan admitted")
			}
		})
	}
}
func TestConcreteGeneratedABIMutations(t *testing.T) {
	source := fixtureSource("main", generatedName)
	step := fixtureSource("gooo_workspace", loweredName)
	if err := inspectGeneratedPair(source, step); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]string{"import": strings.Replace(source, "package main", "package main\nimport \"path\"", 1), "global": source + "var shared bool\n", "init": source + "func init(){}\n", "wrong-input-kind": strings.Replace(source, "input7 int64", "input7 string", 1), "wrong-output": strings.Replace(source, ") "+recordSymbol()+" {", ") string {", 1), "wrong-field-kind": strings.Replace(source, fieldSymbol(7)+" int64", fieldSymbol(7)+" string", 1), "wrong-json-tag": strings.Replace(source, `json:"method"`, `json:"wrong"`, 1), "unexpected-call": strings.Replace(source, "{ return ", "{ panic(0); return ", 1), "recursion": strings.Replace(source, "{ return ", "{ "+generatedName+"(); return ", 1), "directive": "//go:noinline\n" + source, "legacy-build": "// +build linux\n\n" + source, "line": "//line hidden:1\n" + source}
	for name, changed := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := parseBindingSource(changed, "main", generatedName); err == nil {
				t.Fatal("unsupported generated source admitted")
			}
		})
	}
	changed := strings.Replace(source, fieldSymbol(9)+`:""`, fieldSymbol(9)+`:"different"`, 1)
	if changed == source {
		t.Fatal("body mutation ineffective")
	}
	if err := inspectGeneratedPair(changed, step); err == nil {
		t.Fatal("ABI-valid changed composition body admitted")
	}
}
func TestLoweredBodyLiteralBinding(t *testing.T) {
	body := "\nlet x = input0\nreturn x\n"
	quoted := strconv.Quote(body)
	for _, source := range []string{"activity A(Text) -> Text computes " + quoted, "activity A(Text) -> Text computes `" + body + "`"} {
		if bodyDigest(source) != digest([]byte(body)) {
			t.Fatal("body literal spelling changed semantic body hash")
		}
	}
	if bodyDigest("activity A(Text) -> Text computes "+quoted+"\nactivity B(Text) -> Text computes "+quoted) != "" {
		t.Fatal("two computes bodies admitted")
	}
}

func fixtureProcess() (map[string]any, originBindings) {
	o := originBindings{Repository: "/synthetic/repository", SourceFreeze: rawRef{Path: "/synthetic/run/source-freeze.json"}, Contract: rawRef{Path: "/synthetic/C/contract.json"}, OracleBinary: rawRef{Path: "/synthetic/work/bin/oracle"}}
	p := map[string]any{"name": "original-oracle-manual16", "status": "COMPLETED", "exit_code": json.Number("0"), "expected_exit_code": json.Number("0"), "automatic_retries": json.Number("0"), "process_reaped": true, "owned_group_remaining": false, "abort": nil, "primary_error": nil, "source_tools_unchanged_after": true, "secondary_errors": []any{}, "termination_errors": []any{}, "owned_group_state": map[string]any{"state": "absent", "error": nil}, "argv": []any{o.OracleBinary.Path, "--contract", o.Contract.Path, "--manual"}, "cwd": "/synthetic/repository/examples/automatic-redelivery/reference", "pid": json.Number("12"), "owned_pgid": json.Number("12"), "timeout_seconds": json.Number("30"), "elapsed_seconds": json.Number("0.5"), "source_freeze_before": map[string]any{"synthetic": true}}
	return p, o
}
func TestIndependentProducerProcessAndStartedBinding(t *testing.T) {
	base, o := fixtureProcess()
	if err := checkOracleProcess(base, o); err != nil {
		t.Fatal(err)
	}
	started := object(clone(t, base))
	started["status"] = "STARTED"
	if err := checkStarted(base, started); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"exit-kind", "exit-float", "unreaped", "group-unknown", "group-permission", "argv", "cwd", "pid", "timeout", "secondary", "source-changed", "automatic-retry"} {
		t.Run(name, func(t *testing.T) {
			p := object(clone(t, base))
			switch name {
			case "exit-kind":
				p["exit_code"] = false
			case "exit-float":
				p["exit_code"] = json.Number("0.0")
			case "unreaped":
				p["process_reaped"] = false
			case "group-unknown":
				object(p["owned_group_state"])["state"] = "unknown"
			case "group-permission":
				object(p["owned_group_state"])["error"] = "EPERM"
			case "argv":
				p["argv"] = []any{o.OracleBinary.Path, "--manual"}
			case "cwd":
				p["cwd"] = "/synthetic/J"
			case "pid":
				p["owned_pgid"] = json.Number("13")
			case "timeout":
				p["timeout_seconds"] = json.Number("46")
			case "secondary":
				p["secondary_errors"] = []any{"error"}
			case "source-changed":
				p["source_tools_unchanged_after"] = false
			case "automatic-retry":
				p["automatic_retries"] = json.Number("1")
			}
			if checkOracleProcess(p, o) == nil {
				t.Fatal("invalid original producer process admitted")
			}
		})
	}
	for _, name := range []string{"name", "pid", "owned_pgid", "argv", "cwd", "source_freeze_before", "timeout_seconds", "expected_exit_code", "automatic_retries"} {
		t.Run("started-"+name, func(t *testing.T) {
			s := object(clone(t, started))
			s[name] = nil
			if checkStarted(base, s) == nil {
				t.Fatal("changed STARTED identity admitted")
			}
		})
	}
}
func TestCurrentReferenceTamperingAndDuplicates(t *testing.T) {
	root := canonicalTestTempDir(t)
	path := filepath.Join(root, "source.txt")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := currentRef(path, 32)
	if err != nil {
		t.Fatal(err)
	}
	v := clone(t, r)
	if _, err := parseRef(v); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseRef(v); err == nil {
		t.Fatal("same-size changed file hash admitted")
	}
	fresh, err := currentRef(path, 32)
	if err != nil {
		t.Fatal(err)
	}
	dup := []any{clone(t, fresh), clone(t, fresh)}
	if _, err := refIndex(dup); err == nil {
		t.Fatal("duplicate ref roster admitted")
	}
	invalid := object(clone(t, fresh))
	invalid["bytes"] = json.Number("5.0")
	if _, err := parseRef(invalid); err == nil {
		t.Fatal("floating byte count admitted")
	}
	if _, err := currentRef(root, 32); err == nil {
		t.Fatal("directory accepted as source")
	}
	if _, err := currentRef(path, 4); err == nil {
		t.Fatal("file byte cap ignored")
	}
	alias := filepath.Join(root, "alias.txt")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := currentRef(alias, 32); err == nil {
		t.Fatal("real symlink alias accepted as canonical source")
	}
}
func TestPinnedProfileIntegerAndRawScalarPreservation(t *testing.T) {
	if err := loadResultProfile(filepath.Join("..", "check-path-facts", "result-profile-83928fa.json")); err != nil {
		t.Fatal(err)
	}
	var profile resultProfile
	if err := json.Unmarshal(resultProfileJSON, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Compiler != compilerRevision {
		t.Fatal("wrong pinned profile")
	}
	for _, raw := range []string{"0", "-9223372036854775808", "9223372036854775807"} {
		d := json.NewDecoder(strings.NewReader(raw))
		d.UseNumber()
		got, err := normalizeResultValue(d, []byte(raw), "$.runtime.model_calls", profile.Nodes, 0)
		if err != nil || !bytes.Equal(got, []byte(raw)) {
			t.Fatalf("pinned integer exactness %s %v", raw, err)
		}
	}
	for _, raw := range []string{"true", "0.0", "1e0", "-0", "9223372036854775808"} {
		d := json.NewDecoder(strings.NewReader(raw))
		d.UseNumber()
		if _, err := normalizeResultValue(d, []byte(raw), "$.runtime.model_calls", profile.Nodes, 0); err == nil {
			t.Fatalf("pinned typed counter accepted %s", raw)
		}
	}
	raw := []byte(`{"integer":-9223372036854775808,"text":"\u003c"}`)
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	got, err := normalizeResultValue(d, raw, "$.runtime.traces.*.deliveries.*.actual", profile.Nodes, 0)
	if err != nil || !reflect.DeepEqual(got, raw) {
		t.Fatalf("pinned actual RawMessage spelling changed %s %v", got, err)
	}
}

func fixtureReceipt(t *testing.T) (map[string]any, []map[string]any, []oracleRow, identity) {
	t.Helper()
	traces, rows, oracle := fixtureTraces()
	src := "activity " + loweredName + "(Text) -> Text computes \"synthetic-body\""
	srcDigest := digest([]byte(src))
	source := fixtureSource("main", generatedName)
	step := fixtureSource("gooo_workspace", loweredName)
	generated := digest([]byte(source))
	driver := digest([]byte("synthetic-driver"))
	plan := fixturePlan()
	id := identity{Manifest: digest([]byte("synthetic-manifest")), Inputs: digest([]byte("synthetic-inputs")), GoTool: digest([]byte("synthetic-tool")), Body: digest([]byte("synthetic-body")), Sources: map[string]sourceIdentity{sourceName: {appPackage, digest([]byte("synthetic-original-source"))}}}
	exports := []any{}
	for _, pair := range []struct{ name, id string }{{"Text", textEntity}, {"Boolean", booleanEntity}, {"Integer", integerEntity}, {"RedeliveryDecision", resultEntity}} {
		exports = append(exports, map[string]any{"kind": "entity", "name": pair.name, "id": pair.id})
	}
	exports = append(exports, map[string]any{"kind": "activity", "name": activityName, "id": activityID, "input_types": inputEntities, "output_type": resultEntity})
	entry := map[string]any{"package_path": appPackage, "activity": activityName, "lowered_name": loweredName}
	program := map[string]any{"schema": "gooo/workspace-body-program/v1", "entry": entry, "activities": []any{entry}, "lowered_gooo_source": src, "workspace_image": map[string]any{"schema": "gooo/package-runtime-image/v1", "packages": []any{map[string]any{"path": appPackage, "sources": []any{map[string]any{"filename": sourceName, "source_digest": id.Sources[sourceName].Digest}}, "exports": exports}}}}
	parameters := []any{}
	for i := 0; i < 9; i++ {
		parameters = append(parameters, map[string]any{"name": "input" + strconv.Itoa(i), "type": fieldTypes[i]})
	}
	gr := map[string]any{"schema": "gooo/body-codegen-report/v3", "decision": "PASS", "activity": loweredName, "activity_id": activityID, "compiler_source_sha": compilerRevision, "source_digest": srcDigest, "program_digest": id.Body, "generated_digest": digest([]byte(step)), "replay_digest": digest([]byte(step)), "typecheck_passed": true, "deterministic_replay": true, "repository_writes": 0, "input_type": "(string,string,bool,bool,bool,bool,bool,int64,string)", "output_type": "RedeliveryDecision", "input_parameters": parameters, "record_types": fixtureRecordABI()}
	composition := map[string]any{"schema": "gooo/body-composition/v1", "stage": "COMPLETE", "original_source_sha256": srcDigest, "selected_source_sha256": srcDigest, "gooo_source": src, "source": source, "generated_sha256": generated, "driver_source": "synthetic-driver", "driver_sha256": driver, "plan": plan, "steps": []any{map[string]any{"input_source_sha256": srcDigest, "generation": map[string]any{"report": gr, "source": step}}}}
	process := map[string]any{"started": true, "completed": true, "canceled": false, "timed_out": false, "output_truncated": false, "diagnostics_bytes": 0, "exit_code": 0, "stdout_sha256": digest([]byte("synthetic-output")), "stderr_sha256": digest(nil)}
	rt := map[string]any{"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": compilerRevision, "go_version": "go version " + goVersion + " " + runtimePlatform(), "go_tool_sha256": id.GoTool, "go_tool_selection": "explicit", "producer_module": map[string]any{"path": "github.com/kimjooyoon/meta-ontology-go", "version": "v0.0.0-20261011201109-83928fa45382"}, "generated_sha256": generated, "driver_sha256": driver, "original_source_sha256": srcDigest, "selected_source_sha256": srcDigest, "executable_sha256": digest([]byte("synthetic-binary")), "runtime_suite_sha256": digest([]byte("synthetic-suite")), "typed_plan_sha256": plan["typed_plan_sha256"], "model_calls": 0, "finite_total": 0, "finite_passed": 0, "projection_replayed": true, "runtime_replayed": true, "toolchain": process, "build": process, "runs": []any{process, process}, "traces": traces}
	e := map[string]any{"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED", "manifest": "synthetic-manifest", "manifest_digest": id.Manifest, "inputs_digest": id.Inputs, "result": map[string]any{"schema": "gooo/workspace-body-execution/v1", "program": program, "composition": composition, "runtime": rt, "execution_source_sha256": srcDigest}}
	return e, rows, oracle, id
}
func TestReceiptProvenanceAndSavedReplayGates(t *testing.T) {
	base, rows, oracle, id := fixtureReceipt(t)
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspect(raw, rows, oracle, id, replayBinding{}); err != nil {
		t.Fatal("positive structural receipt:", err)
	}
	prior := replayBinding{Receipt: digest(raw), Result: digest([]byte("synthetic-pinned-result"))}
	replay := object(clone(t, base))
	replay["replayed_from_sha256"] = prior.Receipt
	object(replay["result"])["replay"] = map[string]any{"schema": "gooo/workspace-body-replay/v1", "prior_result_sha256": prior.Result, "body_fills_replayed": 0, "model_calls": 0, "scope": "synthetic"}
	reRaw, _ := json.Marshal(replay)
	if _, err := inspect(reRaw, rows, oracle, id, prior); err != nil {
		t.Fatal("positive structural saved replay:", err)
	}
	for i, bad := range []any{false, "", []any{}, map[string]any{}} {
		t.Run(fmt.Sprintf("raw-purecalls-kind-%d", i), func(t *testing.T) {
			r := object(clone(t, base))
			object(object(r["result"])["program"])["pure_calls"] = bad
			changed, _ := json.Marshal(r)
			if _, err := inspect(changed, rows, oracle, id, replayBinding{}); err == nil {
				t.Fatal("coerced pure_calls admitted")
			}
		})
	}
	for _, name := range []string{"manifest", "source-image", "producer", "go-tool", "typed-plan", "program-body", "generation-body", "finite-expectation", "run-count", "raw-replay-parent", "inner-replay-parent"} {
		t.Run(name, func(t *testing.T) {
			r := object(clone(t, base))
			binding := replayBinding{}
			if strings.Contains(name, "replay-parent") {
				r = object(clone(t, replay))
				binding = prior
			}
			result := object(r["result"])
			rt := object(result["runtime"])
			switch name {
			case "manifest":
				r["manifest_digest"] = digest([]byte("other"))
			case "source-image":
				pkg := object(array(object(object(result["program"])["workspace_image"])["packages"])[0])
				object(array(pkg["sources"])[0])["source_digest"] = digest([]byte("other"))
			case "producer":
				rt["producer_source_sha"] = "other"
			case "go-tool":
				rt["go_tool_sha256"] = digest([]byte("other"))
			case "typed-plan":
				rt["typed_plan_sha256"] = digest([]byte("other"))
			case "program-body":
				object(result["program"])["lowered_gooo_source"] = "other"
			case "generation-body":
				gen := object(object(array(object(result["composition"])["steps"])[0])["generation"])
				object(gen["report"])["program_digest"] = digest([]byte("other"))
			case "finite-expectation":
				rt["finite_total"] = 1
			case "run-count":
				rt["runs"] = array(rt["runs"])[:1]
			case "raw-replay-parent":
				r["replayed_from_sha256"] = digest([]byte("other"))
			case "inner-replay-parent":
				object(result["replay"])["prior_result_sha256"] = digest([]byte("other"))
			}
			changed, _ := json.Marshal(r)
			if _, err := inspect(changed, rows, oracle, id, binding); err == nil {
				t.Fatal("receipt provenance mutation admitted")
			}
		})
	}
}

// Only positive fixture roots are resolved; raw-reference guards stay strict.
func canonicalTestTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
