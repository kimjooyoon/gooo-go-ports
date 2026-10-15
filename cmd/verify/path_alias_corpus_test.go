package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPathAliasGoWitnessesPreserveOriginalTextAndAllRequiredKinds(t *testing.T) {
	for _, row := range []struct {
		left, right, lc, rc string
		alias               bool
	}{
		{"", ".", ".", ".", true}, {"/", "/", "/", "/", false},
		{"a/../../b", "../b", "../b", "../b", true}, {"/../../a", "/a", "/a", "/a", true},
		{"../../a", "../a", "../../a", "../a", false}, {`a\..\b`, "b", `a\..\b`, "b", false},
		{"a/%2e%2e/b", "b", "a/%2e%2e/b", "b", false}, {"e\u0301", "é", "e\u0301", "é", false},
		{"a/\x00/../b", "a/b", "a/b", "a/b", true}, {"a", "/a", "a", "/a", false},
	} {
		got, err := pathAliasReference(row.left, row.right)
		if err != nil || len(got) != 11 {
			t.Fatal("required eleven facts", err, got)
		}
		for _, item := range []struct{ name, want string }{{"left", row.left}, {"right", row.right}, {"left_clean", row.lc}, {"right_clean", row.rc}} {
			if value, ok := got[item.name].(string); !ok || value != item.want {
				t.Fatal("original or Go-cleaned Text", item, got)
			}
		}
		want := map[string]bool{"left_is_abs": strings.HasPrefix(row.left, "/"), "right_is_abs": strings.HasPrefix(row.right, "/"), "left_changed": row.left != row.lc, "right_changed": row.right != row.rc, "exact_same": row.left == row.right, "same_clean": row.lc == row.rc, "lexical_alias": row.alias}
		for name, expected := range want {
			if value, ok := got[name].(bool); !ok || value != expected {
				t.Fatal("exact Boolean relation", name, got)
			}
		}
	}
	for _, value := range []string{strings.Repeat("a", 1025), string([]byte{0xff})} {
		if got, err := pathAliasReference(value, ""); err == nil || got != nil {
			t.Fatal("unsupported Text must not yield compatible expectations")
		}
		if got, err := pathAliasReference("", value); err == nil || got != nil {
			t.Fatal("right Text domain must be checked")
		}
	}
}

func TestPathAliasFrozenPopulationsOrderAndAnchorPrefix(t *testing.T) {
	suites, err := pathAliasCorpus("../..")
	if err != nil || len(suites) != 2 {
		t.Fatal(err)
	}
	if suites[0].Package != pathCleanPackage || suites[0].Activity != pathCleanActivity || len(suites[0].Cases) != 9841 || suites[1].Package != pathAliasPackage || suites[1].Activity != pathAliasActivity || len(suites[1].Cases) != 120 {
		t.Fatal("two appended finite suites")
	}
	for _, index := range []int{0, 1, 2, 3, 9840} {
		want := map[int]string{0: "", 1: ".", 2: "/", 3: "a", 9840: "aaaaaaaa"}[index]
		if !reflect.DeepEqual(suites[0].Cases[index].Inputs, []any{want}) {
			t.Fatal("original ordered shortlex", index)
		}
		if _, ok := suites[0].Cases[index].Expected.(string); !ok {
			t.Fatal("scalar Clean expected kind")
		}
	}
	raw, err := os.ReadFile(filepath.Join("../..", pathAliasAnchorFixture))
	if err != nil {
		t.Fatal(err)
	}
	anchors, err := pathAliasInputs(raw, 24)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range anchors {
		if !reflect.DeepEqual(suites[1].Cases[i].Inputs, []any{row.Left, row.Right}) {
			t.Fatal("all original24 prefix inputs/order", i)
		}
	}
	for _, s := range suites {
		if !isPathAliasSuite(s) {
			t.Fatal("new entry predicate")
		}
	}
	for _, s := range []suite{{Package: pathCleanPackage, Activity: "Ext"}, {Package: pathAliasPackage, Activity: "Other"}, {Package: "go/other", Activity: pathCleanActivity}} {
		if isPathAliasSuite(s) {
			t.Fatal("unrelated suite admitted")
		}
	}
}

