package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLastIndexSourceContractsAndVersionedEightFieldResult(t *testing.T) {
	for _, s := range []suite{{Package: literalTextPackage, Activity: "LastIndex"}, {Package: lastIndexConsumerPackage, Activity: "Report"}} {
		contract, err := contractFor(s)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile("../../" + contract.Source)
		if err != nil {
			t.Fatal(err)
		}
		declarations, err := decimalPayloadBodies(raw)
		body := declarations[s.Activity]
		if err != nil || len(declarations) != 1 || body.ID != contract.ID || !reflect.DeepEqual(body.Inputs, []string{"Text", "Text"}) {
			t.Fatal("new declaration identity or scalar inputs differ", s, err)
		}
		if s.Package == literalTextPackage {
			if body.Output != "Integer" || whitespaceResultContract(s).Entity != integerEntity {
				t.Fatal("LastIndex scalar result contract")
			}
			continue
		}
		if body.Output != "FinalMarkerReportV1" || !strings.Contains(string(raw), `entity FinalMarkerReportV1 id "`+lastIndexResultEntity+`" fields {`) {
			t.Fatal("separate versioned report result entity")
		}
		result := whitespaceResultContract(s)
		start := -1
		for i, field := range result.Fields {
			kind := "integer"
			if i < 2 {
				kind = "string"
			}
			if i == 4 {
				kind = "boolean"
			}
			token := `field ` + field.Name + ` id "` + field.ID + `" type ` + kind + ` required one`
			at := strings.Index(string(raw), token)
			if at <= start || at < 0 {
				t.Fatal("typed field identity/order differs", field)
			}
			start = at
		}
		if len(result.Fields) != 8 || !strings.Contains(body.Body, ".Index(") || !strings.Contains(body.Body, ".LastIndex(") || !strings.Contains(body.Body, ".Count(") {
			t.Fatal("separate Report must call all three imported Gooo activities")
		}
	}
	old := whitespaceResultContract(suite{Package: literalTextConsumerPackage, Activity: "Report"})
	if old.Entity != literalTextResultEntity || len(old.Fields) != 7 || old.Entity == lastIndexResultEntity || literalReportCompactTarget(suite{Package: lastIndexConsumerPackage, Activity: "Report"}) {
		t.Fatal("v1 report altered the legacy seven-field ABI or compact target")
	}
}

func TestLastIndexMinimalClosuresAndNoPreviousSourceLeak(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []suite{{Package: literalTextPackage, Activity: "LastIndex"}, {Package: lastIndexConsumerPackage, Activity: "Report"}} {
		selected, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"ports/strings/search.gooo", lastIndexSource}
		if s.Package == lastIndexConsumerPackage {
			want = []string{"ports/strings/search.gooo", "ports/strings/literal.gooo", lastIndexSource, lastIndexConsumerSource}
		}
		if !reflect.DeepEqual(selected.Names, want) || len(selected.Sources) != len(want) || len(selected.Identities) != len(want) {
			t.Fatal("new suite source closure differs", s, selected.Names)
		}
		profiled, err := dependencyClosureWorkspaceProfile.workspaceForSuite(frozen, s)
		if err != nil || !reflect.DeepEqual(profiled, selected) {
			t.Fatal("workspace profile changed the new source closure", err)
		}
		for _, name := range want {
			if !bytes.Equal(selected.Sources[name], frozen.Sources[name]) || selected.Identities[name].Digest == contentDigest(selected.Sources[name]) {
				t.Fatal("frozen bytes or JSON-source-text identity differs", name)
			}
		}
	}
	// Existing source selectors must remove the addition before the optional
	// dependency profile keeps whole already-selected packages.
	previous := 0
	for entry := range nativeActivityContracts() {
		pkg, activity, ok := strings.Cut(entry, ":")
		if !ok {
			t.Fatal("invalid trusted activity key", entry)
		}
		s := suite{Package: pkg, Activity: activity}
		if isLastIndexSuite(s) {
			continue
		}
		previous++
		for _, profile := range []nativeWorkspaceProfile{"", dependencyClosureWorkspaceProfile} {
			selected, err := profile.workspaceForSuite(frozen, s)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := selected.Sources[lastIndexSource]; ok {
				t.Fatal("LastIndex leaked into earlier workspace", s, profile)
			}
			if _, ok := selected.Sources[lastIndexConsumerSource]; ok {
				t.Fatal("v1 Report leaked into earlier workspace", s, profile)
			}
		}
	}
	if previous != 80 {
		t.Fatal("prior source-selector contract roster changed", previous)
	}
	for _, mutate := range []func(*frozenVerificationCatalog){
		func(f *frozenVerificationCatalog) { delete(f.Sources, lastIndexSource) },
		func(f *frozenVerificationCatalog) { f.Hashes[lastIndexSource] = strings.Repeat("0", 64) },
		func(f *frozenVerificationCatalog) {
			f.Catalog.Packages = append(f.Catalog.Packages, sourcePackage{Path: lastIndexConsumerPackage, Sources: []string{lastIndexConsumerSource}})
		},
		func(f *frozenVerificationCatalog) {
			for i := range f.Catalog.Packages {
				if f.Catalog.Packages[i].Path == literalTextPackage {
					f.Catalog.Packages[i].Sources = append(f.Catalog.Packages[i].Sources, lastIndexSource)
				}
			}
		},
	} {
		f, err := freezeVerificationCatalog("../../verification-catalog.json")
		if err != nil {
			t.Fatal(err)
		}
		mutate(&f)
		if _, err := f.workspaceForSuite(suite{Package: lastIndexConsumerPackage, Activity: "Report"}); err == nil {
			t.Fatal("altered source catalog accepted")
		}
	}
}

