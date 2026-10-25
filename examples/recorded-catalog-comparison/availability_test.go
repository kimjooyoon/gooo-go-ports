// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/availabilitykernel"
	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/availabilityobservationkernel"
)

func TestExplicitAvailabilityGeneratedFiniteCases(t *testing.T) {
	raw, err := os.ReadFile("availability-kernel-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Inputs   map[string]json.RawMessage            `json:"inputs"`
			Expected map[string]ExplicitDefinitionDecision `json:"expected"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 40 {
		t.Fatal(len(corpus.Cases))
	}
	for n, c := range corpus.Cases {
		var origin availabilitykernel.OriginInput
		var collection availabilitykernel.CollectionInput
		var definition availabilitykernel.DefinitionInput
		var availability ExplicitAvailabilityInput
		for index, input := range []any{&origin, &collection, &definition, &availability} {
			key := availabilityEntry + ".input" + string(rune('0'+index))
			if err := json.Unmarshal(c.Inputs[key], input); err != nil {
				t.Fatal(n, err)
			}
		}
		got := availabilitykernel.Evaluate(origin, collection, definition, availability)
		if !reflect.DeepEqual(got, c.Expected[availabilityEntry]) {
			t.Fatalf("case %d: %+v != %+v", n, got, c.Expected[availabilityEntry])
		}
	}
	raw, err = os.ReadFile("availability-observation-kernel-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fields struct {
		Cases []struct {
			Inputs   map[string]availabilityobservationkernel.FieldInput `json:"inputs"`
			Expected map[string]availabilityobservationkernel.Result     `json:"expected"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields.Cases) != 12 {
		t.Fatal(len(fields.Cases))
	}
	const entry = "gooo/tool-origin/explicit-observation:AssessObservationField"
	for n, c := range fields.Cases {
		got := availabilityobservationkernel.Evaluate(c.Inputs[entry])
		if !reflect.DeepEqual(got, c.Expected[entry]) {
			t.Fatalf("field %d: %+v != %+v", n, got, c.Expected[entry])
		}
	}
}

func explicitDocument(t *testing.T, raw []byte, before, after string) AvailabilityDocument {
	t.Helper()
	doc, err := AvailabilityTemplateJSON(raw, before, after)
	if err != nil {
		t.Fatal(err)
	}
	for i := range doc.Fields {
		if doc.Fields[i].ValueState != "" {
			t.Fatal("template inferred availability")
		}
		doc.Fields[i].ValueState = "SUPPLIED"
	}
	return doc
}

func explicitState(t *testing.T, doc *AvailabilityDocument, document, path, state string) {
	t.Helper()
	for i := range doc.Fields {
		if doc.Fields[i].SourceDocument == document && doc.Fields[i].FieldPath == path {
			doc.Fields[i].ValueState = state
			return
		}
	}
	t.Fatalf("fixture pointer not found: %s %s", document, path)
}

func compareExplicit(t *testing.T, raw []byte, doc AvailabilityDocument, before, after string) ExplicitObservationReport {
	t.Helper()
	r, err := CompareExplicitObservationJSON(raw, encode(t, doc), before, after)
	if err != nil {
		t.Fatal(err)
	}
	if r.Schema != "gooo/explicit-definition-observation-report/v1" || r.Decision.BehaviorEquivalenceProven || r.Decision.ExecutionAuthorized || r.Decision.LiveEffectPerformed || r.Decision.RemoteAtomicity != "UNKNOWN" {
		t.Fatal(r.Decision)
	}
	for _, d := range r.Observations {
		if d.ProducerAuthenticated || d.BehaviorEquivalenceProven || d.ExecutionAuthorized || d.LiveEffectPerformed {
			t.Fatal(d)
		}
	}
	return r
}

func diagnosticAt(t *testing.T, report ExplicitObservationReport, document, path string) ObservationDiagnostic {
	t.Helper()
	for _, d := range report.Observations {
		if d.SourceDocument == document && d.FieldPath == path {
			return d
		}
	}
	t.Fatalf("diagnostic absent: %s %s", document, path)
	return ObservationDiagnostic{}
}

