// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"unicode/utf8"
)

func digest(raw []byte) string {
	s := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(s[:])
}

// Go's JSON decoder replaces unpaired surrogate escapes. Check the exact string
// tokens before using it. Algorithm adapted from alias-origin-change's bounded
// transport helper; source and license attribution are in NOTICE.md.
func validJSONString(raw []byte) bool {
	var value string
	if len(raw) < 2 || raw[0] != '"' || !utf8.Valid(raw) || json.Unmarshal(raw, &value) != nil {
		return false
	}
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			continue
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		v, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
		if err != nil {
			return false
		}
		if v >= 0xd800 && v <= 0xdbff {
			if i+12 > len(raw)-1 || raw[i+6] != '\\' || raw[i+7] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+8:i+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 11
		} else if v >= 0xdc00 && v <= 0xdfff {
			return false
		} else {
			i += 5
		}
	}
	return true
}

func strictJSON(raw []byte, limit int) error {
	if len(raw) > limit || !utf8.Valid(raw) {
		return invalid("JSON_SIZE_OR_ENCODING")
	}
	// Inspect all raw string tokens, including keys in opaque descriptor fields.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		start := i
		i++
		for ; i < len(raw); i++ {
			if raw[i] == '\\' {
				i++
				continue
			}
			if raw[i] == '"' {
				break
			}
		}
		if i >= len(raw) || !validJSONString(raw[start:i+1]) {
			return invalid("NONSCALAR_JSON_STRING")
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 64 {
			return invalid("JSON_DEPTH_LIMIT")
		}
		t, err := d.Token()
		if err != nil {
			return invalid("INVALID_JSON")
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return invalid("DUPLICATE_OR_INVALID_JSON_KEY")
				}
				seen[name] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
			if end, err := d.Token(); err != nil || end != json.Delim('}') {
				return invalid("INVALID_JSON")
			}
		case json.Delim('['):
			for d.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
			if end, err := d.Token(); err != nil || end != json.Delim(']') {
				return invalid("INVALID_JSON")
			}
		default:
			if _, ok := t.(json.Delim); ok {
				return invalid("INVALID_JSON")
			}
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return invalid("TRAILING_JSON")
	}
	return nil
}

// Every outer schema field is required. In particular [] is not repaired from
// null, omitted metadata is not invented, and unknown decision fields are refused.
func exactShape(raw json.RawMessage, t reflect.Type) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return invalid("MISSING_FIELD")
	}
	switch t.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if raw[0] != '{' || json.Unmarshal(raw, &obj) != nil || len(obj) != t.NumField() {
			return invalid("EXACT_OBJECT_REQUIRED")
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			v, ok := obj[f.Tag.Get("json")]
			if !ok {
				return invalid("MISSING_FIELD")
			}
			if err := exactShape(v, f.Type); err != nil {
				return err
			}
		}
	case reflect.Slice:
		var list []json.RawMessage
		if raw[0] != '[' || json.Unmarshal(raw, &list) != nil {
			return invalid("ARRAY_REQUIRED")
		}
		for _, v := range list {
			if err := exactShape(v, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.String:
		if !validJSONString(raw) {
			return invalid("STRING_REQUIRED")
		}
	case reflect.Bool:
		if !bytes.Equal(raw, []byte("true")) && !bytes.Equal(raw, []byte("false")) {
			return invalid("BOOLEAN_REQUIRED")
		}
	default:
		return invalid("UNSUPPORTED_OUTER_TYPE")
	}
	return nil
}

