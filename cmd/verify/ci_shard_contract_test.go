package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func ciShardTestClone[T any](t *testing.T, v T) T {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var result T
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func ciShardTestInputs(t *testing.T, entries []string) (*NativeVerificationContract, ciShardRun, map[string]ciShardSource, []ciShardWorkspace) {
	t.Helper()
	c := &NativeVerificationContract{Profiles: ContractProfiles{Case: "uuid-and-literal-report-compact-v1", Workspace: "dependency-closure-v1"}}
	sources := map[string]ciShardSource{}
	for _, name := range ciShardReaderSources {
		sources[name] = ciShardSource{"100644", 1, contentDigest([]byte(name))}
	}
	workspaces := []ciShardWorkspace{}
	for i, entry := range entries {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			t.Fatal(entry)
		}
		name := fmt.Sprintf("ports/synthetic/%d.gooo", i)
		cases, batches := int64(i+3), int64(1+i%3)
		c.Suites = append(c.Suites, ContractSuite{Index: int64(i), Package: parts[0], Activity: parts[1], Cases: cases, Batches: batches, SourcePaths: []string{name}})
		c.Sources = append(c.Sources, ContractSource{Path: name, Mode: "100644"})
		c.TotalCases += cases
		c.TotalBatches += batches
		sources[name] = ciShardSource{"100644", int64(i + 1), contentDigest([]byte(name))}
		anchors := []ciShardSourceAnchor{{Name: name, RawSHA256: sources[name].SHA256, Identity: sourceIdentity{parts[0], contentDigest([]byte("image" + name))}}}
		digest, err := canonicalContractDigest(anchors)
		if err != nil {
			t.Fatal(err)
		}
		workspaces = append(workspaces, ciShardWorkspace{OriginalIndex: i, ManifestSHA256: contentDigest([]byte("manifest" + name)), SourceSetSHA256: digest, EntryProducer: "gooo://synthetic/" + parts[1], Sources: anchors})
	}
	run := ciShardRun{Repository: "example/project", RunID: "12345", Attempt: 1, TestedCommit: strings.Repeat("a", 40), TestedTree: strings.Repeat("b", 40), CompilerRevision: strings.Repeat("c", 40), GoVersion: "go1.27.3", Workers: 3}
	return c, run, sources, workspaces
}