func TestExplicitAvailabilityDistinguishesLiteralUnknownWithoutChangingRawOrV1(t *testing.T) {
	in := diagnosticFixture(t)
	in.Before.CatalogAuthorizationContextID = "UNKNOWN"
	in.After.CatalogAuthorizationContextID = "UNKNOWN"
	raw := encode(t, in)
	original := append([]byte(nil), raw...)
	v1, err := CompareDefinitionDiagnosticsJSON(raw, "p", "p")
	if err != nil || v1.Comparison.Decision.State != "BINDINGS_MATCH" {
		t.Fatal(err, v1.Comparison.Decision)
	}
	doc := explicitDocument(t, raw, "p", "p")
	supplied := compareExplicit(t, raw, doc, "p", "p")
	if supplied.Decision.State != "BINDINGS_MATCH" || diagnosticAt(t, supplied, "RECORDED_INPUT", "/after/catalog_authorization_context_id").State != "SAME_SUPPLIED" {
		t.Fatal(supplied.Decision)
	}
	for _, side := range []string{"before", "after"} {
		explicitState(t, &doc, "RECORDED_INPUT", "/"+side+"/catalog_authorization_context_id", "UNOBSERVED")
	}
	unobserved := compareExplicit(t, raw, doc, "p", "p")
	if unobserved.Decision.State != "UNASSESSED" || unobserved.Decision.CollectionState != "COLLECTION_CONTEXT_UNKNOWN" || unobserved.Decision.AuthorizationRelation != "UNKNOWN" {
		t.Fatal(unobserved.Decision)
	}
	for _, side := range []string{"before", "after"} {
		d := diagnosticAt(t, unobserved, "RECORDED_INPUT", "/"+side+"/catalog_authorization_context_id")
		if d.Value != "UNKNOWN" || d.ValueState != "UNOBSERVED" || d.State != "UNASSESSED" || d.KnownChange {
			t.Fatal(d)
		}
	}
	if supplied.Projection.InputSHA256 != unobserved.Projection.InputSHA256 || supplied.Projection.AvailabilitySHA256 == unobserved.Projection.AvailabilitySHA256 || supplied.Decision.ContextID == unobserved.Decision.ContextID || !bytes.Equal(original, raw) {
		t.Fatal("availability rebinding changed source or collapsed declaration identity")
	}
	again, err := CompareDefinitionDiagnosticsJSON(raw, "p", "p")
	if err != nil || !reflect.DeepEqual(v1, again) {
		t.Fatal("opt-in changed legacy result", err)
	}
}

func TestExplicitAvailabilityCannotTurnUnavailableDifferenceIntoKnownChange(t *testing.T) {
	in := diagnosticFixture(t)
	in.Before.CatalogAuthorizationContextID = "retained-placeholder"
	in.After.CatalogAuthorizationContextID = "real-other-supplied-id"
	raw := encode(t, in)
	doc := explicitDocument(t, raw, "UNKNOWN", "p")
	explicitState(t, &doc, "RECORDED_INPUT", "/before/catalog_authorization_context_id", "UNOBSERVED")
	explicitState(t, &doc, "SUPPLIED_POLICY", "/before_revision", "UNOBSERVED")
	r := compareExplicit(t, raw, doc, "UNKNOWN", "p")
	if r.Decision.State != "UNASSESSED" || r.Decision.AuthorizationRelation != "UNKNOWN" || r.Decision.PolicyRelation != "UNASSESSED" {
		t.Fatal(r.Decision)
	}
	for _, loc := range [][2]string{{"RECORDED_INPUT", "/after/catalog_authorization_context_id"}, {"SUPPLIED_POLICY", "/after_revision"}} {
		d := diagnosticAt(t, r, loc[0], loc[1])
		if d.KnownChange || d.State != "UNASSESSED" || d.ExpectedValueState != "UNOBSERVED" {
			t.Fatal(d)
		}
	}
}

