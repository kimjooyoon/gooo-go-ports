//go:build linux || darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeSessionStorageFramesAndReplayKeepExactRawBytes(t *testing.T) {
	c, batch, workspace, cases := nativeSessionFixtureWithStorage(t, "raw", nativeSessionEvidenceGzip)
	execution, replay := filepath.Join(batch, "execution.json"), filepath.Join(batch, "replay.json")
	first, err := c.Execute(context.Background(), c.compiler, execution, "package", "execute", "--json", "--cases", cases, workspace)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(execution)
	if err != nil || !bytes.Equal(first, before) {
		t.Fatal("execution raw bytes changed", err)
	}
	second, err := c.Execute(context.Background(), c.compiler, replay, "package", "replay", "--json", "--receipt", execution, "--cases", cases, workspace)
	if err != nil {
		t.Fatal(err)
	}
	requests := c.Summary()["requests"].([]nativeSessionRequestObservation)
	if requests[0].Receipt.Encoding != "" || requests[1].Receipt.Encoding != "" {
		t.Fatal("receipts compressed before replay/check seam")
	}
	for index, ref := range c.Summary()["frames"].([]nativeSessionFileReference) {
		if ref.Encoding != "gzip" {
			t.Fatal("gzip frame profile not applied", ref)
		}
		raw, err := readNativeSessionStoredEvidence(ref)
		if err != nil {
			t.Fatal(err)
		}
		var frame struct {
			Receipt json.RawMessage `json:"receipt"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			t.Fatal(err)
		}
		want := [][]byte{first, second}[index]
		if !bytes.Equal(append(append([]byte(nil), frame.Receipt...), '\n'), want) {
			t.Fatal("frame inner receipt was reserialized")
		}
	}
	if err := c.compactBatch(batch); err != nil {
		t.Fatal(err)
	}
	requests = c.Summary()["requests"].([]nativeSessionRequestObservation)
	for index, item := range requests {
		if item.Receipt.Encoding != "gzip" {
			t.Fatal("compacted receipt ref was not updated")
		}
		raw, err := readNativeSessionStoredEvidence(*item.Receipt)
		if err != nil || !bytes.Equal(raw, [][]byte{first, second}[index]) {
			t.Fatal("compaction changed raw identity", err)
		}
		logical, err := nativeSessionLogicalPath(*item.Receipt)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(logical); !os.IsNotExist(err) {
			t.Fatal("original retained after successful verified compaction", err)
		}
	}
	if !strings.Contains(strings.Join(requests[1].OriginalArgv, "\n"), "\n"+execution+"\n") || strings.Contains(strings.Join(requests[1].OriginalArgv, "\n"), execution+".gz") {
		t.Fatal("recorded replay argv changed to storage path")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	nativeSessionAssertJoined(t, c)
	for _, ref := range c.Summary()["frames"].([]nativeSessionFileReference) {
		if ref.Encoding != "gzip" {
			t.Fatal("close frame storage differs", ref)
		}
		if _, err := readNativeSessionStoredEvidence(ref); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeSessionStorageFrameFailureRetainsPlainOriginal(t *testing.T) {
	c, batch, workspace, cases := nativeSessionFixtureWithStorage(t, "raw", nativeSessionEvidenceGzip)
	blocked := filepath.Join(c.evidence, "frames", "000001.jsonl.gz")
	if err := os.WriteFile(blocked, []byte("retained original destination"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(batch, "execution.json")
	if _, err := c.Execute(context.Background(), c.compiler, path, "package", "execute", "--json", "--cases", cases, workspace); err == nil {
		t.Fatal("failed frame persistence became a successful request")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("storage refusal gained a saved receipt")
	}
	refs := c.Summary()["frames"].([]nativeSessionFileReference)
	if len(refs) != 1 || refs[0].Encoding != "" || strings.HasSuffix(refs[0].Path, ".gz") {
		t.Fatal("plain original frame was not retained", refs)
	}
	raw, err := readNativeSessionStoredEvidence(refs[0])
	if err != nil || !bytes.Contains(raw, []byte(`"decision" : "PROGRESS"`)) {
		t.Fatal("original refused-frame bytes lost", err)
	}
	if raw, err := os.ReadFile(blocked); err != nil || string(raw) != "retained original destination" {
		t.Fatal("existing gzip destination overwritten", err)
	}
	if err := c.Close(); err == nil {
		t.Fatal("failed storage gained clean Close success")
	}
	nativeSessionAssertJoined(t, c)
}

// This owner has no subprocess. The unchanged production verifier inspects
// source-bound synthetic receipts before invoking the real compaction hook.
func TestNativeSessionStorageProductionHookAfterOwnParentChecks(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprintf("parent_failure=%t", failure), func(t *testing.T) {
			raw, s, identity := nativeSessionInspectFixture(t, false)
			s = repeatNativeCase(s, 129)
			runRoot := t.TempDir()
			ownerRoot := suiteEvidenceDirectory(runRoot, s)
			if err := os.Mkdir(ownerRoot, 0700); err != nil {
				t.Fatal(err)
			}
			ctx := workerTestContext(t)
			c := &nativeSessionClient{root: ownerRoot, storage: nativeSessionEvidenceGzip, ctx: ctx, gate: make(chan struct{}, 1), done: make(chan struct{})}
			fake := &fakeNativeCommand{templates: map[string][]byte{ownerRoot: raw}, calls: map[string][]string{}}
			if failure {
				fake.mutate = func(action string, doc map[string]any) {
					if action == "replay" {
						doc["replayed_from_sha256"] = "sha256:" + strings.Repeat("0", 64)
					}
				}
			}
			v := newNativeSuiteVerifier(ctx, "synthetic-unused", runRoot, upstream{CompilerRevision: uuidCompactCompilerRevision, GoVersion: nativeSessionProfileGoVersion}, nil)
			inspections, compactions := 0, 0
			v.compilerRevisionOverride = nativeSessionCompilerRevision
			v.inspectExecution = func(raw []byte, s suite, replay bool, id expectedIdentity, r *nativeReportedRuntime) (checked, error) {
				result, err := inspectSessionWithRuntime(raw, s, replay, id, r)
				if err == nil {
					inspections++
				}
				return result, err
			}
			v.execute = func(ctx context.Context, compiler, path string, args ...string) ([]byte, error) {
				if args[1] == "replay" {
					for i, arg := range args {
						if arg == "--receipt" {
							if _, err := os.ReadFile(args[i+1]); err != nil {
								return nil, fmt.Errorf("own plain replay parent unavailable: %w", err)
							}
						}
					}
				}
				raw, err := fake.execute(ctx, compiler, path, args...)
				if err != nil {
					return nil, err
				}
				ref := nativeSessionReference(path, raw)
				c.observations = append(c.observations, nativeSessionRequestObservation{Operation: args[1], OriginalArgv: append([]string{compiler}, args...), Receipt: &ref})
				return raw, nil
			}
			v.compactBatch = func(dir string) error {
				if inspections != 2*(compactions+1) {
					return fmt.Errorf("compaction preceded both inspections")
				}
				compactions++
				return c.compactBatch(dir)
			}
			result, err := v.verify(ctx, s, fakeNativeWorkspace(t, s, identity))
			if failure {
				if err == nil || compactions != 0 {
					t.Fatal("invalid parent reached compaction", err, compactions)
				}
				for _, item := range c.observations {
					if item.Receipt.Encoding != "" {
						t.Fatal("failed batch changed references")
					}
					if _, err := os.ReadFile(item.Receipt.Path); err != nil {
						t.Fatal("failed batch lost original receipt", err)
					}
				}
				return
			}
			if err != nil || result.Observation.Passed != 129 || result.Observation.Batches != 2 || compactions != 2 {
				t.Fatal("original verifier behavior changed", result, err, compactions)
			}
			for batch := 0; batch < 2; batch++ {
				execRef, replayRef := *c.observations[batch*2].Receipt, *c.observations[batch*2+1].Receipt
				execution, err := readNativeSessionStoredEvidence(execRef)
				if err != nil {
					t.Fatal(err)
				}
				replayed, err := readNativeSessionStoredEvidence(replayRef)
				if err != nil {
					t.Fatal(err)
				}
				var doc map[string]any
				if err := json.Unmarshal(replayed, &doc); err != nil {
					t.Fatal(err)
				}
				logical, err := nativeSessionLogicalPath(execRef)
				if err != nil {
					t.Fatal(err)
				}
				if execRef.Encoding != "gzip" || replayRef.Encoding != "gzip" || doc["replayed_from_sha256"] != contentDigest(execution) || !strings.Contains(strings.Join(c.observations[batch*2+1].OriginalArgv, "\n"), "\n"+logical+"\n") {
					t.Fatal("logical parent identity changed after compaction")
				}
			}
		})
	}
}

func TestNativeSessionStoragePairFailurePreservesBothOriginals(t *testing.T) {
	for _, mode := range []string{"existing-destination", "second-ref-mismatch"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			batch := filepath.Join(root, "000")
			if err := os.Mkdir(batch, 0700); err != nil {
				t.Fatal(err)
			}
			c := &nativeSessionClient{root: root, storage: nativeSessionEvidenceGzip, ctx: context.Background(), gate: make(chan struct{}, 1), done: make(chan struct{})}
			originals := [][]byte{[]byte(" {\"original\":1e0} \n"), []byte(" {\"replay\":true} \n")}
			for i, name := range []string{"execution.json", "replay.json"} {
				path := filepath.Join(batch, name)
				if err := os.WriteFile(path, originals[i], 0600); err != nil {
					t.Fatal(err)
				}
				ref := nativeSessionReference(path, originals[i])
				c.observations = append(c.observations, nativeSessionRequestObservation{Operation: []string{"execute", "replay"}[i], Receipt: &ref})
			}
			if mode == "existing-destination" {
				if err := os.WriteFile(filepath.Join(batch, "replay.json.gz"), []byte("retain"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				c.observations[1].Receipt.SHA256 = strings.Repeat("0", 64)
			}
			if err := c.compactBatch(batch); err == nil {
				t.Fatal("failed pair accepted")
			}
			for i, name := range []string{"execution.json", "replay.json"} {
				raw, err := os.ReadFile(filepath.Join(batch, name))
				if err != nil || !bytes.Equal(raw, originals[i]) || c.observations[i].Receipt.Encoding != "" {
					t.Fatal("original bytes/ref lost on pair staging failure", err)
				}
			}
		})
	}
}

func TestNativeSessionStorageRunnerSelectionIsProcessFreeAndImmutable(t *testing.T) {
	r := nativeSessionCLITestRunner(t)
	if err := r.EnableCompressedEvidence(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(r.root)
	if err != nil || len(entries) != 0 || r.executionProfile != nativeSessionCLIGzipExecutionProfile {
		t.Fatal("selection started evidence or retained plain profile", err)
	}
	if err := r.WriteSummary(filepath.Join(r.root, "session-summary.json")); err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := readJSON(filepath.Join(r.root, "session-summary.json"), &summary); err != nil {
		t.Fatal(err)
	}
	if summary["execution_profile"] != nativeSessionCLIGzipExecutionProfile || summary["per_request_wait_resources"].(map[string]any)["status"] != "NOT_OBSERVED" {
		t.Fatal("storage changed execution/resource identity", summary)
	}
	if err := r.EnableCompressedEvidence(); err == nil {
		t.Fatal("sealed runner storage changed")
	}
	started := nativeSessionCLITestRunner(t)
	started.records["synthetic-owner"] = nativeSessionCLISuiteRecord{Attempted: true}
	if err := started.EnableCompressedEvidence(); err == nil || started.executionProfile != nativeSessionCLIExecutionProfile {
		t.Fatal("already-started runner changed storage")
	}
}
