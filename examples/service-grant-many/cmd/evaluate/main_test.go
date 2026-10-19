package main

import (
	"bytes"
	"encoding/json"
	policy "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many"
	"strings"
	"testing"
)

func TestBadLaterStructuralInputHasNoDecisionStdout(t *testing.T) {
	good := `{"before":"*","after":"mcp://*","required_types":["mcp"],"required_names":["weather"]}`
	for _, bad := range []string{`{"before":true}`, `{"before":"*","before":"*","after":"mcp://*","required_types":[],"required_names":[]}`, `{"before":"*","after":"mcp://*","required_types":["mcp"],"required_names":["\ud800"]}`} {
		var out, errout bytes.Buffer
		if code := run(strings.NewReader("["+good+","+bad+"]"), &out, &errout, true); code != 1 || out.Len() != 0 || errout.Len() == 0 {
			t.Fatalf("exit/stdout/stderr %d/%d/%d", code, out.Len(), errout.Len())
		}
	}
}
func TestFriendlyDenialAndEmptyRequestsExitZero(t *testing.T) {
	raw := `[{"before":"mcp://weather.*","after":"mcp://weather.health","required_requests":[{"type":"mcp","name":"weather.forecast"}]},{"before":"*","after":"mcp://*","required_requests":[]}]`
	var out, errout bytes.Buffer
	if code := run(strings.NewReader(raw), &out, &errout, false); code != 0 || errout.Len() != 0 {
		t.Fatalf("exit %d %s", code, errout.String())
	}
	var got []policy.Result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Decision.Status != "REFUSED" || got[0].Decision.Reason != "REQUIRED_REQUEST_REMOVED" || got[1].Decision.Status != "INVALID_INPUT" || got[1].Decision.Reason != "EMPTY_REQUIRED_REQUESTS" {
		t.Fatalf("%#v", got)
	}
	if len(got[0].Facts.RequiredNames) != 1 || got[0].Facts.RequiredNames[0] != "weather.forecast" {
		t.Fatalf("%#v", got[0].Facts)
	}
}
