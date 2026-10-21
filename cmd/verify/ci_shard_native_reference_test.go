package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These are synthetic receipt/command fixtures, not real native executions.
func ciShardReferenceFixture(t *testing.T) (ciShardPlan, ciShardNativeInventory, string, ciShardPartial) {
	t.Helper()
	plan, inventory, root, old := ciShardNativeEvidenceFixture(t, 1)
	inventory.Lock.CompilerVersion = "0.6.37-dev"
	namespace := ciShardBatchNamespace(0, 0, 0)
	dir := filepath.Join(root, filepath.FromSlash(namespace))
	var execution map[string]any
	raw, err := os.ReadFile(filepath.Join(dir, "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &execution); err != nil {
		t.Fatal(err)
	}
	execution["manifest"] = "/producer/owned/" + namespace + "/gooo.workspace.json"
	rt := execution["result"].(map[string]any)["runtime"].(map[string]any)
	rt["go_version"] = "go version " + plan.Run.GoVersion + " " + runtime.GOOS + "/" + runtime.GOARCH
	rt["go_tool_sha256"] = "sha256:" + strings.Repeat("b", 64)
	write := func(name string, v any) []byte {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
		return b
	}
	original := write("execution.json", execution)
	execution["replayed_from_sha256"] = contentDigest(original)
	execution["result"].(map[string]any)["replay"] = map[string]any{"model_calls": 0}
	write("replay.json", execution)
	old.Batches = nil
	record, err := inspectCIShardSuiteEvidence(plan, 0, 0, inventory, root, old)
	if err != nil {
		t.Fatal(err)
	}
	partial := ciShardPartial{Schema: ciShardPartialSchema, PlanSHA256: plan.PlanSHA256, Run: plan.Run, Index: 0, Namespace: ciShardNamespace(0), Sources: plan.Sources, Joined: true, State: "SUCCEEDED", Suites: []ciShardSuiteRecord{record}}
	book, err := newCIShardCommandObservations(plan, 0)
	if err != nil {
		t.Fatal(err)
	}
	observe := book.observer(0)
	for _, phase := range []string{"execute", "replay"} {
		observe(0, phase, unavailableNativeCommandResources(runtime.GOOS, "synthetic waited-callback fixture"))
	}
	commands, err := book.finish(partial)
	if err != nil {
		t.Fatal(err)
	}
	tools := map[string]nativeTimingFile{"compiler": {Bytes: 1, SHA256: strings.Repeat("a", 64)}, "go": {Bytes: 2, SHA256: strings.Repeat("b", 64)}, "verifier": {Bytes: 3, SHA256: strings.Repeat("c", 64)}}
	toolDoc := ciShardToolObservation{Schema: "gooo/native-shard-tool-observations/v1", PlanSHA256: plan.PlanSHA256, Run: plan.Run, Index: 0, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: plan.Run.GoVersion, Before: tools, After: tools, Joined: true, CacheCloseSucceeded: true, Scope: "synthetic fixture only"}
	for name, v := range map[string]any{"partial.json": partial, "command-observations.json": commands, "tool-observations.json": toolDoc} {
		if err := writeNewCIShardJSON(filepath.Join(root, name), v); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "compiler.json"), ciShardExecutorIdentity(t, inventory.Lock), 0600); err != nil {
		t.Fatal(err)
	}
	return plan, inventory, root, partial
}