func TestExplicitAvailabilityPreservesIndependentMismatchAndPageFailure(t *testing.T) {
	for _, mode := range []string{"backend-change", "unfinished-page", "undeclared-capture"} {
		t.Run(mode, func(t *testing.T) {
			in := diagnosticFixture(t)
			in.Before.CatalogAuthorizationContextID = "UNKNOWN"
			switch mode {
			case "backend-change":
				in.After.Captures[0].Pages[0].Context.BackendBindingRevision = "different-supplied-binding"
			case "unfinished-page":
				in.After.Captures[0].Pages[0].ResponseJSON = strings.Replace(in.After.Captures[0].Pages[0].ResponseJSON, `"result":{`, `"result":{"nextCursor":"",`, 1)
			case "undeclared-capture":
				in.After.DeclaredConnections = []Connection{}
			}
			raw := encode(t, in)
			doc := explicitDocument(t, raw, "p", "p")
			explicitState(t, &doc, "RECORDED_INPUT", "/before/catalog_authorization_context_id", "UNOBSERVED")
			r := compareExplicit(t, raw, doc, "p", "p")
			want := map[string]string{"backend-change": "UNSTABLE_CONFIGURATION", "unfinished-page": "INCOMPLETE_COLLECTION", "undeclared-capture": "INVALID_CONTEXT"}[mode]
			if r.Decision.CollectionState != want {
				t.Fatal(r.Decision)
			}
			if mode == "backend-change" {
				d := diagnosticAt(t, r, "RECORDED_INPUT", "/after/captures/0/pages/0/context/backend_binding_revision")
				if !d.KnownChange || d.State != "KNOWN_MISMATCH" {
					t.Fatal(d)
				}
			}
		})
	}
}

func TestExplicitAvailabilityOptionalTimestampAndPolicyRemainDistinct(t *testing.T) {
	raw := encode(t, diagnosticFixture(t))
	doc := explicitDocument(t, raw, "p", "p")
	for i := range doc.Fields {
		if strings.HasSuffix(doc.Fields[i].FieldPath, "/captured_at") {
			doc.Fields[i].ValueState = "UNOBSERVED"
		}
	}
	r := compareExplicit(t, raw, doc, "p", "p")
	if r.Decision.State != "BINDINGS_MATCH" || r.Decision.CollectionState != "RECORDED_CONTEXT_COMPLETE" {
		t.Fatal(r.Decision)
	}
	explicitState(t, &doc, "SUPPLIED_POLICY", "/after_revision", "UNOBSERVED")
	r = compareExplicit(t, raw, doc, "p", "p")
	if r.Decision.State != "UNASSESSED" || r.Decision.CollectionState != "RECORDED_CONTEXT_COMPLETE" || r.Decision.PolicyRelation != "UNASSESSED" {
		t.Fatal(r.Decision)
	}
}

func TestExplicitAvailabilityStrictRosterAndSourceBindingRefusals(t *testing.T) {
	raw := encode(t, diagnosticFixture(t))
	base := explicitDocument(t, raw, "p", "p")
	for _, mode := range []string{"input-hash", "policy-hash", "missing", "duplicate", "unknown-pointer", "wrong-document", "null-state", "number-state", "unknown-key", "duplicate-key", "extra-json", "wrong-schema"} {
		t.Run(mode, func(t *testing.T) {
			doc := base
			doc.Fields = append([]AvailabilityField(nil), base.Fields...)
			switch mode {
			case "input-hash":
				doc.InputSHA256 = digest([]byte("other"))
			case "policy-hash":
				doc.PolicySHA256 = digest([]byte("other"))
			case "missing":
				doc.Fields = doc.Fields[1:]
			case "duplicate":
				doc.Fields[1] = doc.Fields[0]
			case "unknown-pointer":
				doc.Fields[0].FieldPath = "/unknown"
			case "wrong-document":
				doc.Fields[0].SourceDocument = "SUPPLIED_POLICY"
			case "wrong-schema":
				doc.Schema = "unknown"
			}
			encoded := encode(t, doc)
			switch mode {
			case "null-state":
				encoded = bytes.Replace(encoded, []byte(`"value_state":"SUPPLIED"`), []byte(`"value_state":null`), 1)
			case "number-state":
				encoded = bytes.Replace(encoded, []byte(`"value_state":"SUPPLIED"`), []byte(`"value_state":1`), 1)
			case "unknown-key":
				encoded = append([]byte(`{"unexpected":true,`), encoded[1:]...)
			case "duplicate-key":
				encoded = append([]byte(`{"schema":"duplicate",`), encoded[1:]...)
			case "extra-json":
				encoded = append(encoded, []byte(` {}`)...)
			}
			r, err := CompareExplicitObservationJSON(raw, encoded, "p", "p")
			var problem *Problem
			if !errors.As(err, &problem) || problem.State != "INVALID_TRANSPORT" || r.Schema != "" {
				t.Fatal("must refuse without partial report", err, r.Schema)
			}
		})
	}
	if r, err := CompareExplicitObservationJSON(append(raw, ' '), encode(t, base), "p", "p"); err == nil || r.Schema != "" {
		t.Fatal("exact source whitespace change accepted")
	}
	if r, err := CompareExplicitObservationJSON(raw, encode(t, base), "changed-policy", "p"); err == nil || r.Schema != "" {
		t.Fatal("different policy arguments accepted")
	}
	if r, err := CompareExplicitObservationJSON(raw, bytes.Repeat([]byte(" "), MaxInputBytes), "p", "p"); err == nil || r.Schema != "" {
		t.Fatal("combined byte limit ignored")
	}
}