func TestLastIndexUpstreamAndDedicatedManifestMetadata(t *testing.T) {
	var lock struct {
		GoVersion        string            `json:"go_version"`
		GoRevision       string            `json:"go_revision"`
		CompilerRevision string            `json:"compiler_revision"`
		SourceSHA        map[string]string `json:"source_sha256"`
		Ports            []struct {
			Source            string   `json:"source"`
			Target            string   `json:"target"`
			SupportingTargets []string `json:"supporting_targets"`
			Symbols           []string `json:"symbols"`
			Activities        []string `json:"target_activities"`
			Mapping           string   `json:"api_mapping"`
		} `json:"ports"`
	}
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	if lock.GoVersion != "go1.27.2" || lock.GoRevision != "022c8636110ebac86a9b88724cda168ed713c21f" || lock.CompilerRevision != uuidCompactCompilerRevision || lock.SourceSHA["src/strings/strings.go"] != "a81bdf73a308277c322e35d0c602e3813f57b6f769c091f8f8962c8884a45ea3" {
		t.Fatal("upstream/compiler identities changed")
	}
	found := 0
	for _, port := range lock.Ports {
		if port.Target != lastIndexSource {
			continue
		}
		found++
		if port.Source != "src/strings/strings.go" || !reflect.DeepEqual(port.Symbols, []string{"LastIndex"}) || !reflect.DeepEqual(port.Activities, []string{"LastIndex"}) || !reflect.DeepEqual(port.SupportingTargets, []string{"ports/strings/search.gooo"}) || !strings.Contains(port.Mapping, "finite v1 corpus") || !strings.Contains(port.Mapping, "9313-pair corpus and compact profile remain unchanged") {
			t.Fatal("LastIndex upstream mapping differs")
		}
	}
	if found != 1 {
		t.Fatal("missing or duplicate LastIndex upstream entry")
	}
	f, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	s := suite{Package: lastIndexConsumerPackage, Activity: "Report"}
	selected, err := f.workspaceForSuite(s)
	if err != nil {
		t.Fatal(err)
	}
	manifest := (suiteWorkspace{}).Manifest
	if err := readJSON("../../literal-final-marker-report.workspace.json", &manifest); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manifest, selected.Manifest) {
		t.Fatal("dedicated v1 manifest differs from trusted source closure")
	}
}

func TestLastIndexNativePartitionContract(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var lock upstream
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	added, err := lastIndexCorpus()
	if err != nil {
		t.Fatal(err)
	}
	totalCases, totalBatches := 0, 0
	for _, s := range added.Suites {
		selected, err := dependencyClosureWorkspaceProfile.workspaceForSuite(frozen, s)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := uuidLiteralReportCompactCaseProfile.policy(s, selected, lock)
		if err != nil || policy != (nativeCaseBatchPolicy{}) {
			t.Fatal("new entries must retain the bounded pretty writer", err)
		}
		batches := 0
		for start := 0; start < len(s.Cases); {
			prepared, err := policy.prepare(s, start)
			if err != nil || prepared.End <= start || prepared.End-start > 128 || prepared.Raw != nil {
				t.Fatal("invalid new pretty batch", err)
			}
			batch := s
			batch.Cases = s.Cases[start:prepared.End]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > stringByteMaxBatchBytes {
				t.Fatal("new batch exceeds exact serialized-byte bound", err)
			}
			if prepared.End < len(s.Cases) && prepared.End-start < 128 {
				batch.Cases = s.Cases[start : prepared.End+1]
				next, err := json.MarshalIndent(caseDocument(batch), "", "  ")
				if err != nil || len(next)+1 <= stringByteMaxBatchBytes {
					t.Fatal("new byte-bounded partition is not maximal", err)
				}
			}
			start = prepared.End
			batches++
		}
		totalCases += len(s.Cases)
		totalBatches += batches
		t.Logf("LastIndex finite partition %s:%s cases=%d batches=%d source_set=%s", s.Package, s.Activity, len(s.Cases), batches, strings.Join(selected.Names, "|"))
	}
	t.Logf("LastIndex addition cases=%d batches=%d timing_events=%d (native execution/replay not performed by this partition test)", totalCases, totalBatches, 4+totalBatches)
}