func TestCIShardReferenceRetainsProducerLocationAndOriginalRawBytes(t *testing.T) {
	plan, inventory, root, partial := ciShardReferenceFixture(t)
	index, err := buildCIShardNativeReference(plan, inventory, []ciShardPartial{partial}, map[int]string{0: filepath.Join(root, "partial.json")}, map[int]string{0: root})
	if err != nil {
		t.Fatal(err)
	}
	if index.NativeAcceptance || index.ProducerAuthenticated || !index.RawReceiptsInspected || len(index.Shards) != 1 || len(index.Suites) != 1 {
		t.Fatal("reference scope inflated", index)
	}
	b := index.Suites[0].Batches[0]
	if b.Workspace.Path != partial.Suites[0].Batches[0].Namespace+"/gooo.workspace.json" || b.ProducerManifestPath == filepath.Join(root, filepath.FromSlash(b.Workspace.Path)) || !strings.HasPrefix(b.ProducerManifestPath, "/producer/owned/") {
		t.Fatal("local and producer locations conflated", b)
	}
	if b.Execution.SHA256 != partial.Suites[0].Batches[0].ExecutionSHA256 || b.Replay.SHA256 != partial.Suites[0].Batches[0].ReplaySHA256 || b.Cases.SHA256 != partial.Suites[0].Batches[0].CasesSHA256 {
		t.Fatal("raw references rebound", b)
	}
	if b.GoBinary.SHA256 != strings.Repeat("b", 64) || len(b.Sources) != 1 || b.Sources[0].Name != plan.Workspaces[0].Sources[0].Name {
		t.Fatal("source/tool references lost", b)
	}
}