func TestExplicitAvailabilityRosterOrderDoesNotReassignFields(t *testing.T) {
	raw := encode(t, diagnosticFixture(t))
	doc := explicitDocument(t, raw, "p", "p")
	explicitState(t, &doc, "RECORDED_INPUT", "/before/catalog_authorization_context_id", "UNOBSERVED")
	a := compareExplicit(t, raw, doc, "p", "p")
	for left, right := 0, len(doc.Fields)-1; left < right; left, right = left+1, right-1 {
		doc.Fields[left], doc.Fields[right] = doc.Fields[right], doc.Fields[left]
	}
	b := compareExplicit(t, raw, doc, "p", "p")
	if a.Decision.State != b.Decision.State || a.Decision.AuthorizationRelation != b.Decision.AuthorizationRelation || a.Decision.CollectionState != b.Decision.CollectionState || a.Decision.ContextID == b.Decision.ContextID {
		t.Fatal("semantic mapping or exact sidecar binding lost")
	}
	for i := range a.Observations {
		x, y := a.Observations[i], b.Observations[i]
		x.ContextID, y.ContextID = "", ""
		if !reflect.DeepEqual(x, y) {
			t.Fatal("field order reassigned observation", x, y)
		}
	}
}

func TestExplicitAvailabilityInvalidDeclarationsAreDomainFailures(t *testing.T) {
	for _, state := range []string{"", "LEGACY_TEXT", "unknown"} {
		raw := encode(t, diagnosticFixture(t))
		doc := explicitDocument(t, raw, "p", "p")
		doc.Fields[0].ValueState = state
		r := compareExplicit(t, raw, doc, "p", "p")
		if r.Decision.CollectionState != "INVALID_CONTEXT" || diagnosticAt(t, r, "RECORDED_INPUT", "/before/catalog_authorization_context_id").State != "INVALID_DECLARATION" {
			t.Fatal(r.Decision)
		}
	}
	in := diagnosticFixture(t)
	in.Before.CatalogAuthorizationContextID = ""
	raw := encode(t, in)
	doc := explicitDocument(t, raw, "p", "p")
	r := compareExplicit(t, raw, doc, "p", "p")
	if r.Decision.CollectionState != "INVALID_CONTEXT" {
		t.Fatal(r.Decision)
	}
}

