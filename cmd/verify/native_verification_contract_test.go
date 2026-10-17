package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var syntheticPrefix = FrozenGuard{strings.Repeat("a", 40), 2, "sha256:56c72db9fea353441caa2dadee5a5cf44cf34f09094c192dcda48bfbe4639c64"}
var syntheticSources = FrozenGuard{strings.Repeat("b", 40), 2, "sha256:83e811c8e6d05c680ab07ec103a03aff3f3549fa195ba16bb355c1fe2a353429"}

func TestProductionEmbeddedContractSourceRoles(t *testing.T) {
	c, err := nativeExpectedVerificationContract()
	if err != nil {
		t.Fatal(err)
	}
	names := nativeVerificationTimingSourceNames(c)
	wanted := map[string]bool{}
	for _, source := range c.Sources {
		for _, role := range source.Roles {
			if role == "explicit-observer-input" || role == "contract-reader" {
				wanted[source.Path] = true
			}
		}
	}
	if len(names) != len(wanted) {
		t.Fatal("role union duplicated or omitted an input")
	}
	for i, name := range names {
		if !wanted[name] || i > 0 && names[i-1] >= name {
			t.Fatal("wrong or unordered timing role union", name)
		}
	}
	// The source identity recorder's global fixture roster has the same origin.
	fixtureNames := mustNativeVerificationTimingSourceNames()
	if !bytes.Equal(mustTestJSON(t, names), mustTestJSON(t, fixtureNames)) {
		t.Fatal("fixture source roster differs")
	}
}

func productionDocument(t *testing.T) map[string]any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(nativeVerificationContractJSON))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func productionMustRejectGenericValid(t *testing.T, raw []byte) {
	t.Helper()
	if _, err := LoadNativeVerificationContract(raw, nativeFrozenPrefixGuard(), nativeFrozenSourceGuard()); err != nil {
		t.Fatal("control is not a valid generic ledger", err)
	}
	if _, err := nativeExpectedVerificationContractFrom(raw); err == nil || !strings.Contains(err.Error(), "version bytes") {
		t.Fatal("changed production version accepted", err)
	}
}

func TestProductionVersionFreezesCurrentSuffixAndAllReaderPaths(t *testing.T) {
	productionMustRejectGenericValid(t, append(append([]byte{}, nativeVerificationContractJSON...), '\n'))
	m := productionDocument(t)
	rows := m["suites"].([]any)
	last := rows[len(rows)-1].(map[string]any)
	cases, _ := last["cases"].(json.Number).Int64()
	last["cases"] = cases + 1
	productionMustRejectGenericValid(t, mustTestJSON(t, m))
	c, err := nativeExpectedVerificationContract()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range c.Sources {
		if source.Baseline {
			continue
		}
		for _, role := range source.Roles {
			if role != "contract-reader" {
				continue
			}
			t.Run(source.Path, func(t *testing.T) {
				m := productionDocument(t)
				rows := m["sources"].([]any)
				kept := []any{}
				for _, row := range rows {
					if row.(map[string]any)["path"] != source.Path {
						kept = append(kept, row)
					}
				}
				if len(kept) != len(rows)-1 {
					t.Fatal("mutation did not remove exactly one reader")
				}
				m["sources"] = kept
				productionMustRejectGenericValid(t, mustTestJSON(t, m))
			})
		}
	}
}

