package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
)

// These are references to files inspected by this invocation. They are not
// attestations, a synthetic legacy timing stream, or permission to execute.
type ciShardReferenceFile struct {
	Path         string `json:"path"`
	Bytes        int64  `json:"bytes"`
	SHA256       string `json:"sha256"`
	Encoding     string `json:"encoding,omitempty"`
	StoredBytes  int64  `json:"stored_bytes,omitempty"`
	StoredSHA256 string `json:"stored_sha256,omitempty"`
}

type ciShardToolObservation struct {
	Schema              string                      `json:"schema"`
	PlanSHA256          string                      `json:"plan_sha256"`
	Run                 ciShardRun                  `json:"run"`
	Index               int                         `json:"index"`
	GOOS                string                      `json:"goos"`
	GOARCH              string                      `json:"goarch"`
	GoVersion           string                      `json:"go_version"`
	Before              map[string]nativeTimingFile `json:"before"`
	After               map[string]nativeTimingFile `json:"after"`
	Joined              bool                        `json:"joined"`
	CacheCloseSucceeded bool                        `json:"cache_close_succeeded"`
	Scope               string                      `json:"scope"`
}

func snapshotCIShardTools(compiler string) (map[string]nativeTimingFile, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	files := map[string]nativeTimingFile{}
	for name, path := range map[string]string{"compiler": compiler, "go": filepath.Join(runtime.GOROOT(), "bin", "go"), "verifier": self} {
		identity, err := nativeTimingHashFile(path)
		if err != nil {
			return nil, err
		}
		files[name] = identity
	}
	return files, nil
}

// Only the production wrapper observes real commands. Injected executor tests
// do not invent wait accounting for fake callbacks.
func executeObservedCIShardNative(ctx context.Context, compiler, out string, plan ciShardPlan, shard int, inventory ciShardNativeInventory) (ciShardPartial, error) {
	book, err := newCIShardCommandObservations(plan, shard)
	if err != nil {
		return ciShardPartial{}, err
	}
	before, err := snapshotCIShardTools(compiler)
	if err != nil {
		return ciShardPartial{}, err
	}
	partial, executionErr := executeCIShardNativeWith(ctx, compiler, out, plan, shard, inventory, ciShardNativeRuntime{
		identify: execute, newCache: newNativeBuildCache,
		closeCache: func(cache *nativeBuildCache) error { return cache.close() },
		verify: func(ctx context.Context, original int, verifier nativeSuiteVerifier, s suite, workspace suiteWorkspace) (nativeSuiteResult, error) {
			verifier.commandObserver = book.observer(original)
			return verifier.verify(ctx, s, workspace)
		},
		inspect: inspectCIShardSuiteEvidence,
	})
	if partial.Schema == "" {
		return partial, executionErr
	}
	// The executor has joined callbacks and attempted cache close before return.
	commands, accountingErr := book.finish(partial)
	after, snapshotErr := snapshotCIShardTools(compiler)
	tools := ciShardToolObservation{
		Schema: "gooo/native-shard-tool-observations/v1", PlanSHA256: plan.PlanSHA256, Run: plan.Run, Index: shard,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version(), Before: before, After: after,
		Joined: partial.Joined, CacheCloseSucceeded: executionErr == nil && partial.State == "SUCCEEDED",
		Scope: "Actual verifier-process tool bytes before and after joined native work. Go receipt tool hashes are checked separately. No child environment, simultaneous memory limit or authenticated job is inferred.",
	}
	if snapshotErr == nil && !reflect.DeepEqual(before, after) {
		snapshotErr = fmt.Errorf("native shard tools changed during execution")
	}
	commandWriteErr := writeNewCIShardJSON(filepath.Join(out, "command-observations.json"), commands)
	toolWriteErr := writeNewCIShardJSON(filepath.Join(out, "tool-observations.json"), tools)
	err = errors.Join(executionErr, accountingErr, snapshotErr, commandWriteErr, toolWriteErr)
	if err != nil {
		if partial.State == "SUCCEEDED" {
			partial.State = "INCOMPLETE"
		}
		// Keep the original full diagnostic as well as any observation failure.
		if diagnosticErr := os.WriteFile(filepath.Join(out, "observation-error.txt"), []byte(err.Error()+"\n"), 0600); diagnosticErr != nil {
			err = errors.Join(err, diagnosticErr)
		}
	}
	return partial, err
}

