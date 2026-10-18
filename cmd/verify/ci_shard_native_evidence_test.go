package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// Synthetic receipts exercise the reader. They are not native executions.
func ciShardNativeEvidenceFixture(t *testing.T, rows int) (ciShardPlan, ciShardNativeInventory, string, ciShardSuiteRecord) {
	t.Helper()
	raw, s, identity := fixture(t)
	row := s.Cases[0]
	s.Cases = make([]testCase, rows)
	for i := range s.Cases {
		s.Cases[i] = row
	}
	contract, err := contractFor(s)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("../..", contract.Source))
	if err != nil {
		t.Fatal(err)
	}
	selected := suiteWorkspace{Names: []string{contract.Source}, Sources: map[string][]byte{contract.Source: source}, Identities: identity.Sources}
	selected.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	selected.Manifest.Entry.Package = s.Package
	selected.Manifest.Entry.Activity = s.Activity
	selected.Manifest.Packages = []sourcePackage{{Path: s.Package, Sources: selected.Names}}
	manifest, err := json.MarshalIndent(selected.Manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifest = append(manifest, '\n')
	_, run, sources, _ := ciShardTestInputs(t, []string{"go/synthetic:A"})
	delete(sources, "ports/synthetic/0.gooo")
	sources[contract.Source] = ciShardSource{Mode: "100644", Bytes: int64(len(source)), SHA256: contentDigest(source)}
	run.CompilerRevision = identity.CompilerRevision
	c := &NativeVerificationContract{Profiles: ContractProfiles{}, Sources: []ContractSource{{Path: contract.Source, Mode: "100644"}}, Suites: []ContractSuite{{Index: 0, Package: s.Package, Activity: s.Activity, Cases: int64(rows), Batches: int64((rows + 127) / 128), SourcePaths: selected.Names}}, TotalCases: int64(rows), TotalBatches: int64((rows + 127) / 128)}
	anchors := []ciShardSourceAnchor{{Name: contract.Source, RawSHA256: contentDigest(source), Identity: identity.Sources[contract.Source]}}
	anchorDigest, err := canonicalContractDigest(anchors)
	if err != nil {
		t.Fatal(err)
	}
	w := ciShardWorkspace{OriginalIndex: 0, ManifestSHA256: contentDigest(manifest), SourceSetSHA256: anchorDigest, EntryProducer: contract.ID, Sources: anchors}
	plan, err := newCIShardPlan(c, contentDigest([]byte("synthetic independent ledger")), run, 1, sources, []ciShardWorkspace{w})
	if err != nil {
		t.Fatal(err)
	}
	lock := upstream{CompilerRevision: identity.CompilerRevision, GoVersion: identity.GoVersion}
	inventory := ciShardNativeInventory{Suites: []suite{s}, Lock: lock, Workspaces: []suiteWorkspace{selected}}
	out := t.TempDir()
	policy, err := nativeCaseProfile("").policy(s, selected, lock)
	if err != nil {
		t.Fatal(err)
	}
	index := 0
	for start := 0; start < rows; {
		prepared, err := policy.prepare(s, start)
		if err != nil {
			t.Fatal(err)
		}
		batch := s
		batch.Cases = s.Cases[start:prepared.End]
		cases, err := ciShardNativeCaseBytes(batch, prepared)
		if err != nil {
			t.Fatal(err)
		}
		dir := ciShardBatchDirectory(out, 0, 0, index)
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(contract.Source)), 0700); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"cases.json": cases, "gooo.workspace.json": manifest, contract.Source: source} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		var e envelope
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatal(err)
		}
		e.ManifestDigest = contentDigest(manifest)
		e.CasesDigest = contentDigest(cases)
		trace := e.Result.Runtime.Traces[0]
		e.Result.Runtime.Traces = nil
		for i := range batch.Cases {
			trace.Index = i
			e.Result.Runtime.Traces = append(e.Result.Runtime.Traces, trace)
		}
		count := len(batch.Cases)
		e.Result.Runtime.Total = &count
		e.Result.Runtime.Passed = &count
		execution, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "execution.json"), execution, 0600); err != nil {
			t.Fatal(err)
		}
		var replay map[string]any
		if err := json.Unmarshal(execution, &replay); err != nil {
			t.Fatal(err)
		}
		replay["replayed_from_sha256"] = contentDigest(execution)
		replay["result"].(map[string]any)["replay"] = map[string]any{"model_calls": 0}
		replayRaw, err := json.Marshal(replay)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "replay.json"), replayRaw, 0600); err != nil {
			t.Fatal(err)
		}
		start = prepared.End
		index++
	}
	return plan, inventory, out, ciShardSuiteRecord{OriginalIndex: 0, Package: s.Package, Activity: s.Activity, State: "VERIFIED", Batches: []ciShardBatch{}, Exhaustive: []ciShardExhaustive{}}
}