func TestProductionDiskLedgerBytesAreExact(t *testing.T) {
	if err := nativeVerificationCheckDiskLedger(append([]byte{}, nativeVerificationContractJSON...)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{append(append([]byte{}, nativeVerificationContractJSON...), '\n'), mustTestJSON(t, productionDocument(t))} {
		if bytes.Equal(raw, nativeVerificationContractJSON) {
			t.Fatal("control did not change ledger raw bytes")
		}
		if err := nativeVerificationCheckDiskLedger(raw); err == nil {
			t.Fatal("changed on-disk bytes accepted")
		}
	}
}

func TestProductionKnownRegularGitModes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.go")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := nativeVerificationCheckSourceMode(path, "100644"); err != nil {
		t.Fatal("non-executable rw permissions are not a Git mode change", err)
	}
	if err := nativeVerificationCheckSourceMode(path, "120000"); err == nil {
		t.Fatal("unsupported expected Git mode accepted")
	}
	if err := nativeVerificationCheckSourceMode(root, "100644"); err == nil {
		t.Fatal("directory accepted")
	}
	if runtime.GOOS == "windows" {
		return
	} // Git executable-bit verification is a separate caller boundary here.
	for _, mode := range []os.FileMode{0645, 0654, 0744} {
		for _, expected := range []string{"100644", "100755"} {
			t.Run(strconv.FormatUint(uint64(mode), 8)+"/"+expected, func(t *testing.T) {
				if err := os.Chmod(path, mode); err != nil {
					t.Fatal(err)
				}
				want := (mode&0100 != 0) == (expected == "100755")
				err := nativeVerificationCheckSourceMode(path, expected)
				if (err == nil) != want {
					t.Fatalf("Git owner-exec mode %04o / %s accepted=%v wanted=%v: %v", mode, expected, err == nil, want, err)
				}
			})
		}
	}
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := nativeVerificationCheckSourceMode(path, "100755"); err != nil {
		t.Fatal(err)
	}
	if err := nativeVerificationCheckSourceMode(path, "100644"); err == nil {
		t.Fatal("unexpected executable bit accepted")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := nativeVerificationCheckSourceMode(path, "100755"); err == nil {
		t.Fatal("missing executable bit accepted")
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if err := nativeVerificationCheckSourceMode(alias, "100644"); err == nil {
		t.Fatal("symlink accepted as regular source")
	}
}

// Only three synthetic suites; these literals are not production expected data.
const syntheticContract = `{
"schema":"gooo/native-verification-contract/v1",
"profiles":{"case":"uuid-and-literal-report-compact-v1","workspace":"dependency-closure-v1"},
"prefix":{"commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","count":2,"sha256":"sha256:56c72db9fea353441caa2dadee5a5cf44cf34f09094c192dcda48bfbe4639c64"},
"suites":[
{"index":0,"package":"go/a","activity":"First","cases":3,"batches":2,"source_paths":["ports/a.gooo"]},
{"index":1,"package":"go/b","activity":"Second","cases":5,"batches":3,"source_paths":["ports/b.gooo"]},
{"index":2,"package":"app/c","activity":"Third","cases":2,"batches":1,"source_paths":["ports/a.gooo","ports/b.gooo"]}],
"source_baseline":{"commit":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","count":2,"sha256":"sha256:83e811c8e6d05c680ab07ec103a03aff3f3549fa195ba16bb355c1fe2a353429"},
"sources":[
{"path":"ports/a.gooo","mode":"100644","owner":"native-ci","roles":["catalog-source"],"baseline":true},
{"path":"ports/b.gooo","mode":"100755","owner":"native-ci","roles":["catalog-source"],"baseline":true},
{"path":"tools/reader.go","mode":"100644","owner":"native-ci","roles":["contract-reader","explicit-observer-input"],"baseline":false}]}`

func syntheticDocument(t *testing.T) map[string]any {
	t.Helper()
	d := json.NewDecoder(strings.NewReader(syntheticContract))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}
func suiteRow(m map[string]any, i int) map[string]any { return m["suites"].([]any)[i].(map[string]any) }
func sourceRow(m map[string]any, i int) map[string]any {
	return m["sources"].([]any)[i].(map[string]any)
}
func rejectRaw(t *testing.T, raw []byte, contains string) {
	t.Helper()
	if _, err := LoadNativeVerificationContract(raw, syntheticPrefix, syntheticSources); err == nil || !strings.Contains(err.Error(), contains) {
		t.Fatalf("wanted rejection containing %q, got %v", contains, err)
	}
}
func rejectMutation(t *testing.T, mutate func(map[string]any), contains string) {
	t.Helper()
	m := syntheticDocument(t)
	mutate(m)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	rejectRaw(t, raw, contains)
}

func TestLoadSyntheticAndCanonicalDigests(t *testing.T) {
	c, err := LoadNativeVerificationContract([]byte(syntheticContract), syntheticPrefix, syntheticSources)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Suites) != 3 || len(c.Sources) != 3 || c.TotalCases != 10 || c.TotalBatches != 6 || c.TimingEvents != 14 || c.Sources[1].Mode != "100755" {
		t.Fatalf("wrong synthetic result: %+v", c)
	}
	prefix, err := canonicalContractDigest(c.Suites[:2])
	if err != nil || prefix != syntheticPrefix.SHA256 {
		t.Fatal(prefix, err)
	}
	digest, err := canonicalContractDigest([]baselineSourceIdentity{{"ports/a.gooo", "100644"}, {"ports/b.gooo", "100755"}})
	if err != nil || digest != syntheticSources.SHA256 {
		t.Fatal(digest, err)
	}
	// Explicit future regular files may use either mode without moving baseline.
	m := syntheticDocument(t)
	sourceRow(m, 2)["mode"] = "100755"
	raw, _ := json.Marshal(m)
	if _, err := LoadNativeVerificationContract(raw, syntheticPrefix, syntheticSources); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalHTMLVector(t *testing.T) {
	digest, err := canonicalContractDigest([]ContractSuite{{0, "go/<&>", "Check", 1, 1, []string{"ports/<&>.gooo"}}})
	if err != nil || digest != "sha256:eeb901b3c57b40ee095a555f76e72ff041ebca76afbe5b21108683e546d9b8d5" {
		t.Fatal(digest, err)
	}
}

func TestStrictJSONEnvelope(t *testing.T) {
	raw := []byte(syntheticContract)
	cases := map[string][]byte{
		"duplicate-root":        bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"x","schema":`), 1),
		"decoded-key-duplicate": bytes.Replace(raw, []byte(`"schema":`), []byte(`"sche\u006da":"x","schema":`), 1),
		"duplicate-nested":      bytes.Replace(raw, []byte(`"case":`), []byte(`"case":"x","case":`), 1),
		"duplicate-row":         bytes.Replace(raw, []byte(`"index":0`), []byte(`"index":0,"index":0`), 1),
		"trailing-json":         append(append([]byte{}, raw...), []byte(` {}`)...),
		"trailing-junk":         append(append([]byte{}, raw...), byte('!')),
		"invalid-utf8":          bytes.Replace(raw, []byte("go/a"), []byte{'g', 'o', '/', 0xff}, 1),
		"empty":                 {}, "null": []byte(`null`), "array": []byte(`[]`),
		"deep-invalid":        []byte(strings.Repeat("[", 1000) + strings.Repeat("]", 1000)),
		"negative-zero-index": bytes.Replace(raw, []byte(`"index":0`), []byte(`"index":-0`), 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) { rejectRaw(t, input, "") })
	}
	padded := append(append([]byte{}, raw...), bytes.Repeat([]byte{' '}, MaxContractBytes-len(raw))...)
	if _, err := LoadNativeVerificationContract(padded, syntheticPrefix, syntheticSources); err != nil {
		t.Fatal("exact1MiB", err)
	}
	rejectRaw(t, append(padded, ' '), "1MiB")
}

func TestUnknownFieldsAtEveryObjectLevel(t *testing.T) {
	for _, name := range []string{"root", "profiles", "prefix", "source_baseline", "suite", "source", "case-sensitive"} {
		t.Run(name, func(t *testing.T) {
			rejectMutation(t, func(m map[string]any) {
				var target map[string]any
				switch name {
				case "root", "case-sensitive":
					target = m
				case "suite":
					target = suiteRow(m, 0)
				case "source":
					target = sourceRow(m, 0)
				default:
					target = m[name].(map[string]any)
				}
				key := "unknown"
				if name == "case-sensitive" {
					key = "Schema"
				}
				target[key] = true
			}, "fields")
		})
	}
}

func TestExactIntegersAndOverflow(t *testing.T) {
	for _, key := range []string{"index", "cases", "batches"} {
		for _, value := range []any{true, false, json.Number("1.0"), json.Number("1e0"), json.Number("-1"), json.Number("9223372036854775808"), "1", nil} {
			t.Run(key+"/"+string(mustTestJSON(t, value)), func(t *testing.T) { rejectMutation(t, func(m map[string]any) { suiteRow(m, 2)[key] = value }, "") })
		}
	}
	for _, key := range []string{"cases", "batches"} {
		t.Run("zero-"+key, func(t *testing.T) {
			rejectMutation(t, func(m map[string]any) { suiteRow(m, 2)[key] = json.Number("0") }, "")
		})
	}
	rejectMutation(t, func(m map[string]any) { suiteRow(m, 2)["cases"] = json.Number("9223372036854775807") }, "overflow")
	rejectMutation(t, func(m map[string]any) {
		suiteRow(m, 2)["cases"] = json.Number("9223372036854775799")
		suiteRow(m, 2)["batches"] = json.Number("9223372036854775799")
	}, "overflow")
	rejectMutation(t, func(m map[string]any) { suiteRow(m, 2)["batches"] = json.Number("3") }, "suite")
	for _, key := range []string{"prefix", "source_baseline"} {
		for _, v := range []any{true, json.Number("2.0"), json.Number("2e0")} {
			rejectMutation(t, func(m map[string]any) { m[key].(map[string]any)["count"] = v }, "metadata")
		}
	}
}

func mustTestJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestFrozenAuthorityAndSameTotalsMutations(t *testing.T) {
	rejectMutation(t, func(m map[string]any) {
		suiteRow(m, 0)["cases"] = json.Number("4")
		suiteRow(m, 1)["cases"] = json.Number("4")
	}, "prefix digest")
	rejectMutation(t, func(m map[string]any) { a := m["suites"].([]any); a[0], a[1] = a[1], a[0] }, "suite")
	rejectMutation(t, func(m map[string]any) {
		a := m["suites"].([]any)
		a[0], a[1] = a[1], a[0]
		suiteRow(m, 0)["index"] = json.Number("0")
		suiteRow(m, 1)["index"] = json.Number("1")
	}, "prefix digest")
	rejectMutation(t, func(m map[string]any) { suiteRow(m, 0)["source_paths"] = []any{"ports/b.gooo"} }, "prefix digest")
	for _, guard := range []string{"prefix", "source_baseline"} {
		for _, field := range []string{"commit", "count", "sha256"} {
			t.Run(guard+"/"+field, func(t *testing.T) {
				rejectMutation(t, func(m map[string]any) {
					g := m[guard].(map[string]any)
					switch field {
					case "commit":
						g[field] = strings.Repeat("c", 40)
					case "count":
						g[field] = json.Number("1")
					case "sha256":
						g[field] = "sha256:" + strings.Repeat("0", 64)
					}
				}, "external guard")
			})
		}
	}
	for _, g := range []FrozenGuard{{"bad", 2, syntheticPrefix.SHA256}, {syntheticPrefix.Commit, 0, syntheticPrefix.SHA256}, {syntheticPrefix.Commit, 2, strings.Repeat("0", 64)}} {
		if _, err := LoadNativeVerificationContract([]byte(syntheticContract), g, syntheticSources); err == nil {
			t.Fatal("invalid external guard accepted")
		}
	}
}

func TestSourceAndSuiteMembership(t *testing.T) {
	cases := map[string]func(map[string]any){
		"missing-baseline":     func(m map[string]any) { m["sources"] = append(m["sources"].([]any)[:1], m["sources"].([]any)[2:]...) },
		"baseline-flags-moved": func(m map[string]any) { sourceRow(m, 1)["baseline"] = false; sourceRow(m, 2)["baseline"] = true },
		"same-count-path-substitution": func(m map[string]any) {
			sourceRow(m, 1)["path"] = "ports/c.gooo"
			suiteRow(m, 1)["source_paths"] = []any{"ports/c.gooo"}
		},
		"baseline-executable-bit":  func(m map[string]any) { sourceRow(m, 1)["mode"] = "100644" },
		"forged-mode":              func(m map[string]any) { sourceRow(m, 0)["mode"] = "100755" },
		"symlink-mode":             func(m map[string]any) { sourceRow(m, 2)["mode"] = "120000" },
		"directory-mode":           func(m map[string]any) { sourceRow(m, 2)["mode"] = "040000" },
		"source-order":             func(m map[string]any) { a := m["sources"].([]any); a[0], a[1] = a[1], a[0] },
		"source-duplicate":         func(m map[string]any) { sourceRow(m, 1)["path"] = "ports/a.gooo" },
		"unknown-role":             func(m map[string]any) { sourceRow(m, 2)["roles"] = []any{"unknown"} },
		"duplicate-role":           func(m map[string]any) { sourceRow(m, 2)["roles"] = []any{"contract-reader", "contract-reader"} },
		"unsorted-role":            func(m map[string]any) { sourceRow(m, 2)["roles"] = []any{"explicit-observer-input", "contract-reader"} },
		"empty-role":               func(m map[string]any) { sourceRow(m, 2)["roles"] = []any{} },
		"missing-source-role":      func(m map[string]any) { sourceRow(m, 0)["roles"] = []any{"contract-reader"} },
		"owner":                    func(m map[string]any) { sourceRow(m, 2)["owner"] = "other" },
		"baseline-not-bool":        func(m map[string]any) { sourceRow(m, 2)["baseline"] = json.Number("0") },
		"missing-suite-source":     func(m map[string]any) { suiteRow(m, 2)["source_paths"] = []any{"ports/missing.gooo"} },
		"unqualified-suite-source": func(m map[string]any) { suiteRow(m, 2)["source_paths"] = []any{"tools/reader.go"} },
		"duplicate-suite-source":   func(m map[string]any) { suiteRow(m, 2)["source_paths"] = []any{"ports/a.gooo", "ports/a.gooo"} },
		"empty-suite-source":       func(m map[string]any) { suiteRow(m, 2)["source_paths"] = []any{} },
		"duplicate-suite-key":      func(m map[string]any) { suiteRow(m, 2)["package"] = "go/a"; suiteRow(m, 2)["activity"] = "First" },
		"bad-activity":             func(m map[string]any) { suiteRow(m, 2)["activity"] = "é" },
		"profile":                  func(m map[string]any) { m["profiles"].(map[string]any)["case"] = "legacy" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) { rejectMutation(t, mutate, "") })
	}
	for _, path := range []string{"", "/abs", "a/../b", "a/./b", "../b", "a//b", "a/", "a\\b", "a:b", "a\x00b", "a\x7fb", "é", "a\nb"} {
		t.Run("unsafe-source/"+path, func(t *testing.T) { rejectMutation(t, func(m map[string]any) { sourceRow(m, 2)["path"] = path }, "") })
		t.Run("unsafe-package/"+path, func(t *testing.T) { rejectMutation(t, func(m map[string]any) { suiteRow(m, 2)["package"] = path }, "") })
	}
}
