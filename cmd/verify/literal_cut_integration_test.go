package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// This source/corpus check keeps the original62 catalog, factories and cases.
// Its legacy roster uses current sources, with only the exact reviewed UUID
// quote rewrite below allowed. It does not claim those bytes or a native PASS
// are the historical original62 source image.
func TestLiteralCutPreservesOriginal62CasesWorkspacesAndBatchBoundaries(t *testing.T) {
	var base struct {
		Schema    string              `json:"schema"`
		Catalog   verificationCatalog `json:"catalog"`
		Sources   map[string]string   `json:"source_sha256"`
		Factories map[string]string   `json:"factory_sha256"`
	}
	raw, err := os.ReadFile("testdata/literal-cut-original62.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &base); err != nil || base.Schema != "gooo/literal-cut-original62-source-freeze/v1" || len(base.Sources) != 49 || len(base.Factories) != 21 {
		t.Fatal("original source freeze", err)
	}
	const quotePath = "ports/google/uuid/quote.gooo"
	const historicalQuoteSHA = "b9b0a9d642a91e2baba25b7636e875f6e5de9713bb7c4913368cd564cf34e339"
	const reviewedCurrentQuoteSHA = "3a14a762ceed11df29b698ab3eb298655a6737bbba76fe5dbb38dbc81baeb7de"
	if base.Sources[quotePath] != historicalQuoteSHA {
		t.Fatal("historical UUID quote source pin changed")
	}
	legacyRosterCurrentSources := frozenVerificationCatalog{Catalog: base.Catalog, Sources: map[string][]byte{}, Hashes: map[string]string{}}
	for name, historicalSHA := range base.Sources {
		wantCurrentSHA := historicalSHA
		if name == quotePath {
			wantCurrentSHA = reviewedCurrentQuoteSHA
		}
		actual, err := os.ReadFile("../../" + name)
		if err != nil || contentDigest(actual) != "sha256:"+wantCurrentSHA {
			t.Fatal("legacy roster current Gooo source differs from reviewed pin", name, err)
		}
		legacyRosterCurrentSources.Sources[name], legacyRosterCurrentSources.Hashes[name] = actual, wantCurrentSHA
	}
	for name, want := range base.Factories {
		actual, err := os.ReadFile("../../" + name)
		if err != nil || contentDigest(actual) != "sha256:"+want {
			t.Fatal("original case factory or row limit changed", name, err)
		}
	}
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	all, before := allNativeSuites(t), original62NativeSuites(t)
	if len(all) != 66 || len(before) != 62 || !reflect.DeepEqual(all[:62], before) {
		t.Fatal("old suite order or exact cases changed")
	}
	cases, batches := 0, 0
	for _, s := range before {
		legacySelected, err := legacyRosterCurrentSources.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		current, err := frozen.workspaceForSuite(s)
		if err != nil || !reflect.DeepEqual(legacySelected, current) {
			t.Fatal("current source roster/manifest differs from original62 catalog", s.Package, s.Activity, err)
		}
		cases += len(s.Cases)
		for start := 0; start < len(s.Cases); {
			end, err := nativeBatchEnd(s, start, nativeBatchSize(s))
			if err != nil || end <= start {
				t.Fatal("original batch partition", err)
			}
			start = end
			batches++
		}
	}
	if cases != 159781 || batches != 2332 {
		t.Fatal("original62 case/batch totals changed", cases, batches)
	}
	// Record only actual calculations made by this test; do not predeclare the
	// new byte-derived batch total. Root freezes these values into CI after it runs.
	newBatches := 0
	for i, s := range all[62:] {
		count := 0
		for start := 0; start < len(s.Cases); {
			end, err := nativeBatchEnd(s, start, nativeBatchSize(s))
			if err != nil || end <= start {
				t.Fatal("new batch partition", err)
			}
			start = end
			count++
		}
		if len(s.Cases) != 32 || count < 1 {
			t.Fatal("new suite roster", s.Package, s.Activity)
		}
		newBatches += count
		t.Logf("literal-cut partition suite=%d %s:%s cases=%d batches=%d", i+62, s.Package, s.Activity, len(s.Cases), count)
	}
	t.Logf("literal-cut proposed integrated totals suites=66 cases=159909 batches=%d event_count=%d", batches+newBatches, 2+2*len(all)+batches+newBatches)
}