func TestCIShardNativeRawReaderChecksFreshBytesAndExactPartitions(t *testing.T) {
	plan, inventory, out, reported := ciShardNativeEvidenceFixture(t, 129)
	record, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, out, reported)
	if err != nil {
		t.Fatal(err)
	}
	if record.State != "VERIFIED" || len(record.Batches) != 2 || record.Batches[0].Start != 0 || record.Batches[0].End != 128 || record.Batches[1].Start != 128 || record.Batches[1].End != 129 {
		t.Fatal(record)
	}
	if err := validateCIShardSuite(plan, 0, record); err != nil {
		t.Fatal(err)
	}
	// The same reader must check a partial after JSON encoding/decoding as well.
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var restored ciShardSuiteRecord
	if err := decodeCIShardJSON(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, out, restored); err != nil {
		t.Fatal(err)
	}
}

func TestCIShardNativeRawReaderRejectsMeaningfulMutations(t *testing.T) {
	mutations := map[string]func(*testing.T, string, *ciShardSuiteRecord){
		"changed input bytes": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			p := filepath.Join(ciShardBatchDirectory(out, 0, 0, 0), "cases.json")
			b, _ := os.ReadFile(p)
			ciShardNativeWrite(t, p, append(b, ' '))
		},
		"changed manifest": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			ciShardNativeWrite(t, filepath.Join(ciShardBatchDirectory(out, 0, 0, 0), "gooo.workspace.json"), []byte("{}\n"))
		},
		"changed copied source": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			ciShardNativeWrite(t, filepath.Join(ciShardBatchDirectory(out, 0, 0, 0), "ports/unicode/utf16.gooo"), []byte("changed\n"))
		},
		"truncated receipt": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			ciShardNativeWrite(t, filepath.Join(ciShardBatchDirectory(out, 0, 0, 0), "execution.json"), []byte("{"))
		},
		"duplicate receipt key": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			p := filepath.Join(ciShardBatchDirectory(out, 0, 0, 0), "execution.json")
			b, _ := os.ReadFile(p)
			b = bytes.Replace(b, []byte(`"decision":"PASS"`), []byte(`"decision":"PASS","decision":"PASS"`), 1)
			ciShardNativeWrite(t, p, b)
		},
		"wrong producer": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			ciShardNativeMutateReceipt(t, out, "execution.json", func(e *envelope) { e.Result.Runtime.Producer = strings.Repeat("f", 40) })
		},
		"wrong result type": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			ciShardNativeMutateReceipt(t, out, "execution.json", func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Actual = json.RawMessage(`"65533"`) })
		},
		"wrong replay raw parent": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			ciShardNativeMutateReceipt(t, out, "replay.json", func(e *envelope) { e.ReplayedFrom = contentDigest([]byte("wrong parent")) })
		},
		"misreported boundary": func(_ *testing.T, _ string, r *ciShardSuiteRecord) {
			r.Batches = []ciShardBatch{{Index: 0, Start: 1, End: 2}}
		},
		"duplicate batch namespace": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			if err := os.Mkdir(ciShardBatchDirectory(out, 0, 0, 1), 0700); err != nil {
				t.Fatal(err)
			}
		},
		"noncanonical batch namespace": func(t *testing.T, out string, _ *ciShardSuiteRecord) {
			if err := os.Rename(ciShardBatchDirectory(out, 0, 0, 0), filepath.Join(ciShardSuiteDirectory(out, 0, 0), "batch-0")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			plan, inventory, out, reported := ciShardNativeEvidenceFixture(t, 1)
			mutate(t, out, &reported)
			record, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, out, reported)
			if err == nil || record.State == "VERIFIED" {
				t.Fatal("invalid raw evidence accepted", record, err)
			}
		})
	}
}