func validateCIShardToolObservation(plan ciShardPlan, partial ciShardPartial, tools ciShardToolObservation) error {
	if tools.Schema != "gooo/native-shard-tool-observations/v1" || tools.PlanSHA256 != plan.PlanSHA256 || tools.Run != plan.Run || tools.Index != partial.Index ||
		!partial.Joined || partial.State != "SUCCEEDED" || !tools.Joined || !tools.CacheCloseSucceeded || tools.GoVersion != plan.Run.GoVersion ||
		(tools.GOOS != "linux" && tools.GOOS != "darwin") || (tools.GOARCH != "amd64" && tools.GOARCH != "arm64") || tools.Scope == "" || !reflect.DeepEqual(tools.Before, tools.After) || len(tools.Before) != 3 {
		return fmt.Errorf("native shard tool observations are incomplete or foreign")
	}
	for _, name := range []string{"compiler", "go", "verifier"} {
		value, ok := tools.Before[name]
		if !ok || value.Bytes <= 0 || !validDigest("sha256:"+value.SHA256) {
			return fmt.Errorf("native shard tool identity differs: %s", name)
		}
	}
	return nil
}

type ciShardReferenceShard struct {
	Index               int                  `json:"index"`
	Namespace           string               `json:"namespace"`
	EvidenceRoot        string               `json:"evidence_root"`
	Partial             ciShardReferenceFile `json:"partial"`
	CompilerIdentity    ciShardReferenceFile `json:"compiler_identity"`
	CommandObservations ciShardReferenceFile `json:"command_observations"`
	ToolObservations    ciShardReferenceFile `json:"tool_observations"`
}

type ciShardReferenceSource struct {
	Name string               `json:"name"`
	File ciShardReferenceFile `json:"file"`
}

type ciShardReferenceBatch struct {
	Batch                ciShardBatch             `json:"batch"`
	Workspace            ciShardReferenceFile     `json:"workspace"`
	Cases                ciShardReferenceFile     `json:"cases"`
	Execution            ciShardReferenceFile     `json:"execution"`
	Replay               ciShardReferenceFile     `json:"replay"`
	Sources              []ciShardReferenceSource `json:"sources"`
	ProducerManifestPath string                   `json:"producer_manifest_path"`
	GoVersion            string                   `json:"go_version"`
	GoBinary             nativeTimingFile         `json:"go_binary"`
}

type ciShardReferenceSuite struct {
	OriginalIndex int                          `json:"original_index"`
	ShardIndex    int                          `json:"shard_index"`
	Package       string                       `json:"package"`
	Activity      string                       `json:"activity"`
	Batches       []ciShardReferenceBatch      `json:"batches"`
	Exhaustive    []ciShardReferenceExhaustive `json:"exhaustive"`
}

type ciShardReferenceExhaustive struct {
	Kind         string                 `json:"kind"`
	ReportSHA256 string                 `json:"report_sha256"`
	Files        []ciShardReferenceFile `json:"files"`
}

type ciShardNativeReference struct {
	Schema                string                  `json:"schema"`
	State                 string                  `json:"state"`
	NativeAcceptance      bool                    `json:"native_acceptance"`
	RawReceiptsInspected  bool                    `json:"raw_receipts_inspected"`
	ProducerAuthenticated bool                    `json:"producer_authenticated"`
	Plan                  ciShardPlan             `json:"plan"`
	Shards                []ciShardReferenceShard `json:"shards"`
	Suites                []ciShardReferenceSuite `json:"suites"`
	Scope                 string                  `json:"scope"`
}

func ciShardReferenceRead(reader *ciShardEvidenceReader, path string, want string) (ciShardReferenceFile, []byte, error) {
	if base := filepath.Base(path); base == "execution.json" || base == "replay.json" {
		identity, raw, err := reader.receipt(path)
		if err == nil && want != "" && identity.SHA256 != want {
			err = fmt.Errorf("native shard reference original JSON differs: %s", path)
		}
		return identity, raw, err
	}
	raw, err := reader.read(path, 64<<20)
	if err != nil {
		return ciShardReferenceFile{}, nil, err
	}
	identity := ciShardReferenceFile{Path: path, Bytes: int64(len(raw)), SHA256: contentDigest(raw)}
	if want != "" && identity.SHA256 != want {
		return identity, raw, fmt.Errorf("native shard reference bytes differ: %s", path)
	}
	return identity, raw, nil
}

