package recordedcatalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func fixtureContext() Context {
	return Context{CapturedAt: "synthetic-time", BackendBindingRevision: "backend-r1", AuthorizationContextID: "auth-a", RelevantConfigurationRevision: "config-r1", ProtocolVersion: "2026-07-28"}
}
func fixturePage(names, aliases []string, requestCursor, nextCursor *string) Page {
	request := map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}
	if requestCursor != nil {
		request["params"] = map[string]any{"cursor": *requestCursor}
	}
	tools := []map[string]any{}
	for _, name := range names {
		tools = append(tools, map[string]any{"name": name, "description": "synthetic fixture", "inputSchema": map[string]any{"type": "object"}})
	}
	result := map[string]any{"tools": tools, "resultType": "complete"}
	if nextCursor != nil {
		result["nextCursor"] = *nextCursor
	}
	req, _ := json.Marshal(request)
	res, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "result": result})
	return Page{Context: fixtureContext(), RequestJSON: string(req), ResponseJSON: string(res), DeclaredExposedNames: aliases}
}
func fixtureCapture(connection, alias string) Capture {
	return Capture{HostID: "host", ConnectionID: connection, ServerNameClaim: "docs", ContextRecordingBasis: "CAPTURE_TIME", EvidenceKind: "SYNTHETIC_FIXTURE", Begin: fixtureContext(), End: fixtureContext(), ConfigurationWatch: Watch{Coverage: "CONTINUOUS"}, RemoteAtomicity: "UNKNOWN", Pages: []Page{fixturePage([]string{"search"}, []string{alias}, nil, nil)}}
}
func fixtureInput() Input {
	side := func() Observation {
		return Observation{CatalogAuthorizationContextID: "catalog-auth", DeclaredConnections: []Connection{{HostID: "host", ConnectionID: "a"}}, Captures: []Capture{fixtureCapture("a", "search")}}
	}
	return Input{TargetAlias: "search", Before: side(), After: side()}
}
func encode(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSavedFramePreservation(t *testing.T) {
	empty := ""
	page := fixturePage([]string{"sérver_검색"}, []string{"alias"}, &empty, &empty)
	page.RequestJSON = " \n" + page.RequestJSON + " \t"
	page.ResponseJSON = strings.Replace(page.ResponseJSON, `"result":{`, `"extension":{"vendor":"opaque"},"result":{"_meta":{"x":[true,null,1e1000]},`, 1)
	got, err := decodePage(page)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.RequestCursor != (Cursor{Tag: "STRING", Value: ""}) || got.Source.NextCursor != (Cursor{Tag: "STRING", Value: ""}) {
		t.Fatalf("empty cursors changed: %+v", got.Source)
	}
	if got.Names[0] != "sérver_검색" || got.Source.RequestSHA256 != digest([]byte(page.RequestJSON)) || got.Source.ResponseSHA256 != digest([]byte(page.ResponseJSON)) {
		t.Fatal("original bytes/names not bound")
	}
}

func TestSavedFrameRefusals(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Page)
		code   string
	}{
		{"wrong method", func(p *Page) { p.RequestJSON = strings.Replace(p.RequestJSON, "tools/list", "tools/call", 1) }, "RPC_METHOD"},
		{"typed id mismatch", func(p *Page) { p.ResponseJSON = strings.Replace(p.ResponseJSON, `"id":2`, `"id":"2"`, 1) }, "RPC_ID_MISMATCH"},
		{"exponent id", func(p *Page) { p.RequestJSON = strings.Replace(p.RequestJSON, `"id":2`, `"id":2e0`, 1) }, "UNSUPPORTED_RPC_ID"},
		{"id overflow", func(p *Page) { p.RequestJSON = strings.Replace(p.RequestJSON, `"id":2`, `"id":9223372036854775808`, 1) }, "UNSUPPORTED_RPC_ID"},
		{"null id", func(p *Page) { p.ResponseJSON = strings.Replace(p.ResponseJSON, `"id":2`, `"id":null`, 1) }, "UNSUPPORTED_RPC_ID"},
		{"null params", func(p *Page) {
			p.RequestJSON = strings.Replace(p.RequestJSON, `"method":`, `"params":null,"method":`, 1)
		}, "RPC_OBJECT_REQUIRED"},
		{"null request cursor", func(p *Page) {
			p.RequestJSON = strings.Replace(p.RequestJSON, `"method":`, `"params":{"cursor":null},"method":`, 1)
		}, "NULL_REQUEST_CURSOR"},
		{"duplicate nested key", func(p *Page) {
			p.ResponseJSON = strings.Replace(p.ResponseJSON, `"name":"search"`, `"name":"search","name":"other"`, 1)
		}, "DUPLICATE_OR_INVALID_JSON_KEY"},
		{"unpaired surrogate", func(p *Page) { p.ResponseJSON = strings.Replace(p.ResponseJSON, "synthetic fixture", `\ud800`, 1) }, "NONSCALAR_JSON_STRING"},
		{"exposure mismatch", func(p *Page) { p.DeclaredExposedNames = []string{} }, "EXPOSURE_COUNT_MISMATCH"},
		{"unsupported result type", func(p *Page) { p.ResponseJSON = strings.Replace(p.ResponseJSON, `"complete"`, `"task"`, 1) }, "UNSUPPORTED_RESULT_TYPE"},
		{"trailing frame", func(p *Page) { p.ResponseJSON += "{}" }, "TRAILING_JSON"},
		{"frame limit", func(p *Page) { p.ResponseJSON += strings.Repeat(" ", MaxFrameBytes) }, "JSON_SIZE_OR_ENCODING"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := fixturePage([]string{"search"}, []string{"search"}, nil, nil)
			tt.mutate(&p)
			_, err := decodePage(p)
			problem, ok := err.(*Problem)
			if !ok || problem.Code != tt.code {
				t.Fatalf("got %v, want %s", err, tt.code)
			}
		})
	}
}

