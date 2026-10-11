package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

const compilerRevision = "83928fa4538245abe33b08549da47f8cbdc50ed2"
const entityPrefix = "gooo://decision-policy-pilot/"
const activityID = entityPrefix + "admit"
const entry = "pilot/policy:Admit"

type Request struct {
	Action    string `json:"action"`
	Tenant    string `json:"tenant"`
	ItemCount int64  `json:"item_count"`
}
type Context struct {
	Authenticated bool   `json:"authenticated"`
	Tenant        string `json:"tenant"`
	AllowedAction string `json:"allowed_action"`
	ItemLimit     int64  `json:"item_limit"`
	Now           int64  `json:"now"`
	ExpiresAt     int64  `json:"expires_at"`
}
type Choice struct {
	Status string `json:"status"`
	Choice string `json:"choice"`
}
type Envelope struct {
	Request Request `json:"request"`
	Context Context `json:"context"`
	Choice  Choice  `json:"decision"`
}
type Outcome struct {
	Allowed     bool   `json:"allowed"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
}
type Expected struct {
	Outcome
	SpyCalls int `json:"spy_calls"`
}
type Fixture struct {
	ID       string          `json:"id"`
	Input    json.RawMessage `json:"input,omitempty"`
	RawInput string          `json:"raw_input,omitempty"`
	Expected Expected        `json:"expected"`
}

func (f Fixture) raw() []byte {
	if f.RawInput != "" {
		return []byte(f.RawInput)
	}
	return f.Input
}

// Reject ambiguous JSON recursively before typed decoding. Input is bounded;
// duplicate keys, trailing values, non-UTF8 bytes and excessive depth are errors.
func strictJSON(raw []byte) error { return checkJSON(raw, 16384, 8) }

func checkJSON(raw []byte, maxBytes, maxDepth int) error {
	if len(raw) == 0 || len(raw) > maxBytes || !utf8.Valid(raw) {
		return fmt.Errorf("JSON must be 1..%d UTF-8 bytes", maxBytes)
	}
	if err := validSurrogates(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var read func(int) error
	read = func(depth int) error {
		if depth > maxDepth {
			return fmt.Errorf("input nesting too deep")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate or invalid key %v", key)
				}
				seen[name] = true
				if err := read(depth + 1); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return fmt.Errorf("invalid object")
			}
		case json.Delim('['):
			for d.More() {
				if err := read(depth + 1); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return fmt.Errorf("invalid array")
			}
		}
		return nil
	}
	if err := read(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON content")
	}
	return nil
}

func object(raw []byte, names []string) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, fmt.Errorf("explicit object required")
	}
	if len(m) != len(names) {
		return nil, fmt.Errorf("exact fields required: %v", names)
	}
	for _, name := range names {
		if value, ok := m[name]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("field %s is required and non-null", name)
		}
	}
	return m, nil
}

func decodeEnvelope(raw []byte) (Envelope, error) {
	var e Envelope
	if err := strictJSON(raw); err != nil {
		return e, err
	}
	m, err := object(raw, []string{"request", "context", "decision"})
	if err != nil {
		return e, err
	}
	parts := []struct {
		key   string
		names []string
		value any
	}{
		{"request", []string{"action", "tenant", "item_count"}, &e.Request},
		{"context", []string{"authenticated", "tenant", "allowed_action", "item_limit", "now", "expires_at"}, &e.Context},
		{"decision", []string{"status", "choice"}, &e.Choice},
	}
	for _, p := range parts {
		if _, err := object(m[p.key], p.names); err != nil {
			return e, fmt.Errorf("%s: %w", p.key, err)
		}
		if err := json.Unmarshal(m[p.key], p.value); err != nil {
			return e, fmt.Errorf("%s: %w", p.key, err)
		}
	}
	for _, value := range []string{e.Request.Action, e.Request.Tenant, e.Context.Tenant, e.Context.AllowedAction, e.Choice.Status, e.Choice.Choice} {
		if len(value) > 1024 {
			return e, fmt.Errorf("text exceeds Gooo's 1024-byte boundary")
		}
	}
	return e, nil
}

// Independent reference implementation. This executes no Gooo/generated code and
// reads no generated output; the manually specified fixture expectations are a
// separate check against both implementations.
func reference(e Envelope) Outcome {
	r, c, d := e.Request, e.Context, e.Choice
	deny := func(reason string) Outcome { return Outcome{false, "deny", reason} }
	switch {
	case !c.Authenticated:
		return deny("unauthenticated")
	case r.Tenant == "" || c.Tenant == "" || r.Tenant != c.Tenant:
		return deny("tenant_mismatch")
	case r.Action != "export_records" || r.Action != c.AllowedAction:
		return deny("action_not_allowed")
	case r.ItemCount < 1 || c.ItemLimit < 1:
		return deny("invalid_limit")
	case c.ItemLimit < r.ItemCount:
		return deny("limit_exceeded")
	case c.Now >= c.ExpiresAt:
		return deny("expired")
	case d.Status != "observed":
		return Outcome{false, "review", "provider_unavailable"}
	}
	switch d.Choice {
	case "allow":
		return Outcome{true, "allow", "policy_satisfied"}
	case "review":
		return Outcome{false, "review", "model_review"}
	case "deny":
		return deny("model_denied")
	default:
		return deny("invalid_choice")
	}
}

// spy deliberately has no export/network/filesystem operation. Only a fully
// verified current Gooo result is passed here; expected values never dispatch.
type spy struct{ calls int }

func dispatch(actual Outcome, verificationError error, s *spy) error {
	if verificationError != nil {
		return verificationError
	}
	valid := map[string]bool{"allow": true, "deny": true, "review": true}
	if !valid[actual.Disposition] || actual.Allowed != (actual.Disposition == "allow") || actual.Reason == "" {
		return fmt.Errorf("inconsistent policy outcome")
	}
	if actual.Allowed {
		s.calls++
	}
	return nil
}

// encoding/json replaces isolated UTF-16 surrogate escapes with U+FFFD. Reject
// those escapes before decoding so distinct tenant inputs cannot become aliases.
func validSurrogates(raw []byte) error {
	quoted := false
	hex4 := func(start int) (uint64, error) {
		if start+4 > len(raw) {
			return 0, fmt.Errorf("incomplete Unicode escape")
		}
		return strconv.ParseUint(string(raw[start:start+4]), 16, 16)
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return fmt.Errorf("incomplete escape")
		}
		if raw[i] != 'u' {
			continue
		}
		v, err := hex4(i + 1)
		if err != nil {
			return err
		}
		i += 4
		if v >= 0xDC00 && v <= 0xDFFF {
			return fmt.Errorf("isolated low surrogate")
		}
		if v >= 0xD800 && v <= 0xDBFF {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return fmt.Errorf("unpaired high surrogate")
			}
			low, err := hex4(i + 3)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return fmt.Errorf("unpaired high surrogate")
			}
			i += 6
		}
	}
	return nil
}
