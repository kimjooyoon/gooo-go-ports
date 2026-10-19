package servicegrantmany

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type corpusCase struct {
	Name     string         `json:"name"`
	Facts    RequestColumns `json:"facts"`
	Decision Decision       `json:"decision"`
}

func corpus(t *testing.T) []corpusCase {
	t.Helper()
	raw, err := os.ReadFile("reference-expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []corpusCase
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 16 {
		t.Fatal(len(cases))
	}
	return cases
}
func TestOriginalSixteenFullDecisions(t *testing.T) {
	for _, c := range corpus(t) {
		t.Run(c.Name, func(t *testing.T) {
			if got := EvaluateColumns(c.Facts); got != c.Decision {
				t.Fatalf("got %#v want %#v", got, c.Decision)
			}
		})
	}
}
func boundary257() RequestColumns {
	f := RequestColumns{Before: "*", After: "mcp://*"}
	for i := 0; i < 257; i++ {
		f.RequiredTypes = append(f.RequiredTypes, "mcp")
		f.RequiredNames = append(f.RequiredNames, "n"+strconv.Itoa(i))
	}
	return f
}
func Test257ReachesGoooPolicy(t *testing.T) {
	f := boundary257()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeColumns(raw)
	if err != nil {
		t.Fatalf("transport refused257: %v", err)
	}
	want := Decision{Status: "INVALID_INPUT", Reason: "REQUIRED_REQUEST_LIMIT_EXCEEDED", InvalidField: "required_types", InvalidIndex: -1, FirstCounterexample: -1, RequestCount: 257}
	if got := EvaluateColumns(decoded); got != want {
		t.Fatalf("%#v", got)
	}
	friendly := Input{Before: "*", After: "mcp://*"}
	for i := 0; i < 257; i++ {
		friendly.RequiredRequests = append(friendly.RequiredRequests, Request{Type: f.RequiredTypes[i], Name: f.RequiredNames[i]})
	}
	if got := Evaluate(friendly); got != want {
		t.Fatalf("friendly %#v", got)
	}
}
func TestFriendlyProjectionPreservesDuplicateOrder(t *testing.T) {
	in := Input{Before: "*", After: "mcp://weather", RequiredRequests: []Request{{"mcp", "weather"}, {"mcp", "weather"}, {"mcp", "calculator"}}}
	want := corpus(t)[5]
	if !reflect.DeepEqual(in.Columns(), want.Facts) {
		t.Fatalf("%#v", in.Columns())
	}
	if got := Evaluate(in); got != want.Decision {
		t.Fatalf("%#v", got)
	}
}
func TestStructuralRefusals(t *testing.T) {
	cases := []string{
		`{"before":"*","after":"mcp://*","required_types":null,"required_names":[]}`,
		`{"before":"*","after":"mcp://*","required_types":[1],"required_names":["n"]}`,
		`{"before":"*","after":"mcp://*","required_types":["mcp"],"required_names":[null]}`,
		`{"before":"*","before":"*","after":"mcp://*","required_types":[],"required_names":[]}`,
		`{"before":"*","after":"mcp://*","required_types":[],"required_names":[],"extra":1}`,
		`{"before":"*","after":"mcp://*","required_types":[]}`,
		`{"before":null,"after":"mcp://*","required_types":[],"required_names":[]}`,
		`{"before":"*","after":"mcp://*","required_types":["mcp"],"required_names":["\ud800"]}`,
	}
	for _, raw := range cases {
		if _, err := DecodeColumns([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, raw := range []string{
		`{"before":"*","after":"mcp://*","required_requests":null}`,
		`{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"n","extra":1}]}`,
		`{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"n","name":"x"}]}`,
		`{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":null}]}`,
	} {
		if _, err := DecodeInput([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestRepresentedDomainInvalidIsNotTransportError(t *testing.T) {
	for _, name := range []string{"", strings.Repeat("a", 129), "weather/forecast", "날씨", "\U0001f600"} {
		raw, _ := json.Marshal(RequestColumns{Before: "*", After: "mcp://*", RequiredTypes: []string{"mcp"}, RequiredNames: []string{name}})
		in, err := DecodeColumns(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := EvaluateColumns(in); got.Status != "INVALID_INPUT" {
			t.Fatalf("%#v", got)
		}
	}
	raw := []byte(`{"before":"*","after":"mcp://*","required_types":["mcp"],"required_names":["\ud83d\ude00"]}`)
	if _, err := DecodeColumns(raw); err != nil {
		t.Fatal("valid surrogate pair rejected", err)
	}
}
func TestFull256WireDoesNotUseSingleRequest4KiBCap(t *testing.T) {
	in := RequestColumns{Before: "*", After: "mcp://*"}
	for i := 0; i < 256; i++ {
		in.RequiredTypes = append(in.RequiredTypes, strings.Repeat("a", 128))
		in.RequiredNames = append(in.RequiredNames, strings.Repeat("b", 128))
	}
	raw, _ := json.Marshal(in)
	if len(raw) <= 4096 {
		t.Fatal("fixture too small")
	}
	if _, err := DecodeColumns(raw); err != nil {
		t.Fatal(err)
	}
}

func TestAssessColumnsSnapshotsCallerOwnedSlices(t *testing.T) {
	input := RequestColumns{Before: "*", After: "mcp://weather.forecast",
		RequiredTypes: []string{"mcp"}, RequiredNames: []string{"weather.forecast"}}
	first := AssessColumns(input)
	input.RequiredTypes[0] = "http"
	input.RequiredNames[0] = "weather.health"
	if first.Facts.RequiredTypes[0] != "mcp" || first.Facts.RequiredNames[0] != "weather.forecast" {
		t.Fatalf("caller reuse changed retained facts: %#v", first.Facts)
	}
	if first.Decision != EvaluateColumns(first.Facts) {
		t.Fatalf("retained facts no longer describe the decision: %#v", first)
	}
	first.Facts.RequiredTypes[0] = "changed-result"
	first.Facts.RequiredNames[0] = "changed-result"
	if input.RequiredTypes[0] != "http" || input.RequiredNames[0] != "weather.health" {
		t.Fatalf("result mutation changed caller input: %#v", input)
	}
}

func TestAssessColumnsSnapshotPreservesNilAndEmptyColumns(t *testing.T) {
	for _, input := range []RequestColumns{
		{Before: "*", After: "mcp://*"},
		{Before: "*", After: "mcp://*", RequiredTypes: []string{}},
		{Before: "*", After: "mcp://*", RequiredNames: []string{}},
		{Before: "*", After: "mcp://*", RequiredTypes: []string{}, RequiredNames: []string{}},
	} {
		result := AssessColumns(input)
		if (result.Facts.RequiredTypes == nil) != (input.RequiredTypes == nil) ||
			(result.Facts.RequiredNames == nil) != (input.RequiredNames == nil) {
			t.Fatalf("snapshot changed nil/empty representation: input %#v facts %#v", input, result.Facts)
		}
		before, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		after, err := json.Marshal(result.Facts)
		if err != nil || string(before) != string(after) {
			t.Fatalf("facts JSON changed: before %s after %s error %v", before, after, err)
		}
		if result.Decision != EvaluateColumns(input) {
			t.Fatal("snapshot changed a represented policy decision")
		}
	}
}