func TestCapturedErrorPreservesOriginalDiagnostic(t *testing.T) {
	p := fixturePage([]string{}, []string{}, nil, nil)
	p.ResponseJSON = ` {"jsonrpc":"2.0","id":2,"error":{"code":-32001,"message":"denied","data":{"retry":false}}} `
	o := Observation{Captures: []Capture{{Pages: []Page{p}}}}
	_, err := projectObservation("AFTER", o)
	problem, ok := err.(*Problem)
	if !ok || problem.State != "UNKNOWN" || problem.Code != "SAVED_RPC_ERROR" || problem.ResponseJSON != p.ResponseJSON || problem.ResponseSHA256 != digest([]byte(p.ResponseJSON)) || problem.Side != "AFTER" || problem.PageIndex == nil || *problem.PageIndex != 0 {
		t.Fatalf("lost diagnostic: %+v", err)
	}
}

func TestOriginUsesHostConnectionBindingOnly(t *testing.T) {
	a := fixtureInput().Before
	b := fixtureInput().Before
	b.Captures[0].ServerNameClaim = "different claim"
	b.Captures[0].Pages[0].Context.AuthorizationContextID = "other auth"
	b.Captures[0].Pages[0].Context.RelevantConfigurationRevision = "other config"
	x, err := projectObservation("BEFORE", a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := projectObservation("AFTER", b)
	if err != nil {
		t.Fatal(err)
	}
	if x.Entries[0].Server != y.Entries[0].Server {
		t.Fatal("informational/auth/config values changed origin identity")
	}
	b.Captures[0].ConnectionID = "b"
	y, err = projectObservation("AFTER", b)
	if err != nil {
		t.Fatal(err)
	}
	if x.Entries[0].Server == y.Entries[0].Server {
		t.Fatal("connection difference lost")
	}
}

func TestOuterExactStructureAndLimits(t *testing.T) {
	base := encode(t, fixtureInput())
	bad := [][]byte{
		[]byte(strings.Replace(string(base), `"target_alias":"search"`, `"target_alias":"search","target_alias":"other"`, 1)),
		[]byte(strings.Replace(string(base), `"target_alias":"search"`, `"target_alias":"search","execution_authorized":true`, 1)),
		[]byte(strings.Replace(string(base), `"declared_exposed_names":["search"]`, `"declared_exposed_names":null`, 1)),
		[]byte(strings.Replace(string(base), `"target_alias":"search"`, `"target_alias":null`, 1)),
		append(append([]byte(nil), base...), []byte(strings.Repeat(" ", MaxInputBytes))...),
		[]byte(strings.Replace(string(base), `"target_alias":"search"`, `"target_alias":"`+strings.Repeat("x", MaxTextBytes+1)+`"`, 1)),
	}
	for i, raw := range bad {
		if _, err := decodeFacts(raw); err == nil {
			t.Errorf("accepted invalid case %d", i)
		}
	}
	in, err := decodeFacts(base)
	if err != nil || in.Before.Captures[0].Pages[0].ResponseJSON != fixtureInput().Before.Captures[0].Pages[0].ResponseJSON {
		t.Fatal("valid saved facts changed", err)
	}
	empty := fixtureInput()
	empty.TargetAlias = ""
	empty.After.Captures = []Capture{}
	empty.Before.Captures[0].Pages = []Page{}
	if _, err := decodeFacts(encode(t, empty)); err != nil {
		t.Fatal("represented domain absence incorrectly refused by transport", err)
	}
}

func TestNextNullAndAbsentRemainDistinct(t *testing.T) {
	p := fixturePage([]string{}, []string{}, nil, nil)
	a, err := decodePage(p)
	if err != nil {
		t.Fatal(err)
	}
	p.ResponseJSON = strings.Replace(p.ResponseJSON, `"result":{`, `"result":{"nextCursor":null,`, 1)
	b, err := decodePage(p)
	if err != nil {
		t.Fatal(err)
	}
	if a.Source.NextCursor.Tag != "ABSENT" || b.Source.NextCursor.Tag != "NULL" {
		t.Fatal("terminal observations normalized")
	}
}

func TestLongCapturedErrorAndCumulativeRowLocation(t *testing.T) {
	p := fixturePage([]string{}, []string{}, nil, nil)
	p.ResponseJSON = `{"jsonrpc":"2.0","id":2,"error":{"code":-32001,"message":"` + strings.Repeat("x", 1025) + `"}}`
	_, err := decodePage(p)
	problem, ok := err.(*Problem)
	if !ok || problem.State != "UNKNOWN" || problem.ResponseJSON != p.ResponseJSON {
		t.Fatalf("long original error lost: %v", err)
	}
	names := make([]string, 256)
	for i := range names {
		names[i] = "tool"
	}
	first := fixturePage(names, names, nil, nil)
	second := fixturePage([]string{"more"}, []string{"more"}, nil, nil)
	o := Observation{Captures: []Capture{{Pages: []Page{first, second}}}}
	_, err = projectObservation("AFTER", o)
	problem, ok = err.(*Problem)
	if !ok || problem.Code != "ROW_LIMIT" || problem.Side != "AFTER" || problem.PageIndex == nil || *problem.PageIndex != 1 || problem.CaptureIndex == nil || *problem.CaptureIndex != 0 || problem.ResponseSHA256 != digest([]byte(second.ResponseJSON)) || problem.RequestSHA256 != digest([]byte(second.RequestJSON)) {
		t.Fatalf("cumulative bound lost location: %+v", err)
	}
}