func decodeFacts(raw []byte) (Input, error) {
	var input Input
	if err := strictJSON(raw, MaxInputBytes); err != nil {
		return input, err
	}
	if err := exactShape(raw, reflect.TypeFor[Input]()); err != nil {
		return input, err
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return Input{}, invalid("INVALID_FACTS")
	}
	checkText := func(v string) error {
		if len(v) > MaxTextBytes {
			return invalid("TEXT_LIMIT")
		}
		return nil
	}
	checkContext := func(c Context) error {
		for _, s := range []string{c.CapturedAt, c.BackendBindingRevision, c.AuthorizationContextID, c.RelevantConfigurationRevision, c.ProtocolVersion} {
			if err := checkText(s); err != nil {
				return err
			}
		}
		return nil
	}
	if err := checkText(input.TargetAlias); err != nil {
		return Input{}, err
	}
	for _, o := range []Observation{input.Before, input.After} {
		if len(o.DeclaredConnections) > MaxConnections || len(o.Captures) > MaxConnections {
			return Input{}, invalid("CONNECTION_LIMIT")
		}
		if err := checkText(o.CatalogAuthorizationContextID); err != nil {
			return Input{}, err
		}
		for _, c := range o.DeclaredConnections {
			for _, s := range []string{c.HostID, c.ConnectionID} {
				if err := checkText(s); err != nil {
					return Input{}, err
				}
			}
		}
		for _, c := range o.Captures {
			for _, s := range []string{c.HostID, c.ConnectionID, c.ServerNameClaim} {
				if err := checkText(s); err != nil {
					return Input{}, err
				}
			}
			if c.ContextRecordingBasis != "CAPTURE_TIME" && c.ContextRecordingBasis != "RETROSPECTIVE" && c.ContextRecordingBasis != "UNKNOWN" {
				return Input{}, invalid("UNKNOWN_CONTEXT_BASIS")
			}
			if c.EvidenceKind != "SYNTHETIC_FIXTURE" && c.EvidenceKind != "RECORDED_OBSERVATION" {
				return Input{}, invalid("UNKNOWN_EVIDENCE_KIND")
			}
			if c.ConfigurationWatch.Coverage != "CONTINUOUS" && c.ConfigurationWatch.Coverage != "ENDPOINTS_ONLY" && c.ConfigurationWatch.Coverage != "UNKNOWN" {
				return Input{}, invalid("UNKNOWN_WATCH_COVERAGE")
			}
			if c.RemoteAtomicity != "UNKNOWN" {
				return Input{}, invalid("UNSUPPORTED_ATOMICITY_CLAIM")
			}
			if len(c.Pages) > MaxPages {
				return Input{}, invalid("PAGE_LIMIT")
			}
			for _, ctx := range []Context{c.Begin, c.End} {
				if err := checkContext(ctx); err != nil {
					return Input{}, err
				}
			}
			for _, p := range c.Pages {
				if err := checkContext(p.Context); err != nil {
					return Input{}, err
				}
				if len(p.RequestJSON) > MaxFrameBytes || len(p.ResponseJSON) > MaxFrameBytes {
					return Input{}, invalid("FRAME_LIMIT")
				}
				if len(p.DeclaredExposedNames) > MaxEntries {
					return Input{}, invalid("ROW_LIMIT")
				}
				for _, s := range p.DeclaredExposedNames {
					if err := checkText(s); err != nil {
						return Input{}, err
					}
				}
			}
		}
	}
	return input, nil
}

func jsonObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	var obj map[string]json.RawMessage
	if len(raw) == 0 || raw[0] != '{' || json.Unmarshal(raw, &obj) != nil {
		return nil, invalid("RPC_OBJECT_REQUIRED")
	}
	return obj, nil
}
func scalarText(raw json.RawMessage) (string, error) {
	if !validJSONString(bytes.TrimSpace(raw)) {
		return "", invalid("RPC_STRING_REQUIRED")
	}
	var text string
	_ = json.Unmarshal(raw, &text)
	if len(text) > MaxTextBytes {
		return "", invalid("TEXT_LIMIT")
	}
	return text, nil
}

var integerID = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

