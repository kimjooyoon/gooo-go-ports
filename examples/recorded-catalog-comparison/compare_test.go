package recordedcatalog

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func reportFor(t *testing.T, in Input) Report {
	t.Helper()
	r, err := CompareJSON(encode(t, in))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func pointer(s string) *string { return &s }

func TestSixRecordedCatalogScenarios(t *testing.T) {
	cases := []struct {
		name, alias, context string
		mutate               func(*Input)
	}{
		{"same claimed name with two connections", "ORIGIN_CHANGED", "RECORDED_CONTEXT_COMPLETE", func(in *Input) {
			in.TargetAlias = "docs_search"
			for _, o := range []*Observation{&in.Before, &in.After} {
				o.DeclaredConnections = []Connection{{"host", "a"}, {"host", "b"}}
				o.Captures = []Capture{fixtureCapture("a", "docs_search"), fixtureCapture("b", "other_search")}
			}
			in.After.Captures[0].Pages[0].DeclaredExposedNames[0] = "other_search"
			in.After.Captures[1].Pages[0].DeclaredExposedNames[0] = "docs_search"
		}},
		{"backend changes between captures", "ORIGIN_CHANGED", "RECORDED_CONTEXT_COMPLETE", func(in *Input) {
			c := &in.After.Captures[0]
			c.Begin.BackendBindingRevision = "backend-r2"
			c.End.BackendBindingRevision = "backend-r2"
			c.Pages[0].Context.BackendBindingRevision = "backend-r2"
		}},
		{"alias only change", "REMOVED", "RECORDED_CONTEXT_COMPLETE", func(in *Input) { in.After.Captures[0].Pages[0].DeclaredExposedNames[0] = "docs_search" }},
		{"missing middle page", "REMOVED", "INCOMPLETE_COLLECTION", func(in *Input) {
			in.After.Captures[0].Pages = []Page{fixturePage([]string{"one"}, []string{"one"}, nil, pointer("middle")), fixturePage([]string{"last"}, []string{"last"}, pointer("last"), nil)}
		}},
		{"change during capture", "UNCHANGED", "UNSTABLE_CONFIGURATION", func(in *Input) { in.After.Captures[0].End.RelevantConfigurationRevision = "config-r2" }},
		{"identical saved input", "UNCHANGED", "RECORDED_CONTEXT_COMPLETE", func(*Input) {}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			in := fixtureInput()
			tt.mutate(&in)
			raw := encode(t, in)
			r, err := CompareJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			if r.AliasDecision.State != tt.alias || r.CollectionContext.State != tt.context {
				t.Fatalf("want %s/%s; got %+v / %+v", tt.alias, tt.context, r.AliasDecision, r.CollectionContext)
			}
			if !r.CollectionContext.ContainsSyntheticFixture || r.CollectionContext.RemoteAtomicity != "UNKNOWN" || r.CollectionContext.LiveRemovalProven || r.CollectionContext.ExecutionAuthorized || r.AliasDecision.ExecutionAuthorized {
				t.Fatal("scope or authority boundary changed")
			}
			if !reflect.DeepEqual(r.Facts, in) || r.InputSHA256 != digest(raw) {
				t.Fatal("input not preserved")
			}
			again, err := CompareJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encode(t, r), encode(t, again)) {
				t.Fatal("same saved input did not replay deterministically")
			}
		})
	}
}