func ciShardTestPlan(t *testing.T, count int, entries ...string) ciShardPlan {
	t.Helper()
	if len(entries) == 0 {
		entries = []string{"go/synthetic:A", "go/synthetic:B", "go/synthetic:C", "go/synthetic:D", "go/synthetic:E"}
	}
	c, run, sources, workspaces := ciShardTestInputs(t, entries)
	p, err := newCIShardPlan(c, contentDigest([]byte("independent contract")), run, count, sources, workspaces)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func ciShardTestPartials(t *testing.T, p ciShardPlan) []ciShardPartial {
	t.Helper()
	partials := []ciShardPartial{}
	for _, assignment := range p.Shards {
		part := ciShardPartial{Schema: ciShardPartialSchema, PlanSHA256: p.PlanSHA256, Run: p.Run, Index: assignment.Index, Namespace: assignment.Namespace, Sources: ciShardTestClone(t, p.Sources), Joined: true, State: "SUCCEEDED", Suites: []ciShardSuiteRecord{}}
		for _, index := range assignment.OriginalIndices {
			s, w := p.Suites[index], p.Workspaces[index]
			record := ciShardSuiteRecord{OriginalIndex: index, Package: s.Package, Activity: s.Activity, State: "VERIFIED", Batches: []ciShardBatch{}, Exhaustive: []ciShardExhaustive{}}
			start := int64(0)
			for batch := 0; batch < int(s.Batches); batch++ {
				end := start + (s.Cases-start)/(s.Batches-int64(batch))
				execution := contentDigest([]byte(fmt.Sprintf("execution-%d-%d", index, batch)))
				record.Batches = append(record.Batches, ciShardBatch{Index: batch, Start: start, End: end, Namespace: ciShardBatchNamespace(part.Index, index, batch), CasesSHA256: contentDigest([]byte(fmt.Sprintf("cases-%d-%d", index, batch))), ManifestSHA256: w.ManifestSHA256, SourceSetSHA256: w.SourceSetSHA256, EntryProducer: w.EntryProducer, GeneratedSHA256: contentDigest([]byte(fmt.Sprintf("generated-%d", index))), ExecutionSHA256: execution, ReplaySHA256: contentDigest([]byte(fmt.Sprintf("replay-%d-%d", index, batch))), ReplayedFrom: execution})
				start = end
			}
			kind := ciShardExhaustiveKind(s)
			if kind != "" {
				var report any
				generated := record.Batches[0].GeneratedSHA256
				switch {
				case kind == "simple-fold":
					report = expectedSimpleFoldExhaustive(generated)
				case strings.HasPrefix(kind, "ycbcr/"):
					v, err := expectedYCbCrExhaustive(generated, s.Activity)
					if err != nil {
						t.Fatal(err)
					}
					report = v
				default:
					v, err := expectedCaseMappingExhaustive(generated, s.Activity)
					if err != nil {
						t.Fatal(err)
					}
					report = v
				}
				raw, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				digest, err := ciShardExhaustiveDigest(raw)
				if err != nil {
					t.Fatal(err)
				}
				record.Exhaustive = append(record.Exhaustive, ciShardExhaustive{Kind: kind, Report: raw, SHA256: digest})
			}
			part.Suites = append(part.Suites, record)
		}
		partials = append(partials, part)
	}
	return partials
}

func TestCIShardPlanDynamicPartitionAndCallerIsolation(t *testing.T) {
	for _, count := range []int{1, 2, 3, 5} {
		p := ciShardTestPlan(t, count)
		seen := map[int]bool{}
		var cases, batches int64
		for _, a := range p.Shards {
			if len(a.OriginalIndices) == 0 {
				t.Fatal("empty shard")
			}
			for _, i := range a.OriginalIndices {
				if seen[i] {
					t.Fatal("duplicate suite")
				}
				seen[i] = true
				cases += p.Suites[i].Cases
				batches += p.Suites[i].Batches
			}
		}
		if len(seen) != 5 || cases != p.Cases || batches != p.Batches {
			t.Fatal("coverage differs")
		}
		if err := validateCIShardPlanShape(p); err != nil {
			t.Fatal(err)
		}
	}
	c, run, sources, workspaces := ciShardTestInputs(t, []string{"go/synthetic:A", "go/synthetic:B"})
	p, err := newCIShardPlan(c, contentDigest([]byte("ledger")), run, 2, sources, workspaces)
	if err != nil {
		t.Fatal(err)
	}
	before := ciShardTestClone(t, p)
	for name := range sources {
		delete(sources, name)
	}
	workspaces[0].Sources[0].RawSHA256 = "changed"
	c.Suites[0].SourcePaths[0] = "changed"
	if !reflect.DeepEqual(p, before) {
		t.Fatal("hashed plan aliases caller state")
	}
}

func TestCIShardPlanRejectsMalformedOrForeignInventory(t *testing.T) {
	for _, mutate := range []func(*NativeVerificationContract, map[string]ciShardSource, []ciShardWorkspace){
		func(c *NativeVerificationContract, _ map[string]ciShardSource, _ []ciShardWorkspace) {
			c.Suites[0].Index = 9
		},
		func(c *NativeVerificationContract, _ map[string]ciShardSource, _ []ciShardWorkspace) { c.TotalCases++ },
		func(_ *NativeVerificationContract, s map[string]ciShardSource, _ []ciShardWorkspace) {
			s["foreign"] = ciShardSource{"100644", 1, contentDigest([]byte("x"))}
		},
		func(_ *NativeVerificationContract, _ map[string]ciShardSource, w []ciShardWorkspace) {
			w[0].Sources[0].RawSHA256 = contentDigest([]byte("foreign"))
		},
	} {
		c, run, sources, workspaces := ciShardTestInputs(t, []string{"go/synthetic:A", "go/synthetic:B"})
		mutate(c, sources, workspaces)
		if _, err := newCIShardPlan(c, contentDigest([]byte("ledger")), run, 2, sources, workspaces); err == nil {
			t.Fatal("malformed plan accepted")
		}
	}
	p := ciShardTestPlan(t, 3)
	altered := ciShardTestClone(t, p)
	altered.Shards[0].OriginalIndices[0] = 999
	altered.PlanSHA256, _ = ciShardPlanDigest(altered)
	if _, err := reduceCIShardPartials(altered, ciShardTestPartials(t, p)); err == nil {
		t.Fatal("self-hashed malformed plan accepted")
	}
}

func TestCIShardReductionRequiresAllBindingsAndNeverClaimsNativeAcceptance(t *testing.T) {
	p := ciShardTestPlan(t, 3)
	parts := ciShardTestPartials(t, p)
	parts[0], parts[2] = parts[2], parts[0]
	r, err := reduceCIShardPartials(p, parts)
	if err != nil || r.NativeAcceptance || r.RawReceiptsInspected || r.State != "CONTRACT_COMPLETE_NATIVE_UNCHECKED" || len(r.Suites) != 5 || r.Cases != p.Cases || r.Batches != p.Batches {
		t.Fatal("metadata reduction or acceptance scope differs", r, err)
	}
	for i, s := range r.Suites {
		if s.OriginalIndex != i {
			t.Fatal("completion order replaced original catalog order")
		}
	}
	mutations := map[string]func([]ciShardPartial){
		"duplicate shard": func(ps []ciShardPartial) { ps[1] = ps[0] },
		"foreign shard":   func(ps []ciShardPartial) { ps[0].Index = 999 },
		"namespace":       func(ps []ciShardPartial) { ps[0].Namespace = "../escape" },
		"producer run":    func(ps []ciShardPartial) { ps[0].Run.RunID = "9" },
		"producer tree":   func(ps []ciShardPartial) { ps[0].Run.TestedTree = strings.Repeat("d", 40) },
		"source": func(ps []ciShardPartial) {
			for n, s := range ps[0].Sources {
				s.Bytes++
				ps[0].Sources[n] = s
				break
			}
		},
		"not joined":       func(ps []ciShardPartial) { ps[0].Joined = false },
		"incomplete state": func(ps []ciShardPartial) { ps[0].State = "INCOMPLETE" },
		"duplicate suite":  func(ps []ciShardPartial) { ps[0].Suites[0].OriginalIndex = ps[1].Suites[0].OriginalIndex },
		"missing batches":  func(ps []ciShardPartial) { ps[0].Suites[0].Batches = nil },
		"batch gap":        func(ps []ciShardPartial) { ps[0].Suites[0].Batches[0].Start++ },
		"batch namespace":  func(ps []ciShardPartial) { ps[0].Suites[0].Batches[0].Namespace = "foreign" },
		"workspace": func(ps []ciShardPartial) {
			ps[0].Suites[0].Batches[0].ManifestSHA256 = contentDigest([]byte("foreign"))
		},
		"source set": func(ps []ciShardPartial) {
			ps[0].Suites[0].Batches[0].SourceSetSHA256 = contentDigest([]byte("foreign"))
		},
		"entry producer": func(ps []ciShardPartial) { ps[0].Suites[0].Batches[0].EntryProducer = "gooo://foreign" },
		"saved parent":   func(ps []ciShardPartial) { ps[0].Suites[0].Batches[0].ReplayedFrom = contentDigest([]byte("foreign")) },
		"case digest":    func(ps []ciShardPartial) { ps[0].Suites[0].Batches[0].CasesSHA256 = "PASS" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			ps := ciShardTestPartials(t, p)
			mutate(ps)
			if _, err := reduceCIShardPartials(p, ps); err == nil {
				t.Fatal("foreign/incomplete evidence accepted")
			}
		})
	}
	if _, err := reduceCIShardPartials(p, parts[:2]); err == nil {
		t.Fatal("missing shard accepted")
	}
}