// Synthetic receipts test evidence rejection, never substitute for the fresh
// native execution and saved replay performed by the complete verifier.
func TestLastIndexReceiptRejectsCorrelatedAndTypedIdentityTampering(t *testing.T) {
	for _, pair := range []literalTextPair{{"aaaaa", "aa"}, {"가x가", "가"}, {"가🙂\x00", ""}, {"A\x00B\x00", "\x00"}} {
		for _, entry := range []suite{{Package: literalTextPackage, Activity: "LastIndex"}, {Package: lastIndexConsumerPackage, Activity: "Report"}} {
			want := any(strings.LastIndex(pair.Text, pair.Needle))
			if entry.Package == lastIndexConsumerPackage {
				want = lastIndexReportOracle(pair.Text, pair.Needle)
			}
			entry.Cases = []testCase{{Inputs: []any{pair.Text, pair.Needle}, Expected: want}}
			raw, one, id := contractFixture(t, entry)
			changes := map[string]func(*envelope){
				"input order": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Inputs[0].Value, d.Inputs[1].Value = d.Inputs[1].Value, d.Inputs[0].Value
				},
				"input entity":      func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Entity = integerEntity },
				"input scalar type": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Value = mustJSON(t, 0) },
				"input scalar fields": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Fields = []recordFieldValue{{"bad", "bad", mustJSON(t, pair.Text)}}
				},
				"entry": func(e *envelope) { e.Result.Program.Entry.Activity = "Index" },
				"source digest": func(e *envelope) {
					e.Result.Program.Image.Packages[0].Sources[0].Digest = contentDigest([]byte("other source"))
				},
				"source owner":     func(e *envelope) { e.Result.Program.Image.Packages[0].Path = "other" },
				"export identity":  func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].ID += "-wrong" },
				"output entity":    func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].OutputType = textEntity },
				"producer":         func(e *envelope) { e.Result.Runtime.Producer = strings.Repeat("f", 40) },
				"toolchain":        func(e *envelope) { e.Result.Runtime.GoVersion = "go version go1.0 linux/amd64" },
				"case binding":     func(e *envelope) { e.CasesDigest = contentDigest([]byte("other cases")) },
				"manifest binding": func(e *envelope) { e.ManifestDigest = contentDigest([]byte("other manifest")) },
				"correlated last offset": func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					wrong := any(strings.LastIndex(pair.Text, pair.Needle) + 1)
					if entry.Package == lastIndexConsumerPackage {
						v := lastIndexReportOracle(pair.Text, pair.Needle).(map[string]any)
						v["last_byte_offset"] = wrong
						wrong = v
						d.ActualFields[3].Value = mustJSON(t, v["last_byte_offset"])
					}
					d.Expected, d.Actual = mustJSON(t, wrong), mustJSON(t, wrong)
				},
			}
			if entry.Package == lastIndexConsumerPackage {
				changes["missing last field"] = func(e *envelope) {
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.ActualFields = append(d.ActualFields[:3], d.ActualFields[4:]...)
				}
				changes["field identity"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[3].ID += "-wrong" }
				changes["field scalar type"] = func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[3].Value = mustJSON(t, "3") }
				changes["field order"] = func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[2], f[3] = f[3], f[2] }
			} else {
				changes["scalar with record fields"] = func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = []recordFieldValue{{"bad", "bad", mustJSON(t, 0)}}
				}
			}
			for name, change := range changes {
				t.Run(entry.Package+"/"+name, func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(raw, &e); err != nil {
						t.Fatal(err)
					}
					change(&e)
					if _, err := inspect(mustJSON(t, e), one, false, id); err == nil {
						t.Fatal("altered evidence accepted")
					}
				})
			}
			original, err := inspect(raw, one, false, id)
			if err != nil {
				t.Fatal(err)
			}
			var e envelope
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			zero := 0
			e.Result.Replay = &struct {
				Calls *int `json:"model_calls"`
			}{&zero}
			e.ReplayedFrom = contentDigest(raw)
			replayed, err := inspect(mustJSON(t, e), one, true, id)
			if err != nil || inspectReplayBinding(original, replayed, contentDigest(raw)) != nil {
				t.Fatal("valid saved replay rejected", err)
			}
			if inspectReplayBinding(original, replayed, contentDigest([]byte("other receipt"))) == nil {
				t.Fatal("stale replay parent accepted")
			}
			replayed.Generated = contentDigest([]byte("other generated program"))
			if inspectReplayBinding(original, replayed, contentDigest(raw)) == nil {
				t.Fatal("changed replay generated program accepted")
			}
		}
	}
}