func typedID(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '"' {
		s, e := scalarText(raw)
		return "string:" + s, e
	}
	if !integerID.Match(raw) {
		return "", invalid("UNSUPPORTED_RPC_ID")
	}
	n, e := strconv.ParseInt(string(raw), 10, 64)
	if e != nil {
		return "", invalid("UNSUPPORTED_RPC_ID")
	}
	return "integer:" + strconv.FormatInt(n, 10), nil
}
func parseCursor(raw json.RawMessage, present, allowNull bool) (Cursor, error) {
	if !present {
		return Cursor{Tag: "ABSENT"}, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if allowNull {
			return Cursor{Tag: "NULL"}, nil
		}
		return Cursor{}, invalid("NULL_REQUEST_CURSOR")
	}
	s, err := scalarText(raw)
	return Cursor{Tag: "STRING", Value: s}, err
}

type decodedPage struct {
	Names  []string
	Source PageSource
}

func decodePage(page Page) (decodedPage, error) {
	return decodeSavedPage(page.RequestJSON, page.ResponseJSON, len(page.DeclaredExposedNames), true)
}

// decodeSavedPage validates saved JSON-RPC bytes without asserting a host
// identity. The existing host path still checks exposure count at the original
// position, before decoding individual tool names.
func decodeSavedPage(requestJSON, responseJSON string, exposedCount int, requireExposure bool) (decodedPage, error) {
	var result decodedPage
	if err := strictJSON([]byte(requestJSON), MaxFrameBytes); err != nil {
		return result, err
	}
	if err := strictJSON([]byte(responseJSON), MaxFrameBytes); err != nil {
		return result, err
	}
	req, err := jsonObject([]byte(requestJSON))
	if err != nil {
		return result, err
	}
	res, err := jsonObject([]byte(responseJSON))
	if err != nil {
		return result, err
	}
	for _, o := range []map[string]json.RawMessage{req, res} {
		v, e := scalarText(o["jsonrpc"])
		if e != nil || v != "2.0" {
			return result, invalid("RPC_VERSION")
		}
	}
	method, e := scalarText(req["method"])
	if e != nil || method != "tools/list" {
		return result, invalid("RPC_METHOD")
	}
	if _, present := req["result"]; present {
		return result, invalid("INVALID_REQUEST_ENVELOPE")
	}
	if _, present := req["error"]; present {
		return result, invalid("INVALID_REQUEST_ENVELOPE")
	}
	rid, e := typedID(req["id"])
	if e != nil {
		return result, e
	}
	sid, e := typedID(res["id"])
	if e != nil {
		return result, e
	}
	if rid != sid {
		return result, invalid("RPC_ID_MISMATCH")
	}
	if _, present := res["method"]; present {
		return result, invalid("INVALID_RESPONSE_ENVELOPE")
	}
	if raw, present := res["error"]; present {
		if _, hasResult := res["result"]; hasResult {
			return result, invalid("RESULT_AND_ERROR")
		}
		errObj, e := jsonObject(raw)
		if e != nil {
			return result, invalid("INVALID_RPC_ERROR")
		}
		code := bytes.TrimSpace(errObj["code"])
		if !integerID.Match(code) {
			return result, invalid("INVALID_RPC_ERROR")
		}
		if _, e := strconv.ParseInt(string(code), 10, 64); e != nil {
			return result, invalid("INVALID_RPC_ERROR")
		}
		// Error text is opaque diagnostic data, bounded by the complete frame.
		// Applying the origin/context identifier limit would erase long messages.
		if !validJSONString(bytes.TrimSpace(errObj["message"])) {
			return result, invalid("INVALID_RPC_ERROR")
		}
		return result, &Problem{State: "UNKNOWN", Stage: "saved_response", Code: "SAVED_RPC_ERROR", ResponseJSON: responseJSON}
	}
	cursor := Cursor{Tag: "ABSENT"}
	if raw, present := req["params"]; present {
		params, e := jsonObject(raw)
		if e != nil {
			return result, e
		}
		raw, ok := params["cursor"]
		cursor, e = parseCursor(raw, ok, false)
		if e != nil {
			return result, e
		}
	}
	body, e := jsonObject(res["result"])
	if e != nil {
		return result, e
	}
	if raw, present := body["resultType"]; present {
		value, e := scalarText(raw)
		if e != nil || value != "complete" {
			return result, &Problem{State: "INVALID_TRANSPORT", Stage: "saved_response", Code: "UNSUPPORTED_RESULT_TYPE", ResponseJSON: responseJSON}
		}
	}
	var tools []json.RawMessage
	raw, ok := body["tools"]
	raw = bytes.TrimSpace(raw)
	if !ok || len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &tools) != nil {
		return result, invalid("TOOLS_ARRAY_REQUIRED")
	}
	if len(tools) > MaxEntries {
		return result, invalid("ROW_LIMIT")
	}
	if requireExposure && len(tools) != exposedCount {
		return result, invalid("EXPOSURE_COUNT_MISMATCH")
	}
	result.Names = make([]string, len(tools))
	for i, raw := range tools {
		tool, e := jsonObject(raw)
		if e != nil {
			return result, e
		}
		result.Names[i], e = scalarText(tool["name"])
		if e != nil {
			return result, e
		}
	}
	raw, present := body["nextCursor"]
	next, e := parseCursor(raw, present, true)
	if e != nil {
		return result, e
	}
	result.Source = PageSource{RequestSHA256: digest([]byte(requestJSON)), ResponseSHA256: digest([]byte(responseJSON)), RequestID: append(json.RawMessage(nil), req["id"]...), ResponseID: append(json.RawMessage(nil), res["id"]...), RequestCursor: cursor, NextCursor: next}
	return result, nil
}