func TestCIShardFailureWaitsForEveryJoinedShardAndChoosesOriginalMinimum(t *testing.T) {
	p := ciShardTestPlan(t, 3)
	parts := ciShardTestPartials(t, p)
	for i := range parts {
		parts[i].State = "FAILED"
		parts[i].Suites[0].State = "FAILED"
		parts[i].Suites[0].Error = fmt.Sprintf("failure%d", parts[i].Suites[0].OriginalIndex)
	}
	parts[0], parts[2] = parts[2], parts[0]
	_, err := reduceCIShardPartials(p, parts)
	var failure *ciShardFailure
	if !errors.As(err, &failure) || failure.OriginalIndex != 0 {
		t.Fatal("original earliest failure not selected", err)
	}
	parts[0].Joined = false
	if _, err := reduceCIShardPartials(p, parts); err == nil || errors.As(err, &failure) {
		t.Fatal("unjoined shard produced accepted failure diagnosis", err)
	}
}

func TestCIShardAllSevenExhaustiveContractsRemainRequired(t *testing.T) {
	p := ciShardTestPlan(t, 3, "go/unicode:SimpleFold", "go/image/color:RGBToYCbCr", "go/image/color:YCbCrToRGB", "go/unicode:To", "go/unicode:ToUpper", "go/unicode:ToLower", "go/unicode:ToTitle")
	parts := ciShardTestPartials(t, p)
	if _, err := reduceCIShardPartials(p, parts); err != nil {
		t.Fatal(err)
	}
	for shard := range parts {
		for entry := range parts[shard].Suites {
			ps := ciShardTestClone(t, parts)
			ps[shard].Suites[entry].Exhaustive = nil
			if _, err := reduceCIShardPartials(p, ps); err == nil {
				t.Fatal("missing exhaustive report accepted")
			}
			ps = ciShardTestClone(t, parts)
			report := &ps[shard].Suites[entry].Exhaustive[0]
			var v map[string]any
			if err := json.Unmarshal(report.Report, &v); err != nil {
				t.Fatal(err)
			}
			v["mismatches"] = 1
			report.Report, _ = json.Marshal(v)
			report.SHA256, _ = ciShardExhaustiveDigest(report.Report)
			if _, err := reduceCIShardPartials(p, ps); err == nil {
				t.Fatal("self-consistent bad exhaustive report accepted")
			}
		}
	}
}