func TestCollectionContextBoundaries(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*Input)
	}{
		{"retrospective", "COLLECTION_CONTEXT_UNKNOWN", func(in *Input) { in.After.Captures[0].ContextRecordingBasis = "RETROSPECTIVE" }},
		{"unknown basis", "COLLECTION_CONTEXT_UNKNOWN", func(in *Input) { in.Before.Captures[0].ContextRecordingBasis = "UNKNOWN" }},
		{"endpoints not continuous", "COLLECTION_CONTEXT_UNKNOWN", func(in *Input) { in.After.Captures[0].ConfigurationWatch.Coverage = "ENDPOINTS_ONLY" }},
		{"observed ABA", "UNSTABLE_CONFIGURATION", func(in *Input) { in.After.Captures[0].ConfigurationWatch.RelevantChangeObserved = true }},
		{"unknown cannot hide missing capture", "INCOMPLETE_COLLECTION", func(in *Input) {
			in.Before.Captures[0].ContextRecordingBasis = "RETROSPECTIVE"
			in.After.Captures = []Capture{}
		}},
		{"unknown cannot hide unstable", "UNSTABLE_CONFIGURATION", func(in *Input) {
			in.Before.Captures[0].ContextRecordingBasis = "UNKNOWN"
			in.After.Captures[0].End.BackendBindingRevision = "changed"
		}},
		{"empty required host id", "INVALID_CONTEXT", func(in *Input) { in.After.Captures[0].HostID = "" }},
		{"missing declared capture", "INCOMPLETE_COLLECTION", func(in *Input) { in.After.Captures = []Capture{} }},
		{"zero pages", "INCOMPLETE_COLLECTION", func(in *Input) { in.After.Captures[0].Pages = []Page{} }},
		{"duplicate declared", "INVALID_CONTEXT", func(in *Input) {
			in.After.DeclaredConnections = append(in.After.DeclaredConnections, in.After.DeclaredConnections[0])
		}},
		{"duplicate captured", "INVALID_CONTEXT", func(in *Input) { in.After.Captures = append(in.After.Captures, in.After.Captures[0]) }},
		{"undeclared capture", "INVALID_CONTEXT", func(in *Input) { in.After.Captures = append(in.After.Captures, fixtureCapture("b", "other")) }},
		{"catalog auth changes", "AUTH_CONTEXT_CHANGED", func(in *Input) { in.After.CatalogAuthorizationContextID = "other-scope" }},
		{"retained connection auth changes", "AUTH_CONTEXT_CHANGED", func(in *Input) {
			c := &in.After.Captures[0]
			c.Begin.AuthorizationContextID = "auth-b"
			c.End.AuthorizationContextID = "auth-b"
			c.Pages[0].Context.AuthorizationContextID = "auth-b"
		}},
		{"connection change is comparison input", "RECORDED_CONTEXT_COMPLETE", func(in *Input) {
			in.After.DeclaredConnections[0].ConnectionID = "b"
			in.After.Captures[0].ConnectionID = "b"
		}},
		{"initial continuation incomplete", "INCOMPLETE_COLLECTION", func(in *Input) {
			in.After.Captures[0].Pages = []Page{fixturePage([]string{"search"}, []string{"search"}, pointer(""), nil)}
		}},
		{"empty next cursor needs continuation", "INCOMPLETE_COLLECTION", func(in *Input) {
			in.After.Captures[0].Pages = []Page{fixturePage([]string{"search"}, []string{"search"}, nil, pointer(""))}
		}},
		{"linked empty cursor", "RECORDED_CONTEXT_COMPLETE", func(in *Input) {
			in.After.Captures[0].Pages = []Page{fixturePage([]string{"search"}, []string{"search"}, nil, pointer("")), fixturePage([]string{}, []string{}, pointer(""), nil)}
		}},
		{"repeated cursor", "INCOMPLETE_COLLECTION", func(in *Input) {
			in.After.Captures[0].Pages = []Page{fixturePage([]string{"search"}, []string{"search"}, nil, pointer("a")), fixturePage([]string{}, []string{}, pointer("a"), pointer("a")), fixturePage([]string{}, []string{}, pointer("a"), nil)}
		}},
		{"pages after terminal", "INCOMPLETE_COLLECTION", func(in *Input) {
			in.After.Captures[0].Pages = append(in.After.Captures[0].Pages, fixturePage([]string{}, []string{}, nil, nil))
		}},
		{"explicit empty scope", "RECORDED_CONTEXT_COMPLETE", func(in *Input) {
			in.Before = Observation{CatalogAuthorizationContextID: "scope", DeclaredConnections: []Connection{}, Captures: []Capture{}}
			in.After = in.Before
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			in := fixtureInput()
			tt.mutate(&in)
			r := reportFor(t, in)
			if r.CollectionContext.State != tt.want {
				t.Fatalf("want %s got %+v", tt.want, r.CollectionContext)
			}
		})
	}
}