type ciShardReceiptLocation struct {
	Manifest     string `json:"manifest"`
	ReplayedFrom string `json:"replayed_from_sha256"`
	Result       struct {
		Runtime struct {
			GoVersion string `json:"go_version"`
			GoSHA256  string `json:"go_tool_sha256"`
		} `json:"runtime"`
	} `json:"result"`
}

func referenceCIShardExhaustive(reader *ciShardEvidenceReader, directory string, s suite, replay []byte, generated string, report ciShardExhaustive) (ciShardReferenceExhaustive, error) {
	zero := ciShardReferenceExhaustive{}
	// Recheck semantic bytes with this same reader before exporting their hashes.
	checked, err := inspectCIShardNativeExhaustive(reader, directory, s, replay, generated)
	if err != nil || checked.Kind != report.Kind || checked.SHA256 != report.SHA256 {
		return zero, fmt.Errorf("exhaustive reference semantic reinspection differs: %v", err)
	}
	result := ciShardReferenceExhaustive{Kind: report.Kind, ReportSHA256: report.SHA256, Files: []ciShardReferenceFile{}}
	for _, name := range []string{"native/generated.go", "native/exhaustive_test.go", "native/go.mod", "native-test.txt", "exhaustive.json"} {
		file, raw, err := ciShardReferenceRead(reader, directory+"/"+name, "")
		if err != nil {
			return zero, err
		}
		if name == "exhaustive.json" {
			digest, err := ciShardExhaustiveDigest(raw)
			if err != nil || digest != report.SHA256 {
				return zero, fmt.Errorf("exhaustive reference report differs")
			}
		}
		result.Files = append(result.Files, file)
	}
	return result, nil
}