func TestLiteralCutWorkspaceClosureAndNoLegacySourceLeak(t *testing.T) {
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	const cut = "ports/strings/cut.gooo"
	const report = "examples/literal-cut-report/main.gooo"
	for _, s := range []suite{{Package: literalCutPackage, Activity: "Cut"}, {Package: literalCutPackage, Activity: "CutPrefix"}, {Package: literalCutPackage, Activity: "CutSuffix"}, {Package: literalCutConsumerPackage, Activity: "Report"}} {
		selected, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"ports/strings/search.gooo", "ports/strings/literal.gooo", cut}
		if s.Package == literalCutConsumerPackage {
			want = append(want, report)
		}
		if !reflect.DeepEqual(selected.Names, want) || len(selected.Sources) != len(want) {
			t.Fatal("Cut closure", selected.Names, want)
		}
	}
	// Both passes select workspaces for the same original inputs. Only the
	// frozen source map changes below; workspace selection does not mutate cases.
	legacySuites := original62NativeSuites(t)
	for _, s := range legacySuites {
		before, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := before.Sources[cut]; exists {
			t.Fatal("Cut leaked into old workspace")
		}
		if _, exists := before.Sources[report]; exists {
			t.Fatal("new Report leaked into old workspace")
		}
	}
	// Source selection must occur before an unrelated Cut file is copied/hashed.
	frozen.Sources[cut][0] ^= 1
	delete(frozen.Sources, report)
	for _, s := range legacySuites {
		if _, err := frozen.workspaceForSuite(s); err != nil {
			t.Fatal("unrelated Cut source affected old workspace", err)
		}
	}
}

func TestLiteralCutResultDeclarationsFixFieldTypesAndOrder(t *testing.T) {
	for _, s := range []suite{{Package: literalCutPackage, Activity: "Cut"}, {Package: literalCutPackage, Activity: "CutPrefix"}, {Package: literalCutPackage, Activity: "CutSuffix"}, {Package: literalCutConsumerPackage, Activity: "Report"}} {
		contract, err := contractFor(s)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile("../../" + contract.Source)
		if err != nil {
			t.Fatal(err)
		}
		result := literalCutResultContract(s)
		marker := "id \"" + result.Entity + "\" fields {"
		at := strings.Index(string(raw), marker)
		if at < 0 {
			t.Fatal("result declaration missing", result.Entity)
		}
		block := string(raw)[at+len(marker):]
		end := strings.Index(block, "}")
		if end < 0 {
			t.Fatal("incomplete result declaration")
		}
		block = block[:end]
		previous := 0
		for _, f := range result.Fields {
			kind := "string"
			if f.Name == "found" || strings.HasSuffix(f.Name, "_found") {
				kind = "boolean"
			}
			if strings.HasSuffix(f.Name, "_byte_length") {
				kind = "integer"
			}
			declaration := "field " + f.Name + " id \"" + f.ID + "\" type " + kind + " required one"
			at := strings.Index(block, declaration)
			if at < previous {
				t.Fatal("field type, ID or order", s.Activity, f.Name)
			}
			previous = at + len(declaration)
		}
		if strings.Count(block, "field ") != len(result.Fields) {
			t.Fatal("extra result field", s.Activity)
		}
	}
}