func pathAliasTestCleanRaw(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../..", pathCleanFixture))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPathCleanInputLoaderRejectsChangedOrderKindsAndLateFailure(t *testing.T) {
	raw := pathAliasTestCleanRaw(t)
	if rows, err := pathCleanInputs(raw); err != nil || len(rows) != 9841 {
		t.Fatal("positive full input base", err)
	}
	for _, mutation := range []string{"null", "boolean", "swapped", "float-bound", "wrong-alphabet", "missing", "unknown", "late-invalid"} {
		v, err := decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		top := v.(map[string]any)
		values := top["texts"].([]any)
		switch mutation {
		case "null":
			values[0] = nil
		case "boolean":
			values[0] = false
		case "swapped":
			values[1], values[2] = values[2], values[1]
		case "float-bound":
			top["maximum_length"] = json.Number("8.0")
		case "wrong-alphabet":
			top["alphabet"] = []string{"/", ".", "a"}
		case "missing":
			delete(top, "order")
		case "unknown":
			top["expected"] = []any{}
		case "late-invalid":
			values[len(values)-1] = strings.Repeat("a", 1025)
		}
		changed, err := json.Marshal(top)
		if err != nil {
			t.Fatal(err)
		}
		if rows, err := pathCleanInputs(changed); err == nil || rows != nil {
			t.Fatal("invalid population returned partial/admitted rows", mutation)
		}
	}
}

func TestPathAliasPairFixtureStrictBoundaryAndPinnedReader(t *testing.T) {
	const before = `{"schema":"gooo/path-alias-inputs/v1","cases":[{"name":"one","inputs":{"left":`
	const after = `,"right":"b"}}]}`
	for _, value := range []string{`""`, `"\ud83d\ude42"`, `"\\ud800"`, `"�"`} {
		if _, err := pathAliasInputs([]byte(before+value+after), 1); err != nil {
			t.Fatal("valid raw Unicode base", err)
		}
	}
	for _, value := range []string{`null`, `false`, `1`, `1.0`, `[]`, `{}`, `"\ud800"`, `"\udc00"`, `"\ud800\u0041"`} {
		if rows, err := pathAliasInputs([]byte(before+value+after), 1); err == nil || rows != nil {
			t.Fatal("wrong Text kind/Unicode admitted", value)
		}
	}
	for _, raw := range [][]byte{
		[]byte(before + `"a"` + after + `{}`),
		[]byte(`{"schema":"gooo/path-alias-inputs/v1","schema":"gooo/path-alias-inputs/v1","cases":[]}`),
		[]byte(`{"schema":"gooo/path-alias-inputs/v1","cases":[{"name":"one","inputs":{"left":"a","le\u0066t":"x","right":"b"}}]}`),
		[]byte(`{"schema":"gooo/path-alias-inputs/v1","cases":[{"name":"one","inputs":{"left":"a"}}]}`),
		[]byte(`{"schema":"gooo/path-alias-inputs/v1","cases":[{"name":"one","inputs":{"left":"a","right":"b","extra":false}}]}`),
		append(append([]byte(before+`"`), 0xff), []byte(`"`+after)...),
	} {
		if rows, err := pathAliasInputs(raw, 1); err == nil || rows != nil {
			t.Fatal("malformed raw fixture admitted")
		}
	}
	late := []byte(`{"schema":"gooo/path-alias-inputs/v1","cases":[{"name":"one","inputs":{"left":"a","right":"b"}},{"name":"two","inputs":{"left":"x","right":false}}]}`)
	if rows, err := pathAliasInputs(late, 2); err == nil || rows != nil {
		t.Fatal("late malformed row returned partial population")
	}
	duplicate := []byte(`{"schema":"gooo/path-alias-inputs/v1","cases":[{"name":"one","inputs":{"left":"a","right":"b"}},{"name":"two","inputs":{"left":"a","right":"b"}}]}`)
	if _, err := pathAliasInputs(duplicate, 2); err == nil {
		t.Fatal("duplicate input pair")
	}
	dir := t.TempDir()
	name := filepath.Join(dir, "fixture.json")
	good := []byte(before + `"a"` + after)
	if err := os.WriteFile(name, good, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := decimalPayloadReadFixture(dir, "fixture.json", strings.TrimPrefix(contentDigest(good), "sha256:")); err != nil {
		t.Fatal("positive pinned regular fixture", err)
	}
	if _, err := decimalPayloadReadFixture(dir, "fixture.json", strings.Repeat("0", 64)); err == nil {
		t.Fatal("changed fixture hash")
	}
	if err := os.Symlink(name, filepath.Join(dir, "alias.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := decimalPayloadReadFixture(dir, "alias.json", strings.TrimPrefix(contentDigest(good), "sha256:")); err == nil {
		t.Fatal("symlink fixture admitted")
	}
}