func TestCIShardExhaustiveReportsSurviveIndentedPartialRoundTrip(t *testing.T) {
	p := ciShardTestPlan(t, 3, "go/unicode:SimpleFold", "go/image/color:RGBToYCbCr", "go/image/color:YCbCrToRGB", "go/unicode:To", "go/unicode:ToUpper", "go/unicode:ToLower", "go/unicode:ToTitle")
	parts := ciShardTestPartials(t, p)
	for i := range parts {
		raw, err := json.MarshalIndent(parts[i], "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		var decoded ciShardPartial
		if err := decodeCIShardJSON(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		parts[i] = decoded
	}
	if _, err := reduceCIShardPartials(p, parts); err != nil {
		t.Fatal("valid reports changed meaning after the actual partial writer", err)
	}
}

func TestCIShardStrictJSONAndEarlyCLIValidation(t *testing.T) {
	_, run, _, _ := ciShardTestInputs(t, []string{"go/synthetic:A"})
	raw, _ := json.Marshal(run)
	var loaded ciShardRun
	if err := decodeCIShardJSON(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{bytes.Replace(raw, []byte(`"attempt":1,`), nil, 1), bytes.Replace(raw, []byte(`"attempt"`), []byte(`"Attempt"`), 1), bytes.Replace(raw, []byte(`"attempt":1`), []byte(`"attempt":1.0`), 1), append(append([]byte{}, raw...), []byte(` {}`)...), []byte(`{"schema":"one","\u0073chema":"two"}`)} {
		if err := decodeCIShardJSON(bad, &loaded); err == nil {
			t.Fatal("noncanonical/ambiguous JSON accepted", string(bad))
		}
	}
	root := t.TempDir()
	t.Chdir(root)
	priorFlags, priorArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = priorFlags, priorArgs })
	flag.CommandLine = flag.NewFlagSet("shard-early-error", flag.ContinueOnError)
	flag.CommandLine.SetOutput(&bytes.Buffer{})
	os.Args = []string{"verify", "--ci-shard-plan-out", "not-created/plan.json", "--ci-shard-context", "missing.json", "--ci-shard-count", "0"}
	if err := runCLIShardTest(); err == nil || !strings.Contains(err.Error(), "planning requires") {
		t.Fatal("invalid shard mode reached filesystem/upstream", err)
	}
	if _, err := os.Stat("not-created"); !os.IsNotExist(err) {
		t.Fatal("invalid mode created output")
	}
}

// Keep the local variable named run above separate from the production entry.
func runCLIShardTest() error { return run() }

func TestCIShardExplicitDefaultFlagsRejectBeforeFullWork(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root) // No upstream.json, compiler or corpus fixtures are present.
	priorFlags, priorArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = priorFlags, priorArgs })
	for _, option := range []string{
		"--ci-shard-count=0", "--ci-shard-index=-1", "--ci-shard-plan-out=",
		"--ci-shard-select-out=", "--ci-shard-reduce-out=", "--ci-shard-plan=", "--ci-shard-context=",
	} {
		for _, withOut := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/out=%t", option, withOut), func(t *testing.T) {
				flag.CommandLine = flag.NewFlagSet("shard-explicit-default", flag.ContinueOnError)
				flag.CommandLine.SetOutput(&bytes.Buffer{})
				os.Args = []string{"verify", option}
				if withOut {
					os.Args = append(os.Args, "--out", "not-created/evidence")
				}
				if err := run(); err == nil || !strings.Contains(err.Error(), "CI shard metadata mode requires") {
					t.Fatal("explicit shard default reached full source work", err)
				}
				if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
					t.Fatal("invalid metadata mode changed empty checkout", entries, err)
				}
			})
		}
	}
}

