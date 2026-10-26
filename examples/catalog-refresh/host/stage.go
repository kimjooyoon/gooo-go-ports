// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package catalogrefresh

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	recordedcatalog "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

// Candidate has no exported mutable state. The active registry never points at
// collector-owned input. Envelope binds the exact page strings, projected facts,
// shape outcomes and captured transaction tuple before its digest is assigned.
type Candidate struct {
	attempt  *Attempt
	id       string
	catalog  []byte
	envelope []byte
	facts    SnapshotFacts
	evidence RefreshEvidence
}

func (c *Candidate) ID() string       { return c.id }
func (c *Candidate) Bytes() []byte    { return bytes.Clone(c.catalog) }
func (c *Candidate) Envelope() []byte { return bytes.Clone(c.envelope) }
func (c *Candidate) Inputs() (SnapshotFacts, RefreshEvidence) {
	return cloneFacts(c.facts), cloneEvidence(c.evidence)
}

// Stage handles already obtained JSON-RPC pages. It neither collects live data
// nor invents remote/capture identity. The collector state is an explicit host
// observation and is checked by Gooo, including when the pages are empty.
func (a *Attempt) Stage(pages []SnapshotPage, collectorState string) (*Candidate, error) {
	if a == nil || a.registry == nil {
		return nil, errors.New("invalid attempt")
	}
	if len(pages) > recordedcatalog.MaxPages {
		return nil, errors.New("page limit exceeded")
	}
	if len(collectorState) > recordedcatalog.MaxTextBytes {
		return nil, errors.New("collector state exceeds text limit")
	}
	for _, page := range pages {
		// Bound raw bytes before either scanning UTF-8 or allocating the outer
		// JSON envelope. At most MaxPages pairs of bounded frames can proceed.
		if len(page.RequestJSON) > recordedcatalog.MaxFrameBytes || len(page.ResponseJSON) > recordedcatalog.MaxFrameBytes {
			return nil, errors.New("saved frame exceeds byte limit")
		}
		// Check before nesting the page strings in JSON: json.Marshal would
		// otherwise replace invalid UTF-8 with U+FFFD and lose exact bytes.
		if !utf8.ValidString(page.RequestJSON) || !utf8.ValidString(page.ResponseJSON) {
			return nil, errors.New("saved frame contains invalid UTF-8")
		}
	}
	// Append to a nonnil empty slice so no pages stays [] rather than null.
	saved := append([]SnapshotPage{}, pages...)
	input := recordedcatalog.DefinitionSnapshotInput{
		TargetName: "", Before: recordedcatalog.SnapshotObservation{Pages: []SnapshotPage{}},
		After: recordedcatalog.SnapshotObservation{Pages: saved},
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	projection, err := recordedcatalog.ProjectDefinitionSnapshotInputsJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("project saved candidate: %w", err)
	}
	states, err := shapeStates(saved, projection.After.Fields)
	if err != nil {
		return nil, err
	}
	evidence := RefreshEvidence{
		AttemptID: a.id, CollectorState: collectorState, BindingState: "BOUND",
		ValidatorProfile:    ValidatorProfile,
		RowDefinitionIDs:    append([]string{}, projection.After.Fields.DefinitionIDs...),
		RowValidationStates: states,
	}
	// Stored bytes are the exact page-string bundle, not the candidate envelope:
	// refreshing identical data still receives a fresh version, without changing
	// the catalog merely because an attempt ID changed.
	catalog, err := json.Marshal(recordedcatalog.SnapshotObservation{Pages: saved})
	if err != nil {
		return nil, err
	}
	envelope, err := json.Marshal(struct {
		Schema    string          `json:"schema"`
		AttemptID string          `json:"attempt_id"`
		Expected  RefreshState    `json:"captured_state"`
		Pages     []SnapshotPage  `json:"pages"`
		Facts     SnapshotFacts   `json:"facts"`
		Evidence  RefreshEvidence `json:"evidence_before_candidate_digest"`
	}{"gooo/local-catalog-candidate/v1", a.id, stateBetween(a.expected, a.expected), saved, projection.After.Fields, evidence})
	if err != nil {
		return nil, err
	}
	evidence.CandidateID = digest(envelope)
	return &Candidate{attempt: a, id: evidence.CandidateID, catalog: catalog,
		envelope: envelope, facts: cloneFacts(projection.After.Fields), evidence: evidence}, nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// shapeStates re-reads exact protocol keys after the strict saved-page parser
// has rejected duplicate keys, malformed UTF-8/escapes and oversized frames.
// Case variants such as Tools/InputSchema remain different opaque members.
// This profile is deliberately only a descriptor shape observation: it does not
// validate JSON Schema keywords, references, satisfiability or runtime outputs.
func shapeStates(pages []SnapshotPage, facts SnapshotFacts) ([]string, error) {
	states := []string{}
	for pi, page := range pages {
		var envelope map[string]json.RawMessage
		var result map[string]json.RawMessage
		var tools []map[string]json.RawMessage
		if json.Unmarshal([]byte(page.ResponseJSON), &envelope) != nil ||
			json.Unmarshal(envelope["result"], &result) != nil ||
			json.Unmarshal(result["tools"], &tools) != nil {
			return nil, errors.New("shape projection lost exact protocol binding")
		}
		for _, tool := range tools {
			i := len(states)
			var name string
			if i >= len(facts.OriginalNames) || json.Unmarshal(tool["name"], &name) != nil ||
				name != facts.OriginalNames[i] || facts.RowPageIndexes[i] != int64(pi) {
				return nil, errors.New("shape projection lost row binding")
			}
			state := "REJECTED"
			if descriptorShape(tool) {
				state = "ACCEPTED"
			}
			states = append(states, state)
		}
	}
	if len(states) != len(facts.DefinitionIDs) {
		return nil, errors.New("shape projection lost row count")
	}
	return states, nil
}

// Descriptor shape v1: nonempty bounded name; required inputSchema object with
// exact type="object"; optional outputSchema under that same restriction;
// optional bounded title/description strings; optional annotations object whose
// known hints are booleans and whose optional title is a bounded string. Unknown
// members are retained in descriptor IDs but are not interpreted by this profile.
func descriptorShape(tool map[string]json.RawMessage) bool {
	if !boundedString(tool["name"], true) || !objectSchema(tool["inputSchema"]) {
		return false
	}
	for _, key := range []string{"title", "description"} {
		if value, ok := tool[key]; ok && !boundedString(value, false) {
			return false
		}
	}
	if value, ok := tool["outputSchema"]; ok && !objectSchema(value) {
		return false
	}
	if value, ok := tool["annotations"]; ok {
		var annotations map[string]json.RawMessage
		if json.Unmarshal(value, &annotations) != nil || annotations == nil {
			return false
		}
		for _, key := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			if hint, ok := annotations[key]; ok && !bytes.Equal(hint, []byte("true")) && !bytes.Equal(hint, []byte("false")) {
				return false
			}
		}
		if title, ok := annotations["title"]; ok && !boundedString(title, false) {
			return false
		}
	}
	return true
}