func TestCIShardNativeRawReaderRetainsOnlyCompletedPrefix(t *testing.T) {
	plan, inventory, out, reported := ciShardNativeEvidenceFixture(t, 129)
	if err := os.Remove(filepath.Join(ciShardBatchDirectory(out, 0, 0, 1), "replay.json")); err != nil {
		t.Fatal(err)
	}
	reported.State = "FAILED"
	reported.Error = "original verifier replay failure"
	record, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, out, reported)
	if err == nil || record.State != "FAILED" || len(record.Batches) != 1 || record.Batches[0].End != 128 {
		t.Fatal("incomplete batch entered prefix", record, err)
	}
	if _, err := os.Stat(filepath.Join(ciShardBatchDirectory(out, 0, 0, 1), "execution.json")); err != nil {
		t.Fatal("failure evidence removed", err)
	}
}

func TestCIShardNativeRawReaderRejectsMissingEarlierBatch(t *testing.T) {
	plan, inventory, out, reported := ciShardNativeEvidenceFixture(t, 129)
	if err := os.RemoveAll(ciShardBatchDirectory(out, 0, 0, 0)); err != nil {
		t.Fatal(err)
	}
	reported.State = "FAILED"
	reported.Error = "original verifier failed"
	if record, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, out, reported); err == nil || len(record.Batches) != 0 || record.State == "VERIFIED" {
		t.Fatal("gap was accepted as a prefix", record, err)
	}
}

func TestCIShardNativeInventoryUsesOriginalProviderOrderWithoutExecutingIt(t *testing.T) {
	providers := func(path, function string) []string {
		t.Helper()
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		found := false
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Name.Name != function {
				continue
			}
			found = true
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := call.Fun.(*ast.Ident)
				if ok && (name.Name == "corpus" || strings.HasSuffix(name.Name, "Corpus")) {
					names = append(names, name.Name)
				}
				return true
			})
		}
		if !found || len(names) == 0 {
			t.Fatal("original corpus providers not found", path)
		}
		return names
	}
	want := providers("main.go", "run")
	got := providers("ci_shard_native_evidence.go", "loadCIShardNativeInventory")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatal("native shard replaced or reordered the full verifier's Go oracles", got, want)
	}
}

func TestCIShardNativeEvidenceRejectsSymlinksAndOutsidePaths(t *testing.T) {
	for _, target := range []string{"root", "suite", "batch", "file"} {
		t.Run(target, func(t *testing.T) {
			plan, inventory, out, reported := ciShardNativeEvidenceFixture(t, 1)
			path := out
			switch target {
			case "suite":
				path = ciShardSuiteDirectory(out, 0, 0)
			case "batch":
				path = ciShardBatchDirectory(out, 0, 0, 0)
			case "file":
				path = filepath.Join(ciShardBatchDirectory(out, 0, 0, 0), "cases.json")
			}
			moved := path + "-outside"
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(moved, path); err != nil {
				t.Fatal(err)
			}
			if r, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, out, reported); err == nil || r.State == "VERIFIED" {
				t.Fatal("symlink accepted")
			}
		})
	}
	reader, err := newCIShardEvidenceReader(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside", "/tmp/outside", "a/../../outside"} {
		if _, err := reader.read(name, 1024); err == nil {
			t.Fatal("outside namespace accepted", name)
		}
	}
}

func TestCIShardNativeEvidenceDetectsChangedFilesAfterInspection(t *testing.T) {
	out := t.TempDir()
	path := filepath.Join(out, "cases.json")
	ciShardNativeWrite(t, path, []byte("first\n"))
	reader, err := newCIShardEvidenceReader(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.read("cases.json", 1024); err != nil {
		t.Fatal(err)
	}
	ciShardNativeWrite(t, path, []byte("later\n"))
	if err := reader.finish(); err == nil {
		t.Fatal("changed input after inspection accepted")
	}
}

func TestCIShardNativeFailureMetadataRemainsBoundedWithoutLosingRawError(t *testing.T) {
	err := errors.New(strings.Repeat("한", 3000))
	message := ciShardNativeErrorMessage(err)
	if len(message) > 4096 || !utf8.ValidString(message) || !strings.Contains(message, "raw evidence") || len(err.Error()) != 9000 {
		t.Fatal("failure metadata bound or original diagnosis changed")
	}
}

func ciShardNativeWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func ciShardNativeMutateReceipt(t *testing.T, out, name string, mutate func(*envelope)) {
	t.Helper()
	p := filepath.Join(ciShardBatchDirectory(out, 0, 0, 0), name)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	mutate(&e)
	raw, err = json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	ciShardNativeWrite(t, p, raw)
}