func TestCIShardReferenceRejectsMissingChangedAndForeignEvidence(t *testing.T) {
	for _, kind := range []string{"missing-command", "incomplete-command", "tool-drift", "cases-drift", "raw-parent-drift", "producer-path-drift", "Go-binary-drift", "foreign-partial-path", "symlink-case"} {
		t.Run(kind, func(t *testing.T) {
			plan, inventory, root, partial := ciShardReferenceFixture(t)
			path := filepath.Join(root, "partial.json")
			dir := filepath.Join(root, filepath.FromSlash(partial.Suites[0].Batches[0].Namespace))
			mutate := func(file string, change func(map[string]any)) {
				t.Helper()
				b, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var d map[string]any
				if err := json.Unmarshal(b, &d); err != nil {
					t.Fatal(err)
				}
				change(d)
				b, err = json.Marshal(d)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "missing-command":
				if err := os.Remove(filepath.Join(root, "command-observations.json")); err != nil {
					t.Fatal(err)
				}
			case "incomplete-command":
				mutate(filepath.Join(root, "command-observations.json"), func(d map[string]any) { d["complete"] = false })
			case "tool-drift":
				mutate(filepath.Join(root, "tool-observations.json"), func(d map[string]any) {
					d["after"].(map[string]any)["compiler"].(map[string]any)["sha256"] = strings.Repeat("e", 64)
				})
			case "cases-drift":
				if err := os.WriteFile(filepath.Join(dir, "cases.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "raw-parent-drift":
				mutate(filepath.Join(dir, "replay.json"), func(d map[string]any) { d["replayed_from_sha256"] = "sha256:" + strings.Repeat("0", 64) })
			case "producer-path-drift":
				mutate(filepath.Join(dir, "replay.json"), func(d map[string]any) { d["manifest"] = "/producer/foreign/gooo.workspace.json" })
			case "Go-binary-drift":
				mutate(filepath.Join(dir, "execution.json"), func(d map[string]any) {
					d["result"].(map[string]any)["runtime"].(map[string]any)["go_tool_sha256"] = "sha256:" + strings.Repeat("e", 64)
				})
			case "foreign-partial-path":
				path = filepath.Join(root, "foreign.json")
			case "symlink-case":
				file := filepath.Join(dir, "cases.json")
				if err := os.Rename(file, file+".real"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(file+".real", file); err != nil {
					t.Fatal(err)
				}
			}
			// Rebind metadata to the modified raw files for these cases. This
			// exercises the new parent/location/tool checks, not just stale hashes.
			if kind == "raw-parent-drift" || kind == "producer-path-drift" || kind == "Go-binary-drift" {
				original, err := os.ReadFile(filepath.Join(dir, "execution.json"))
				if err != nil {
					t.Fatal(err)
				}
				if kind != "raw-parent-drift" {
					mutate(filepath.Join(dir, "replay.json"), func(d map[string]any) { d["replayed_from_sha256"] = contentDigest(original) })
				}
				replay, err := os.ReadFile(filepath.Join(dir, "replay.json"))
				if err != nil {
					t.Fatal(err)
				}
				b := &partial.Suites[0].Batches[0]
				b.ExecutionSHA256 = contentDigest(original)
				b.ReplaySHA256 = contentDigest(replay)
				b.ReplayedFrom = b.ExecutionSHA256
				raw, err := json.MarshalIndent(partial, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := buildCIShardNativeReference(plan, inventory, []ciShardPartial{partial}, map[int]string{0: path}, map[int]string{0: root}); err == nil {
				t.Fatal("invalid reference accepted", kind)
			}
		})
	}
}

func TestCIShardToolSnapshotAndJoinedGate(t *testing.T) {
	file := filepath.Join(t.TempDir(), "fake-compiler-bytes")
	if err := os.WriteFile(file, []byte("test fixture; never executed"), 0600); err != nil {
		t.Fatal(err)
	}
	tools, err := snapshotCIShardTools(file)
	if err != nil || len(tools) != 3 || tools["compiler"].Bytes != 28 {
		t.Fatal("actual file snapshots missing", tools, err)
	}
	plan, _, root, partial := ciShardReferenceFixture(t)
	raw, err := os.ReadFile(filepath.Join(root, "tool-observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc ciShardToolObservation
	if err := decodeCIShardJSON(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"unjoined", "cache-open", "foreign-run", "missing-go", "bad-unit-platform"} {
		t.Run(kind, func(t *testing.T) {
			v := ciShardTestClone(t, doc)
			switch kind {
			case "unjoined":
				v.Joined = false
			case "cache-open":
				v.CacheCloseSucceeded = false
			case "foreign-run":
				v.Run.RunID = "999"
			case "missing-go":
				delete(v.Before, "go")
			case "bad-unit-platform":
				v.GOOS = "unknown"
			}
			if validateCIShardToolObservation(plan, partial, v) == nil {
				t.Fatal("incomplete tool observation accepted")
			}
		})
	}
}

func TestCIShardReferenceReinspectsExhaustiveSemanticFiles(t *testing.T) {
	// A fabricated exhaustive result exercises file binding only. No generated
	// harness is executed and this fixture supplies no native execution evidence.
	for _, changed := range []string{"none", "native/generated.go", "native/exhaustive_test.go", "native/go.mod", "native-test.txt", "exhaustive.json"} {
		t.Run(changed, func(t *testing.T) {
			root := t.TempDir()
			dir := "native-ci-shard-000/suite-0000/simple-fold-exhaustive"
			if err := os.MkdirAll(filepath.Join(root, dir, "native"), 0700); err != nil {
				t.Fatal(err)
			}
			generated := contentDigest([]byte(simpleFoldFixtureSource))
			replay := mustJSON(t, simpleFoldFixture(simpleFoldFixtureSource))
			harness, err := makeSimpleFoldExhaustiveHarness("Fold", generated)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{"native/generated.go": []byte(simpleFoldFixtureSource), "native/exhaustive_test.go": harness, "native/go.mod": []byte("module gooo-simple-fold-exhaustive\n\ngo 1.27.3\n"), "native-test.txt": []byte("--- PASS: TestSimpleFoldExhaustive (0.00s)\nPASS\n"), "exhaustive.json": mustJSON(t, expectedSimpleFoldExhaustive(generated))}
			for name, raw := range files {
				if err := os.WriteFile(filepath.Join(root, dir, name), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			reader, err := newCIShardEvidenceReader(root)
			if err != nil {
				t.Fatal(err)
			}
			s := suite{Package: "go/unicode", Activity: "SimpleFold"}
			report, err := inspectCIShardNativeExhaustive(reader, dir, s, replay, generated)
			if err != nil {
				t.Fatal(err)
			}
			if err := reader.finish(); err != nil {
				t.Fatal(err)
			}
			if changed != "none" {
				if err := os.WriteFile(filepath.Join(root, dir, changed), []byte("changed after the prior semantic inspection"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			fresh, err := newCIShardEvidenceReader(root)
			if err != nil {
				t.Fatal(err)
			}
			refs, err := referenceCIShardExhaustive(fresh, dir, s, replay, generated, report)
			if changed == "none" {
				if err != nil || len(refs.Files) != 5 {
					t.Fatal("unchanged semantic refs refused", err)
				}
			} else if err == nil {
				t.Fatal("exported changed post-inspection exhaustive bytes", changed)
			}
		})
	}
}
