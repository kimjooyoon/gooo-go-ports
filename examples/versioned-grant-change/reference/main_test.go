package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// These arbitrary typed sentinels test the writer, not the grant policy.
func fixtureRow(t *testing.T, marker string) json.RawMessage {
	t.Helper()
	inputs := map[string]any{}
	for i := 0; i < 6; i++ {
		inputs[entry+".input"+strconv.Itoa(i)] = marker
	}
	expected := map[string]any{
		"status": "fixture-only", "reason": "opaque-sentinel", "expected_validator_strict": false,
		"current_validator_strict": true, "if_match_condition": 2, "version_match": false,
		"grant_status": "opaque-grant", "grant_reason": "opaque-reason", "invalid_field": "opaque-field",
		"required_before_match": true, "required_after_match": false, "subset_rule_id": "opaque-rule",
	}
	raw, err := json.Marshal(map[string]any{"inputs": inputs, "expected": map[string]any{entry: expected}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func fixtureDocument(t *testing.T, rows ...json.RawMessage) []byte {
	t.Helper()
	raw, err := json.Marshal(struct {
		Schema string            `json:"schema"`
		Cases  []json.RawMessage `json:"cases"`
	}{"gooo/body-composition-cases/v1", rows})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func editRow(t *testing.T, raw json.RawMessage, edit func(map[string]any)) json.RawMessage {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatal(err)
	}
	edit(row)
	result, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func decodedExpected(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestStrictJSONDecodedDuplicatesAndUnicode(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"a":1,"\u0061":2}`), []byte(`{"nested":{"a":1,"a":2}}`),
		[]byte(`{"s":"\ud800"}`), []byte(`{"s":"\udc00"}`), []byte(`{"s":"\ud800x"}`),
		{'"', 0xff, '"'}, []byte(`{} {}`), []byte(`{"a":NaN}`), []byte(`{"a":01}`),
	} {
		if err := strictJSON(raw); err == nil {
			t.Errorf("accepted invalid representation %q", raw)
		}
	}
	for _, raw := range []string{`{"s":"\ud83e\uddea"}`, `{"s":"\\ud800"}`, `{"s":"\u0000"}`, `{"s":"café 🧪"}`} {
		if err := strictJSON([]byte(raw)); err != nil {
			t.Errorf("valid scalar JSON %s: %v", raw, err)
		}
	}
}

func TestSixRawTextDomainAndTypedExpectedValues(t *testing.T) {
	for port := 0; port < 6; port++ {
		for _, value := range []string{"", strings.Repeat("\x00", 1024), strings.Repeat("é", 512), strings.Repeat("x", 129)} {
			raw := editRow(t, fixtureRow(t, "seed"), func(row map[string]any) { row["inputs"].(map[string]any)[entry+".input"+strconv.Itoa(port)] = value })
			rows, err := decodeRows(fixtureDocument(t, raw))
			if err != nil || rows[0].Inputs[port] != value {
				t.Fatalf("exact raw port%d: %v", port, err)
			}
		}
		for _, value := range []any{nil, false, 1, []any{}, map[string]any{}, strings.Repeat("a", 1025), strings.Repeat("é", 513)} {
			raw := editRow(t, fixtureRow(t, "seed"), func(row map[string]any) { row["inputs"].(map[string]any)[entry+".input"+strconv.Itoa(port)] = value })
			if _, err := decodeRows(fixtureDocument(t, raw)); err == nil {
				t.Fatalf("accepted wrong type/bound port%d", port)
			}
		}
	}
	for _, mutate := range []func(map[string]any){
		func(r map[string]any) { delete(r["inputs"].(map[string]any), entry+".input0") },
		func(r map[string]any) { r["inputs"].(map[string]any)["other"] = "x" },
		func(r map[string]any) { r["expected"].(map[string]any)["other"] = nil },
		func(r map[string]any) { r["expected"].(map[string]any)[entry].(map[string]any)["reason"] = nil },
		func(r map[string]any) { r["expected"].(map[string]any)[entry].(map[string]any)["version_match"] = nil },
	} {
		if _, err := decodeRows(fixtureDocument(t, editRow(t, fixtureRow(t, "s"), mutate))); err == nil {
			t.Fatal("accepted schema/type mutation")
		}
	}
	for _, number := range []string{"1.0", "1e0", "9223372036854775808"} {
		raw := bytes.Replace(fixtureRow(t, "s"), []byte(`"if_match_condition":2`), []byte(`"if_match_condition":`+number), 1)
		if _, err := decodeRows(fixtureDocument(t, raw)); err == nil {
			t.Fatal("expected int64 token accepted", number)
		}
	}
}

func TestOrderedRawMessageWriterAndGreedyBudgets(t *testing.T) {
	original := []json.RawMessage{}
	for i := 0; i < 19; i++ {
		original = append(original, fixtureRow(t, strconv.Itoa(i)))
	}
	master := fixtureDocument(t, original...)
	frozen := bytes.Clone(master)
	rows, err := decodeRows(master)
	if err != nil {
		t.Fatal(err)
	}
	expected := bytes.Clone(rows[0].Expected)
	master[0] = ' '
	if !bytes.Equal(rows[0].Expected, expected) {
		t.Fatal("RawMessage aliases caller buffer")
	}
	master = frozen
	batches, err := prepareBatches(rows, 50000)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 {
		t.Fatalf("max8 expected3 batches, got%d", len(batches))
	}
	position := 0
	for _, batch := range batches {
		if batch.Start != position || batch.End <= batch.Start || batch.End-batch.Start > 8 || len(batch.Raw) > maxRaw || batch.Raw[len(batch.Raw)-1] != '\n' || batch.LoweredUpperBytes > maxLowered {
			t.Fatal("span/budget violation")
		}
		got, err := decodeRows(batch.Raw)
		if err != nil {
			t.Fatal(err)
		}
		_, compact, err := document(rows[batch.Start:batch.End])
		if err != nil {
			t.Fatal(err)
		}
		growth := max(0, len("GoooPackage")+len(strconv.Itoa(50000))+len("Activity")+len(activity)-len(entry))
		if batch.LoweredUpperBytes != len(compact)+7*len(got)*growth {
			t.Fatal("pinned source-lowering bound changed")
		}
		for i, row := range got {
			want := rows[position+i]
			if row.Inputs != want.Inputs || !reflect.DeepEqual(decodedExpected(t, row.Expected), decodedExpected(t, want.Expected)) {
				t.Fatal("row omitted, repeated, reordered or expected rewritten")
			}
		}
		position = batch.End
	}
	if position != 19 || !bytes.Equal(master, frozen) {
		t.Fatal("master changed or incomplete spans")
	}
	large := editRow(t, fixtureRow(t, "s"), func(r map[string]any) {
		r["expected"].(map[string]any)[entry].(map[string]any)["reason"] = strings.Repeat("x", 8000)
	})
	largeRows, err := decodeRows(fixtureDocument(t, large, large, large, large, large, large))
	if err != nil {
		t.Fatal(err)
	}
	cut, err := prepareBatches(largeRows, 50000)
	if err != nil {
		t.Fatal(err)
	}
	if len(cut) < 2 || cut[0].End >= 6 {
		t.Fatal("raw byte budget did not split before max8")
	}
	for _, sourceBytes := range []int{0, maxSources + 1} {
		if _, err := prepareBatches(rows, sourceBytes); err == nil {
			t.Fatal("source bound accepted")
		}
	}
}

func TestIndividuallyOversizedScalarTupleRefusedWithoutRepair(t *testing.T) {
	for _, raw := range []json.RawMessage{
		fixtureRow(t, strings.Repeat("\x00", 1024)),
		editRow(t, fixtureRow(t, "s"), func(r map[string]any) {
			r["expected"].(map[string]any)[entry].(map[string]any)["reason"] = strings.Repeat("x", maxRaw+1)
		}),
	} {
		rows, err := decodeRows(fixtureDocument(t, raw))
		if err != nil {
			t.Fatal(err)
		}
		original := bytes.Clone(rows[0].Raw)
		batches, err := prepareBatches(rows, 50000)
		if err == nil || !strings.Contains(err.Error(), "individual row") || batches != nil || !bytes.Equal(rows[0].Raw, original) {
			t.Fatal("oversized singleton omitted or repaired")
		}
	}
}

func repository(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location")
	}
	root, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(file), "../../.."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func temporaryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func copyFile(t *testing.T, from, to, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(from, name))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(to, name)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestActualOriginalSourceAndThreeLookupAdapterProvenance(t *testing.T) {
	root := repository(t)
	if err := validateProvenance(root); err != nil {
		t.Fatal(err)
	}
	files := []string{"third_party/go/net/http/fs.go.txt", "third_party/go/net/textproto/textproto.go.txt", "third_party/GO-LICENSE", "examples/versioned-grant-change/reference/main.go"}
	for _, changed := range files {
		copy := temporaryRoot(t)
		for _, name := range files {
			copyFile(t, root, copy, name)
		}
		target := filepath.Join(copy, changed)
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if changed == files[3] {
			raw = bytes.Replace(raw, []byte("return condFalse\n}"), []byte("return condTrue\n}"), 1)
		} else {
			raw = append(raw, ' ')
		}
		if err := os.WriteFile(target, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err := validateProvenance(copy); err == nil {
			t.Fatal("altered original/license/adapter accepted", changed)
		}
	}
	got, err := extractFunction([]byte("package main\nvar misleading = \"func f(\"\nfunc f() int {\n return 1\n}\n"), "f")
	if err != nil || !bytes.HasPrefix(got, []byte("func f()")) {
		t.Fatal("function extractor matched quoted text")
	}
	if _, err := adaptedIfMatch([]byte("func checkIfMatch() int {\n return 0\n}\n")); err == nil {
		t.Fatal("adapter accepted missing substitutions")
	}
}

func TestWorkspaceImagesExactSourcePinsAndNoSymlink(t *testing.T) {
	root := repository(t)
	manifest, err := os.ReadFile(filepath.Join(root, "versioned-grant-change.workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, total, err := images(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 12 || total < 1 {
		t.Fatal("incomplete source image")
	}
	for _, image := range got {
		raw, err := os.ReadFile(filepath.Join(root, image.Filename))
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		if image.SourceDigest != "sha256:"+sum(encoded) {
			t.Fatal("source image is not actual Go JSON-string bytes")
		}
	}
	for _, mutation := range []string{
		strings.Replace(string(manifest), `"gooo/package-workspace-manifest/v1"`, `"wrong"`, 1),
		strings.Replace(string(manifest), `"examples/agentmesh-service-policy/domain.gooo"`, `"../outside.gooo"`, 1),
	} {
		if _, _, err := images(root, []byte(mutation)); err == nil {
			t.Fatal("altered manifest accepted")
		}
	}
	copy := temporaryRoot(t)
	for _, image := range got {
		copyFile(t, root, copy, image.Filename)
	}
	// Unknown map keys must not inherit Go's empty-string zero value as a
	// valid package mapping, even when all12 files/nine records still exist.
	var unknown workspace
	if err := json.Unmarshal(manifest, &unknown); err != nil {
		t.Fatal(err)
	}
	unknownName := "examples/versioned-grant-change/unknown.gooo"
	unknown.Packages[len(unknown.Packages)-1].Path = ""
	unknown.Packages[len(unknown.Packages)-1].Sources = []string{unknownName}
	rootSource, err := os.ReadFile(filepath.Join(root, newSource))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copy, unknownName), rootSource, 0600); err != nil {
		t.Fatal(err)
	}
	unknownRaw, err := json.Marshal(unknown)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := images(copy, unknownRaw); err == nil || !strings.Contains(err.Error(), "roster") {
		t.Fatal("unknown source and empty package escaped the exact roster", err)
	}
	name := "examples/agentmesh-service-policy/domain.gooo"
	path := filepath.Join(copy, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := images(copy, manifest); err == nil {
		t.Fatal("changed existing source bytes accepted")
	}
	copyFile(t, root, copy, name)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, name), path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := images(copy, manifest); err == nil {
		t.Fatal("symlink source accepted")
	}
}

func TestOriginalIfMatchQuirksAreComponentObservationsOnly(t *testing.T) {
	for _, row := range []struct {
		selected, current string
		want              condResult
	}{
		{"", `"a"`, condNone}, {"   ", `"a"`, condFalse}, {"*garbage", "bad", condTrue},
		{`"a"bad`, `"a"`, condTrue}, {"\r\n\"a\"\t", `"a"`, condTrue},
		{`W/"a"`, `"a"`, condFalse}, {`"a"`, `W/"a"`, condFalse},
		{`"café 🧪"`, `"café 🧪"`, condFalse}, // Embedded ASCII SPACE is invalid ETag grammar.
		{`"café🧪"`, `"café🧪"`, condTrue},
	} {
		if actual := checkIfMatch(row.selected, row.current); actual != row.want {
			t.Fatalf("original compatibility %q: %v", row.selected, actual)
		}
	}
	if observe("\r\n\"a\"\t").HeaderValid {
		t.Fatal("scanner trimming sanitized raw header validity")
	}
	if !etagStrongMatch(`"a"`, `"a"`) || etagStrongMatch(`W/"a"`, `W/"a"`) {
		t.Fatal("original strong helper")
	}
}

func TestExistingOutputIsPreserved(t *testing.T) {
	root := temporaryRoot(t)
	cases := filepath.Join(root, "cases.json")
	manifest := filepath.Join(root, "workspace.json")
	out := filepath.Join(root, "out")
	for _, p := range []string{cases, manifest} {
		if err := os.WriteFile(p, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(out, "mine")
	if err := os.WriteFile(sentinel, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "--cases", cases, "--manifest", manifest, "--out", out}); err == nil {
		t.Fatal("existing output accepted")
	}
	raw, err := os.ReadFile(sentinel)
	if err != nil || string(raw) != "preserve" {
		t.Fatal("existing output touched")
	}
}