func objectSchema(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	var kind string
	return json.Unmarshal(raw, &object) == nil && object != nil &&
		json.Unmarshal(object["type"], &kind) == nil && kind == "object"
}

func boundedString(raw json.RawMessage, required bool) bool {
	var value string
	return len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, &value) == nil &&
		len(value) <= recordedcatalog.MaxTextBytes && (!required || value != "")
}

func cloneFacts(f SnapshotFacts) SnapshotFacts {
	f.PageRequestCursorKinds = append([]string{}, f.PageRequestCursorKinds...)
	f.PageRequestCursorValues = append([]string{}, f.PageRequestCursorValues...)
	f.PageNextCursorKinds = append([]string{}, f.PageNextCursorKinds...)
	f.PageNextCursorValues = append([]string{}, f.PageNextCursorValues...)
	f.OriginalNames = append([]string{}, f.OriginalNames...)
	f.DefinitionIDs = append([]string{}, f.DefinitionIDs...)
	f.DefinitionProfiles = append([]string{}, f.DefinitionProfiles...)
	f.AnnotationIDs = append([]string{}, f.AnnotationIDs...)
	f.RowPageIndexes = append([]int64{}, f.RowPageIndexes...)
	return f
}

func cloneEvidence(e RefreshEvidence) RefreshEvidence {
	e.RowDefinitionIDs = append([]string{}, e.RowDefinitionIDs...)
	e.RowValidationStates = append([]string{}, e.RowValidationStates...)
	return e
}