type projectedObservation struct {
	Entries []Entry
	Pages   []PageSource
	Rows    []RowSource
	Decoded [][]decodedPage
}

func projectObservation(side string, observation Observation) (projectedObservation, error) {
	result := projectedObservation{Entries: []Entry{}, Pages: []PageSource{}, Rows: []RowSource{}, Decoded: make([][]decodedPage, len(observation.Captures))}
	for ci, capture := range observation.Captures {
		result.Decoded[ci] = make([]decodedPage, len(capture.Pages))
		for pi, page := range capture.Pages {
			decoded, err := decodePage(page)
			if err != nil {
				if problem, ok := err.(*Problem); ok {
					problem.Side = side
					problem.CaptureIndex = &ci
					problem.PageIndex = &pi
					problem.RequestSHA256 = digest([]byte(page.RequestJSON))
					problem.ResponseSHA256 = digest([]byte(page.ResponseJSON))
				}
				return projectedObservation{}, err
			}
			decoded.Source.CaptureIndex = ci
			decoded.Source.PageIndex = pi
			result.Decoded[ci][pi] = decoded
			result.Pages = append(result.Pages, decoded.Source)
			if len(result.Entries)+len(decoded.Names) > MaxEntries {
				problem := invalid("ROW_LIMIT")
				problem.Side = side
				problem.CaptureIndex = &ci
				problem.PageIndex = &pi
				problem.RequestSHA256 = decoded.Source.RequestSHA256
				problem.ResponseSHA256 = decoded.Source.ResponseSHA256
				return projectedObservation{}, problem
			}
			tuple := [3]string{capture.HostID, capture.ConnectionID, page.Context.BackendBindingRevision}
			encoded, err := json.Marshal(tuple)
			if err != nil {
				return projectedObservation{}, fmt.Errorf("encode origin: %w", err)
			}
			origin := "gooo/host-origin/v1/" + digest(encoded)
			for ti, name := range decoded.Names {
				result.Entries = append(result.Entries, Entry{Server: origin, OriginalName: name, ExposedName: page.DeclaredExposedNames[ti]})
				result.Rows = append(result.Rows, RowSource{CaptureIndex: ci, PageIndex: pi, ToolIndex: ti, ResponseSHA256: decoded.Source.ResponseSHA256, OriginTuple: tuple, OriginID: origin})
			}
		}
	}
	return result, nil
}