func TestCIShardParseErrorsAndPositionalsRejectBeforeFullWork(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	priorFlags, priorArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = priorFlags, priorArgs })
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--ci-shard-partial=", "--out", "not-created/evidence"}, "empty CI shard partial path"},
		{[]string{"--ci-shard-count=bad", "--out", "not-created/evidence"}, "invalid value"},
		{[]string{"--ci-shard-plan-out=not-created/plan.json", "--ci-shard-count=1", "--ci-shard-context=missing.json", "unexpected"}, "positional"},
	} {
		flag.CommandLine = flag.NewFlagSet("shard-parse-error", flag.ContinueOnError)
		flag.CommandLine.SetOutput(&bytes.Buffer{})
		os.Args = append([]string{"verify"}, test.args...)
		if err := run(); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatal("invalid metadata arguments reached filesystem/source work", test.args, err)
		}
		if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
			t.Fatal("invalid metadata arguments changed empty checkout", entries, err)
		}
	}
}

func TestCIShardFilesAreBoundedRegularAndNeverOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.json")
	original := []byte("retained\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeNewCIShardJSON(path, map[string]bool{"replace": true}); err == nil {
		t.Fatal("overwrote evidence")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Fatal("existing evidence changed")
	}
	if _, err := readCIShardFile(path, 2); err == nil {
		t.Fatal("oversized input accepted")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err == nil {
		if _, err := readCIShardFile(link, 100); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}

func TestCIShardSourcesBindTestedGitTreeAndRejectLaterChanges(t *testing.T) {
	// This is the independently known Git blob identity of the six bytes hello\n.
	const blob = "ce013625030ba8dba906f756967f9e9ca394464a"
	raw := []byte("hello\n")
	modes := map[string]string{"sample.gooo": "100644"}
	row := []byte("100644 blob " + blob + "\tsample.gooo\x00")
	entries, err := decodeCIShardGitSources(row, modes)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCIShardGitSource(raw, "100644", entries["sample.gooo"]); err != nil {
		t.Fatal("known Git blob rejected", err)
	}
	if err := validateCIShardGitSource([]byte("dirty\n"), "100644", entries["sample.gooo"]); err == nil {
		t.Fatal("new dirty bytes could be bound to the old tested tree")
	}
	if err := validateCIShardGitSource(raw, "100755", entries["sample.gooo"]); err == nil {
		t.Fatal("mode differs from immutable tested tree")
	}
	for _, bad := range [][]byte{
		row[:len(row)-1], append(append([]byte{}, row...), row...),
		bytes.Replace(row, []byte("sample.gooo"), []byte("foreign.gooo"), 1),
		bytes.Replace(row, []byte("100644"), []byte("100755"), 1),
		bytes.Replace(row, []byte("blob"), []byte("tree"), 1),
	} {
		if _, err := decodeCIShardGitSources(bad, modes); err == nil {
			t.Fatal("missing, duplicate, foreign or mode/type-altered Git binding accepted")
		}
	}
	root := t.TempDir()
	path := filepath.Join(root, "sample.gooo")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sources := map[string]ciShardSource{"sample.gooo": {Mode: "100644", Bytes: int64(len(raw)), SHA256: contentDigest(raw)}}
	if err := validateCIShardSourceSnapshot(root, sources); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("dirty\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateCIShardSourceSnapshot(root, sources); err == nil {
		t.Fatal("source edit after initial snapshot accepted")
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := validateCIShardSourceSnapshot(root, sources); err == nil {
			t.Fatal("source executable mode changed after initial snapshot")
		}
	}
}