// The optional retained CE observation stays outside this public repository.
// Its hash is a fixed historical input identity, not a claim of capture-time context.
func TestExplicitAvailabilityRetainedCEBoundaryAndExport(t *testing.T) {
	path := os.Getenv("GOOO_AVAILABILITY_CE")
	if path == "" {
		t.Skip("retained local observation not supplied")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if digest(raw) != "sha256:3c3594278dc721804b0b46f8fc3bd3f6ff37a094e5b0bf0802978362ece4f7fb" {
		t.Fatal("unexpected retained observation")
	}
	doc := explicitDocument(t, raw, "", "")
	for i := range doc.Fields {
		// The retained source recorded protocol only; availability is the explicit
		// result of the historical recovery audit, not a production string heuristic.
		if !strings.HasSuffix(doc.Fields[i].FieldPath, "/protocol_version") {
			doc.Fields[i].ValueState = "UNOBSERVED"
		}
	}
	r := compareExplicit(t, raw, doc, "", "")
	if r.Decision.State != "UNASSESSED" || r.Decision.CollectionState != "COLLECTION_CONTEXT_UNKNOWN" || r.Decision.OriginState != "ORIGIN_UNOBSERVED" || r.Decision.NextAction != "OBTAIN_ORIGIN_OBSERVATION" || r.Decision.ContainsSyntheticFixture {
		t.Fatal(r.Decision)
	}
	missing, protocol, optional := 0, 0, 0
	for _, d := range r.Observations {
		if d.RequiredForContext && d.ValueState == "UNOBSERVED" {
			missing++
			if d.State != "UNASSESSED" || d.Value != "" {
				t.Fatal(d)
			}
		}
		if d.FieldKind == "PROTOCOL_VERSION" {
			protocol++
			if d.RecordingBasis != "RETROSPECTIVE" || d.Value != "2025-06-18" {
				t.Fatal(d)
			}
		}
		if !d.RequiredForContext {
			optional++
		}
	}
	if missing != 20 || protocol != 6 || optional != 8 || len(r.Observations) != 34 {
		t.Fatal(missing, protocol, optional)
	}
	v1, err := CompareDefinitionJSON(raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v1.Projection, r.Projection.SourceProjection) {
		t.Fatal("original source/rows/frame hashes changed")
	}
	exportAvailabilityRecords(t, "GOOO_AVAILABILITY_CE_EXPORT", []availabilityExport{{"retained-ce-missing20", string(raw), string(encode(t, doc)), "", "", r, "UNASSESSED", "COLLECTION_CONTEXT_UNKNOWN"}})
}

type availabilityExport struct {
	Label                   string                    `json:"label"`
	RawInputJSON            string                    `json:"raw_input_json"`
	AvailabilityJSON        string                    `json:"availability_json"`
	BeforePolicy            string                    `json:"before_policy_revision"`
	AfterPolicy             string                    `json:"after_policy_revision"`
	Report                  ExplicitObservationReport `json:"report"`
	ExpectedState           string                    `json:"expected_state"`
	ExpectedCollectionState string                    `json:"expected_collection_state"`
}

var backendAvailabilityLabels = []string{"backend-unobserved-same", "backend-unobserved-different", "backend-unobserved-asymmetric", "backend-unobserved-known-host", "backend-unobserved-known-connection", "backend-unobserved-known-name"}

func backendAvailabilityFixture(t *testing.T, mode string) ([]byte, AvailabilityDocument) {
	t.Helper()
	in := diagnosticFixture(t)
	for side, observation := range []*Observation{&in.Before, &in.After} {
		value := "UNKNOWN"
		if side == 1 && (mode == "backend-unobserved-different" || mode == "backend-unobserved-asymmetric") {
			value = "independent-retained-backend"
		}
		c := &observation.Captures[0]
		c.Begin.BackendBindingRevision, c.End.BackendBindingRevision, c.Pages[0].Context.BackendBindingRevision = value, value, value
	}
	switch mode {
	case "backend-unobserved-known-host":
		in.After.Captures[0].HostID = "independent-host"
		in.After.DeclaredConnections[0].HostID = "independent-host"
	case "backend-unobserved-known-connection":
		in.After.Captures[0].ConnectionID = "independent-connection"
		in.After.DeclaredConnections[0].ConnectionID = "independent-connection"
	case "backend-unobserved-known-name":
		in.After.Captures[0].Pages[0].ResponseJSON = strings.Replace(in.After.Captures[0].Pages[0].ResponseJSON, `"name":"search"`, `"name":"different_original_name"`, 1)
	}
	raw := encode(t, in)
	doc := explicitDocument(t, raw, "p", "p")
	explicitState(t, &doc, "RECORDED_INPUT", "/before/captures/0/pages/0/context/backend_binding_revision", "UNOBSERVED")
	if mode != "backend-unobserved-asymmetric" {
		explicitState(t, &doc, "RECORDED_INPUT", "/after/captures/0/pages/0/context/backend_binding_revision", "UNOBSERVED")
	}
	return raw, doc
}

func TestExplicitUnavailableBackendRestrictsOriginWithoutReplacingSavedSource(t *testing.T) {
	for _, mode := range backendAvailabilityLabels {
		t.Run(mode, func(t *testing.T) {
			raw, doc := backendAvailabilityFixture(t, mode)
			r := compareExplicit(t, raw, doc, "p", "p")
			legacy, err := CompareDefinitionJSON(raw, "p", "p")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r.Projection.SourceProjection, legacy.Projection) {
				t.Fatal("explicit availability changed original raw rows/frame hashes/origin tuple")
			}
			if strings.Contains(mode, "known-") {
				if r.Decision.OriginState != "ORIGIN_CHANGED" || r.Decision.State != "ORIGIN_NOT_UNCHANGED" {
					t.Fatal("independent known origin change lost", r.Decision)
				}
			} else {
				if r.Decision.OriginState != "ORIGIN_UNOBSERVED" || r.Decision.State != "UNASSESSED" || r.Decision.NextAction != "OBTAIN_ORIGIN_OBSERVATION" || r.Decision.DefinitionRelation != "UNASSESSED" {
					t.Fatal(r.Decision)
				}
				if mode == "backend-unobserved-same" && legacy.Decision.OriginState != "UNCHANGED" {
					t.Fatal("legacy same-value control changed")
				}
				if mode != "backend-unobserved-same" && legacy.Decision.OriginState != "ORIGIN_CHANGED" {
					t.Fatal("legacy different-value control changed")
				}
			}
			before := diagnosticAt(t, r, "RECORDED_INPUT", "/before/captures/0/pages/0/context/backend_binding_revision")
			if before.Value != "UNKNOWN" || before.ValueState != "UNOBSERVED" || before.KnownChange {
				t.Fatal(before)
			}
		})
	}
	// The spelling itself remains a legal supplied backend identity.
	raw, doc := backendAvailabilityFixture(t, "backend-unobserved-same")
	for i := range doc.Fields {
		doc.Fields[i].ValueState = "SUPPLIED"
	}
	r := compareExplicit(t, raw, doc, "p", "p")
	if r.Decision.OriginState != "UNCHANGED" || r.Decision.State != "BINDINGS_MATCH" {
		t.Fatal(r.Decision)
	}
}