func TestLiteralCutWorkspaceRejectsBrokenOrExpandedClosure(t *testing.T) {
	const search = "ports/strings/search.gooo"
	const literal = "ports/strings/literal.gooo"
	const cut = "ports/strings/cut.gooo"
	const report = "examples/literal-cut-report/main.gooo"
	mutations := map[string]func(*frozenVerificationCatalog){
		"absent shared declarations": func(f *frozenVerificationCatalog) { delete(f.Sources, search) },
		"absent Index helper":        func(f *frozenVerificationCatalog) { delete(f.Sources, literal) },
		"absent Cut":                 func(f *frozenVerificationCatalog) { delete(f.Sources, cut) },
		"absent Report":              func(f *frozenVerificationCatalog) { delete(f.Sources, report) },
		"changed Cut bytes":          func(f *frozenVerificationCatalog) { f.Sources[cut][0] ^= 1 },
		"changed Report bytes":       func(f *frozenVerificationCatalog) { f.Sources[report][0] ^= 1 },
		"reordered port closure": func(f *frozenVerificationCatalog) {
			for i := range f.Catalog.Packages {
				if f.Catalog.Packages[i].Path == literalCutPackage {
					f.Catalog.Packages[i].Sources = []string{search, cut, literal}
				}
			}
		},
		"extra port source": func(f *frozenVerificationCatalog) {
			for i := range f.Catalog.Packages {
				if f.Catalog.Packages[i].Path == literalCutPackage {
					f.Catalog.Packages[i].Sources = append(f.Catalog.Packages[i].Sources, "ports/time/duration.gooo")
				}
			}
		},
		"extra consumer source": func(f *frozenVerificationCatalog) {
			for i := range f.Catalog.Packages {
				if f.Catalog.Packages[i].Path == literalCutConsumerPackage {
					f.Catalog.Packages[i].Sources = append(f.Catalog.Packages[i].Sources, "examples/response.gooo")
				}
			}
		},
		"rehashed absent import": func(f *frozenVerificationCatalog) {
			f.Sources[report] = append(f.Sources[report], []byte("\nimport absent \"app/response\"\n")...)
			f.Hashes[report] = strings.TrimPrefix(contentDigest(f.Sources[report]), "sha256:")
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
			if err != nil {
				t.Fatal(err)
			}
			mutate(&frozen)
			if _, err := frozen.workspaceForSuite(suite{Package: literalCutConsumerPackage, Activity: "Report"}); err == nil {
				t.Fatal("accepted broken or expanded Cut closure")
			}
		})
	}
	for _, s := range []suite{{Package: literalTextPackage, Activity: "Index"}, {Package: literalTextPackage, Activity: "Count"}, {Package: literalTextPackage, Activity: "IndexByte"}, {Package: literalCutConsumerPackage, Activity: "Cut"}, {Package: "other", Activity: "Cut"}} {
		if isLiteralCutSuite(s) {
			t.Fatal("Cut selector matched another API", s)
		}
	}
}

// Synthetic receipts exercise every public result field against an independent
// Go expectation. They do not claim that synthetic generated code executed.
func TestLiteralCutFiniteReceiptRejectsTypedResultAndIdentityTampering(t *testing.T) {
	cut, err := literalCutCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range cut.Suites {
		s := s
		t.Run(s.Activity, func(t *testing.T) {
			raw, one, identity := contractFixture(t, s)
			mutations := map[string]func(*envelope){
				"wrong input order": func(e *envelope) { d := &e.Result.Runtime.Traces[0].Deliveries[0]; d.Inputs[0].Port = "input1" },
				"wrong original input": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[0].Value = mustJSON(t, "changed input")
				},
				"scalar input record fields": func(e *envelope) {
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[1].Fields = []recordFieldValue{{ID: "wrong", Name: "wrong", Value: mustJSON(t, "")}}
				},
				"missing result fields": func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields = nil },
				"result field ID":       func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ActualFields[0].ID += "-wrong" },
				"result field order":    func(e *envelope) { f := e.Result.Runtime.Traces[0].Deliveries[0].ActualFields; f[0], f[1] = f[1], f[0] },
				"source digest": func(e *envelope) {
					e.Result.Program.Image.Packages[0].Sources[0].Digest = contentDigest([]byte("changed source"))
				},
				"producer":  func(e *envelope) { e.Result.Runtime.Producer = strings.Repeat("f", 40) },
				"toolchain": func(e *envelope) { e.Result.Runtime.GoVersion = "go version go1.0 linux/amd64" },
			}
			for i, field := range literalCutResultContract(s).Fields {
				i, field := i, field
				mutations["typed correlated field/"+field.Name] = func(e *envelope) {
					wrong := map[string]any{}
					for name, value := range one.Cases[0].Expected.(map[string]any) {
						wrong[name] = value
					}
					switch value := wrong[field.Name].(type) {
					case string:
						wrong[field.Name] = value + "changed"
					case bool:
						wrong[field.Name] = !value
					case int:
						wrong[field.Name] = value + 1
					default:
						t.Fatal("unhandled oracle field type", field.Name)
					}
					d := &e.Result.Runtime.Traces[0].Deliveries[0]
					d.Expected, d.Actual = mustJSON(t, wrong), mustJSON(t, wrong)
					d.ActualFields[i].Value = mustJSON(t, wrong[field.Name])
				}
			}
			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(raw, &e); err != nil {
						t.Fatal(err)
					}
					mutate(&e)
					if _, err := inspect(mustJSON(t, e), one, false, identity); err == nil {
						t.Fatal("accepted altered finite evidence")
					}
				})
			}
			original, err := inspect(raw, one, false, identity)
			if err != nil {
				t.Fatal(err)
			}
			if err := inspectReplayBinding(original, checked{Generated: original.Generated, ReplayedFrom: contentDigest([]byte("another receipt"))}, contentDigest(raw)); err == nil {
				t.Fatal("accepted different saved receipt")
			}
		})
	}
}