func TestBoundFactsAndCoreProjection(t *testing.T) {
	in := fixtureInput()
	r := reportFor(t, in)
	if r.Projection.FactsSHA256 != digest(encode(t, r.Projection.Facts)) || r.AliasDecision.ContextID != "gooo/alias-origin/v1/"+r.Projection.FactsSHA256 || r.FactsSHA256 != digest(encode(t, in)) {
		t.Fatal("facts binding changed")
	}
	whitespace := append([]byte(" \n"), encode(t, in)...)
	changed, err := CompareJSON(whitespace)
	if err != nil {
		t.Fatal(err)
	}
	if changed.InputSHA256 == r.InputSHA256 || changed.FactsSHA256 != r.FactsSHA256 || changed.CollectionContext.ContextID != r.CollectionContext.ContextID {
		t.Fatal("raw input and decoded facts hashes conflated")
	}
	in.After.Captures[0].ServerNameClaim = "new informational claim"
	other := reportFor(t, in)
	if other.AliasDecision != r.AliasDecision || other.CollectionContext.ContextID == r.CollectionContext.ContextID {
		t.Fatal("complete observation binding or informational origin separation lost")
	}
	in.After.Captures[0].Pages[0].DeclaredExposedNames = []string{"search"}
	in.After.Captures[0].Pages[0].ResponseJSON = `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"search"}],"nextCursor":null}}`
	other = reportFor(t, in)
	if other.CollectionContext.State != "RECORDED_CONTEXT_COMPLETE" {
		t.Fatal("null response continuation should end chain")
	}
}

func TestInvalidFrameReturnsNoPartialReport(t *testing.T) {
	in := fixtureInput()
	in.After.Captures[0].Pages[0].ResponseJSON = `{"jsonrpc":"2.0","id":3,"result":{"tools":[]}}`
	r, err := CompareJSON(encode(t, in))
	if err == nil || !reflect.DeepEqual(r, Report{}) {
		t.Fatal("partial decision returned for invalid frame")
	}
}

func TestRawFactsSnapshotIsIndependent(t *testing.T) {
	in := fixtureInput()
	raw := encode(t, in)
	r, err := CompareJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	for i := range raw {
		raw[i] = 'x'
	}
	if r.Facts.TargetAlias != "search" || !json.Valid(r.Bindings) {
		t.Fatal("returned report aliases mutable caller bytes")
	}
}

func TestPageAuthChangeDoesNotDisappearInEndpointRelation(t *testing.T) {
	in := fixtureInput()
	in.After.Captures[0].Pages[0].Context.AuthorizationContextID = "temporary-auth"
	r := reportFor(t, in)
	if r.CollectionContext.State != "UNSTABLE_CONFIGURATION" || r.CollectionContext.AuthorizationRelation != "SAME_RECORDED" || r.CollectionContext.AfterConfigurationState != "CHANGED" {
		t.Fatalf("page auth observation lost or endpoint relation overclaimed: %+v", r.CollectionContext)
	}
}

// The verifier requests this explicit evidence export after normal assertions.
// It records the actual private projection used for the public CLI fixtures;
// expected public states are fixture literals, not inferred from kernel output.
func TestConsumerFixturesAndExport(t *testing.T) {
	raw, err := os.ReadFile("consumer-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Label   string `json:"label"`
			Input   Input  `json:"input"`
			Alias   string `json:"expected_alias_state"`
			Context string `json:"expected_collection_state"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 6 {
		t.Fatal("six consumer scenarios required")
	}
	records := []map[string]any{}
	for _, c := range corpus.Cases {
		r := reportFor(t, c.Input)
		if r.AliasDecision.State != c.Alias || r.CollectionContext.State != c.Context {
			t.Fatalf("%s: unexpected %s/%s", c.Label, r.AliasDecision.State, r.CollectionContext.State)
		}
		b, err := projectObservation("BEFORE", c.Input.Before)
		if err != nil {
			t.Fatal(err)
		}
		a, err := projectObservation("AFTER", c.Input.After)
		if err != nil {
			t.Fatal(err)
		}
		origin, context, _, err := columns(c.Input, b, a, r.FactsSHA256)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, map[string]any{"label": c.Label, "input": c.Input, "origin": origin, "collection": context, "report": r, "expected_alias_state": c.Alias, "expected_collection_state": c.Context})
	}
	if target := os.Getenv("GOOO_RECORDED_EXPORT"); target != "" {
		abs, err := filepath.Abs(target)
		if err != nil || abs != target {
			t.Fatal("absolute evidence export required")
		}
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		root := filepath.Dir(filepath.Dir(cwd))
		rel, err := filepath.Rel(root, abs)
		if err != nil || rel == ".." || len(rel) < 3 || rel[:3] != ".."+string(filepath.Separator) {
			t.Fatal("evidence export must be outside checkout")
		}
		f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.Write(append(encode(t, records), '\n'))
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal(writeErr, closeErr)
		}
	}
}