func TestExplicitRowPageMappingRetainsSideCapturePageAndRepeatedRows(t *testing.T) {
	var in Input
	if err := json.Unmarshal(ExampleInputJSON(), &in); err != nil {
		t.Fatal(err)
	}
	for _, side := range []*Observation{&in.Before, &in.After} {
		for ci := range side.Captures {
			page := side.Captures[ci].Pages[0]
			page.ResponseJSON = `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"a"},{"name":"b"}]}}`
			page.DeclaredExposedNames = []string{"a", "b"}
			side.Captures[ci].Pages = []Page{page, page}
		}
	}
	raw := encode(t, in)
	doc := explicitDocument(t, raw, "p", "p")
	p, err := ProjectExplicitObservationInputsJSON(raw, encode(t, doc), "p", "p")
	if err != nil {
		t.Fatal(err)
	}
	a := p.Inputs[availabilityEntry+".input3"].(ExplicitAvailabilityInput)
	if !reflect.DeepEqual(a.BeforeRowPageIndexes, []int64{0, 0, 1, 1, 2, 2, 3, 3}) || !reflect.DeepEqual(a.AfterRowPageIndexes, []int64{4, 4, 5, 5, 6, 6, 7, 7}) {
		t.Fatal(a.BeforeRowPageIndexes, a.AfterRowPageIndexes)
	}
	for _, item := range []struct {
		rows      []DefinitionRow
		indexes   []int64
		firstPage int64
	}{{p.SourceProjection.BeforeRows, a.BeforeRowPageIndexes, 0}, {p.SourceProjection.AfterRows, a.AfterRowPageIndexes, 4}} {
		for n, row := range item.rows {
			if int64(row.Source.CaptureIndex*2+row.Source.PageIndex)+item.firstPage != item.indexes[n] {
				t.Fatal("row not bound to original page", row.Source)
			}
		}
	}
}

func TestExplicitUnavailableOriginKeepsKnownCollectionRecoveryFirst(t *testing.T) {
	for _, mode := range []string{"unfinished-cursor", "known-config-change", "undeclared-capture"} {
		t.Run(mode, func(t *testing.T) {
			raw, previous := backendAvailabilityFixture(t, "backend-unobserved-same")
			var in Input
			if err := json.Unmarshal(raw, &in); err != nil {
				t.Fatal(err)
			}
			state, action := "INCOMPLETE_COLLECTION", "RECAPTURE_DECLARED_SCOPE"
			switch mode {
			case "unfinished-cursor":
				in.After.Captures[0].Pages[0].ResponseJSON = strings.Replace(in.After.Captures[0].Pages[0].ResponseJSON, `"result":{`, `"result":{"nextCursor":"",`, 1)
			case "known-config-change":
				in.After.Captures[0].End.RelevantConfigurationRevision = "known-changed-configuration"
				state, action = "UNSTABLE_CONFIGURATION", "RECAPTURE_STABLE_CONTEXT"
			case "undeclared-capture":
				in.After.DeclaredConnections = []Connection{}
				state, action = "INVALID_CONTEXT", "FIX_CAPTURE_RECORD"
			}
			raw = encode(t, in)
			doc := explicitDocument(t, raw, "p", "p")
			for _, field := range previous.Fields {
				explicitState(t, &doc, field.SourceDocument, field.FieldPath, field.ValueState)
			}
			r := compareExplicit(t, raw, doc, "p", "p")
			if r.Decision.OriginState != "ORIGIN_UNOBSERVED" || r.Decision.State != "UNASSESSED" || r.Decision.CollectionState != state || r.Decision.NextAction != action || r.Decision.Reason != "COLLECTION_CONTEXT_NOT_ESTABLISHED" {
				t.Fatal(r.Decision)
			}
		})
	}
}