// The caller must first inspect every native suite against current Go values.
// This additional pass emits immutable byte/path references and checks their
// post-read stability; it never upgrades unsigned results into CI acceptance.
func buildCIShardNativeReference(plan ciShardPlan, inventory ciShardNativeInventory, partials []ciShardPartial, partialPaths, evidence map[int]string) (ciShardNativeReference, error) {
	result := ciShardNativeReference{
		Schema: "gooo/native-shard-reference-index/v2", State: "RAW_REFERENCES_CHECKED_PRODUCER_UNAUTHENTICATED", RawReceiptsInspected: true, Plan: plan,
		Shards: []ciShardReferenceShard{}, Suites: []ciShardReferenceSuite{},
		Scope: "References from the current raw native inspection, original case boundaries and actual waited-command/tool observations. Producer manifest strings are data, separate from local artifact paths. No authenticated job, legacy footer, child environment or CI acceptance is asserted.",
	}
	if _, err := reduceCIShardPartials(plan, partials); err != nil {
		return result, err
	}
	if len(partialPaths) != len(plan.Shards) || len(evidence) != len(plan.Shards) {
		return result, fmt.Errorf("complete shard reference path set required")
	}
	ordered := append([]ciShardPartial(nil), partials...)
	readers := []*ciShardEvidenceReader{}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Index < ordered[j].Index })
	for _, partial := range ordered {
		root := evidence[partial.Index]
		if filepath.Clean(partialPaths[partial.Index]) != filepath.Join(root, "partial.json") {
			return result, fmt.Errorf("reference export requires the original paired partial.json")
		}
		reader, err := newCIShardEvidenceReader(root)
		if err != nil {
			return result, err
		}
		readers = append(readers, reader)
		ref := ciShardReferenceShard{Index: partial.Index, Namespace: partial.Namespace, EvidenceRoot: root}
		var raw []byte
		ref.Partial, raw, err = ciShardReferenceRead(reader, "partial.json", "")
		if err != nil {
			return result, err
		}
		var saved ciShardPartial
		if err := decodeCIShardJSON(raw, &saved); err != nil {
			return result, err
		}
		if !reflect.DeepEqual(saved, partial) {
			return result, fmt.Errorf("paired partial changed before reference export")
		}
		ref.CompilerIdentity, raw, err = ciShardReferenceRead(reader, "compiler.json", "")
		if err != nil {
			return result, err
		}
		if err := validateCIShardCompilerIdentity(raw, inventory.Lock); err != nil {
			return result, err
		}
		ref.CommandObservations, raw, err = ciShardReferenceRead(reader, "command-observations.json", "")
		if err != nil {
			return result, err
		}
		var commands ciShardCommandObservationDocument
		if err := decodeCIShardJSON(raw, &commands); err != nil {
			return result, err
		}
		if err := validateCIShardCommandObservationDocument(plan, partial, commands); err != nil {
			return result, err
		}
		ref.ToolObservations, raw, err = ciShardReferenceRead(reader, "tool-observations.json", "")
		if err != nil {
			return result, err
		}
		var tools ciShardToolObservation
		if err := decodeCIShardJSON(raw, &tools); err != nil {
			return result, err
		}
		if err := validateCIShardToolObservation(plan, partial, tools); err != nil {
			return result, err
		}
		if commands.GOOS != tools.GOOS {
			return result, fmt.Errorf("command/tool platform observations differ")
		}
		for _, suite := range partial.Suites {
			r := ciShardReferenceSuite{OriginalIndex: suite.OriginalIndex, ShardIndex: partial.Index, Package: suite.Package, Activity: suite.Activity, Batches: []ciShardReferenceBatch{}, Exhaustive: []ciShardReferenceExhaustive{}}
			for _, batch := range suite.Batches {
				b := ciShardReferenceBatch{Batch: batch, Sources: []ciShardReferenceSource{}, GoBinary: tools.Before["go"]}
				b.Workspace, _, err = ciShardReferenceRead(reader, batch.Namespace+"/gooo.workspace.json", batch.ManifestSHA256)
				if err != nil {
					return result, err
				}
				b.Cases, _, err = ciShardReferenceRead(reader, batch.Namespace+"/cases.json", batch.CasesSHA256)
				if err != nil {
					return result, err
				}
				var original, replay []byte
				b.Execution, original, err = ciShardReferenceRead(reader, batch.Namespace+"/execution.json", batch.ExecutionSHA256)
				if err != nil {
					return result, err
				}
				b.Replay, replay, err = ciShardReferenceRead(reader, batch.Namespace+"/replay.json", batch.ReplaySHA256)
				if err != nil {
					return result, err
				}
				var location, replayLocation ciShardReceiptLocation
				if err := json.Unmarshal(original, &location); err != nil {
					return result, err
				}
				if err := json.Unmarshal(replay, &replayLocation); err != nil {
					return result, err
				}
				if location.ReplayedFrom != "" || replayLocation.ReplayedFrom != batch.ExecutionSHA256 {
					return result, fmt.Errorf("reference raw saved replay parent differs")
				}
				replayLocation.ReplayedFrom = ""
				b.GoVersion = "go version " + tools.GoVersion + " " + tools.GOOS + "/" + tools.GOARCH
				if location != replayLocation || !filepath.IsAbs(location.Manifest) || location.Manifest == "" || location.Result.Runtime.GoVersion != b.GoVersion || location.Result.Runtime.GoSHA256 != "sha256:"+b.GoBinary.SHA256 {
					return result, fmt.Errorf("reference producer path or actual Go tool binding differs")
				}
				b.ProducerManifestPath = location.Manifest
				for _, source := range plan.Workspaces[suite.OriginalIndex].Sources {
					file, _, err := ciShardReferenceRead(reader, batch.Namespace+"/"+source.Name, source.RawSHA256)
					if err != nil {
						return result, err
					}
					b.Sources = append(b.Sources, ciShardReferenceSource{Name: source.Name, File: file})
				}
				r.Batches = append(r.Batches, b)
			}
			for _, report := range suite.Exhaustive {
				directory := filepath.ToSlash(filepath.Join(ciShardNamespace(partial.Index), fmt.Sprintf("suite-%04d", suite.OriginalIndex), ciShardNativeExhaustiveDirectory(inventory.Suites[suite.OriginalIndex])))
				if len(r.Batches) == 0 {
					return result, fmt.Errorf("exhaustive reference requires its original first batch")
				}
				_, replay, err := reader.receipt(r.Batches[0].Replay.Path)
				if err != nil {
					return result, err
				}
				exhaustive, err := referenceCIShardExhaustive(reader, directory, inventory.Suites[suite.OriginalIndex], replay, suite.Batches[0].GeneratedSHA256, report)
				if err != nil {
					return result, err
				}
				r.Exhaustive = append(r.Exhaustive, exhaustive)
			}
			result.Suites = append(result.Suites, r)
		}
		if err := reader.finish(); err != nil {
			return result, err
		}
		result.Shards = append(result.Shards, ref)
	}
	for _, reader := range readers {
		if err := reader.finish(); err != nil {
			return result, err
		}
	}
	sort.Slice(result.Suites, func(i, j int) bool { return result.Suites[i].OriginalIndex < result.Suites[j].OriginalIndex })
	return result, nil
}