func exportAvailabilityRecords(t *testing.T, env string, records []availabilityExport) {
	t.Helper()
	path := os.Getenv(env)
	if path == "" {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil || path != abs {
		t.Fatal("absolute export path required")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(filepath.Dir(filepath.Dir(cwd)), abs)
	if err != nil || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatal("export must be outside checkout")
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

func TestExplicitAvailabilityConsumerExport(t *testing.T) {
	var records []availabilityExport
	for _, mode := range []string{"supplied-unknown", "unobserved-unknown", "unobserved-other-text", "asymmetric-unobserved", "known-page-change", "unobserved-policy", "optional-timestamps"} {
		in := diagnosticFixture(t)
		in.Before.CatalogAuthorizationContextID = "UNKNOWN"
		in.After.CatalogAuthorizationContextID = "UNKNOWN"
		if mode == "unobserved-other-text" {
			in.Before.CatalogAuthorizationContextID = "retained-unavailable-text"
		}
		if mode == "asymmetric-unobserved" {
			in.After.CatalogAuthorizationContextID = "different-supplied-id"
		}
		if mode == "known-page-change" {
			in.After.Captures[0].Pages[0].Context.RelevantConfigurationRevision = "different-supplied-configuration"
		}
		raw := encode(t, in)
		doc := explicitDocument(t, raw, "p", "p")
		state, collection := "BINDINGS_MATCH", "RECORDED_CONTEXT_COMPLETE"
		switch mode {
		case "unobserved-unknown", "unobserved-other-text", "asymmetric-unobserved", "known-page-change":
			explicitState(t, &doc, "RECORDED_INPUT", "/before/catalog_authorization_context_id", "UNOBSERVED")
			state, collection = "UNASSESSED", "COLLECTION_CONTEXT_UNKNOWN"
			if mode == "unobserved-unknown" {
				explicitState(t, &doc, "RECORDED_INPUT", "/after/catalog_authorization_context_id", "UNOBSERVED")
			}
			if mode == "known-page-change" {
				collection = "UNSTABLE_CONFIGURATION"
			}
		case "unobserved-policy":
			explicitState(t, &doc, "SUPPLIED_POLICY", "/after_revision", "UNOBSERVED")
			state = "UNASSESSED"
		case "optional-timestamps":
			for i := range doc.Fields {
				if strings.HasSuffix(doc.Fields[i].FieldPath, "/captured_at") {
					doc.Fields[i].ValueState = "UNOBSERVED"
				}
			}
		}
		r := compareExplicit(t, raw, doc, "p", "p")
		if r.Decision.State != state || r.Decision.CollectionState != collection {
			t.Fatal(mode, r.Decision)
		}
		records = append(records, availabilityExport{mode, string(raw), string(encode(t, doc)), "p", "p", r, state, collection})
	}
	for _, mode := range backendAvailabilityLabels {
		raw, doc := backendAvailabilityFixture(t, mode)
		r := compareExplicit(t, raw, doc, "p", "p")
		state := "UNASSESSED"
		if strings.Contains(mode, "known-") {
			state = "ORIGIN_NOT_UNCHANGED"
		}
		if r.Decision.State != state || r.Decision.CollectionState != "COLLECTION_CONTEXT_UNKNOWN" {
			t.Fatal(mode, r.Decision)
		}
		records = append(records, availabilityExport{mode, string(raw), string(encode(t, doc)), "p", "p", r, state, "COLLECTION_CONTEXT_UNKNOWN"})
	}
	exportAvailabilityRecords(t, "GOOO_AVAILABILITY_EXPORT", records)
}
