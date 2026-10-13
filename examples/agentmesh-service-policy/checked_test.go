package servicepolicy

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Frozen independent grammar from domain.go at
// 0c6a55f7021128358be05615f033c4f8c7678bef; original file SHA256:
// 62345dae60badeb5bf69bc5e5995322c09402ab9eb3f0141903aac3041f91528.
// These test-only predicates must not follow the candidate Gooo implementation.
var checkedReferenceScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)
var checkedReferenceName = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

func checkedReferenceType(value string) bool {
	return len(value) <= 128 && (value == "" || checkedReferenceScheme.MatchString(value))
}

func checkedReferenceServiceName(value string) bool {
	return len(value) <= 128 && (value == "" || checkedReferenceName.MatchString(value))
}

func checkedReferencePattern(value string) bool {
	if len(value) > 128 {
		return false
	}
	if value == "" || value == "*" {
		return true
	}
	typ, part, found := strings.Cut(value, "://")
	if !found || !checkedReferenceScheme.MatchString(typ) || part == "" {
		return false
	}
	if part == "*" {
		return true
	}
	if strings.HasPrefix(part, "*.") {
		part = part[2:]
	} else if strings.HasSuffix(part, ".*") {
		part = part[:len(part)-2]
	}
	return checkedReferenceName.MatchString(part)
}

func TestCheckedCanonicalASCII512(t *testing.T) {
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	digits := "0123456789"
	observations := 0
	for c := 0; c < 128; c++ {
		ch := string(rune(c))
		for _, tc := range []struct {
			position string
			value    string
			alphabet string
			ref      func(string) bool
			native   func(string) bool
		}{
			{"type-first", ch, letters, checkedReferenceType, validType},
			{"type-continuation", "a" + ch, letters + digits + "+.-", checkedReferenceType, validType},
			{"name-single-label", ch, letters + digits + "_-", checkedReferenceServiceName, validName},
			{"name-after-dot", "a." + ch, letters + digits + "_-", checkedReferenceServiceName, validName},
		} {
			t.Run(fmt.Sprintf("%s/%03d", tc.position, c), func(t *testing.T) {
				want := strings.Contains(tc.alphabet, ch)
				if got := tc.ref(tc.value); got != want {
					t.Fatalf("frozen oracle disagrees with independently authored byte alphabet: %q => %v, want %v", tc.value, got, want)
				}
				if got := tc.native(tc.value); got != want {
					t.Fatalf("generated Gooo predicate differs at %q: got %v, want %v", tc.value, got, want)
				}
			})
			observations++
		}
	}
	if observations != 512 {
		t.Fatal("ASCII observation contract changed", observations)
	}
}

func TestCheckedCanonicalGrammarBoundaries(t *testing.T) {
	for _, value := range []string{"é", "한", "😀", "a\u0080b", "a\x7fb", "a\x00b", "a\nb", string([]byte{0xff}), "\ufffd", "a b"} {
		for _, tc := range []struct {
			kind string
			ref  func(string) bool
			got  func(string) bool
		}{
			{"type", checkedReferenceType, validType},
			{"name", checkedReferenceServiceName, validName},
		} {
			t.Run(fmt.Sprintf("%s/%x", tc.kind, value), func(t *testing.T) {
				if tc.ref(value) || tc.got(value) {
					t.Fatalf("non-domain bytes accepted: %q", value)
				}
			})
		}
	}
	for _, n := range []int{0, 1, 127, 128, 129} {
		value := strings.Repeat("a", n)
		want := n <= 128
		if checkedReferenceType(value) != want || validType(value) != want || checkedReferenceServiceName(value) != want || validName(value) != want {
			t.Fatalf("independent request component length %d differs", n)
		}
	}
	patterns := []struct {
		value string
		want  bool
	}{
		{"", true}, {"*", true}, {"a://a", true}, {"a://*", true},
		{"a+1.-://x", true}, {"a..://x", true}, {"MCP://A", true},
		{"mcp://0_A-.b_2", true}, {"mcp://_", true}, {"mcp://-", true},
		{"mcp://a.*", true}, {"mcp://*.a", true}, {"mcp://a.b.*", true}, {"mcp://*.a.b", true},
		{":", false}, {":/", false}, {"://", false}, {"://a", false}, {"a://", false},
		{"mcp:a", false}, {"1mcp://a", false}, {"mcp://*.*", false}, {"mcp://*.a.*", false},
		{"mcp://*.", false}, {"mcp://.*", false}, {"mcp://a*b", false}, {"mcp://a..b", false},
		{"mcp://.a", false}, {"mcp://a.", false}, {"mcp://a://b", false}, {"mcp://a/path", false},
		{"mcp://a?x", false}, {"mcp://a#x", false}, {"mcp://user@a", false}, {"mcp://a:443", false},
		{"mcp://%61", false}, {" mcp://a", false}, {"mcp://a ", false}, {"mcp://é", false},
		{"mcp://a\x00b", false}, {"mcp://" + string([]byte{0xff}), false},
	}
	for _, n := range []int{127, 128, 129} {
		patterns = append(patterns,
			struct {
				value string
				want  bool
			}{"a://" + strings.Repeat("x", n-4), n <= 128},
			struct {
				value string
				want  bool
			}{"a://" + strings.Repeat("x", n-6) + ".*", n <= 128},
			struct {
				value string
				want  bool
			}{"a://*." + strings.Repeat("x", n-6), n <= 128},
			struct {
				value string
				want  bool
			}{strings.Repeat("a", n-4) + "://*", n <= 128})
	}
	for i, tc := range patterns {
		t.Run(fmt.Sprintf("pattern/%02d", i), func(t *testing.T) {
			if checkedReferencePattern(tc.value) != tc.want || validPattern(tc.value) != tc.want {
				t.Fatalf("pattern boundary differs: %q, want %v", tc.value, tc.want)
			}
		})
	}
}

func TestCheckedMatcherInvalidSentinelAndPrecedence(t *testing.T) {
	for _, tc := range []struct {
		f     MatcherFacts
		field string
	}{
		{MatcherFacts{"*", "bad/type", "x"}, "req_type"},
		{MatcherFacts{"*", "mcp", "bad/name"}, "req_name"},
		{MatcherFacts{"", "1mcp", "x"}, "req_type"},
		{MatcherFacts{"mcp://a/path", "", ""}, "pattern"},
		{MatcherFacts{"mcp://a/path", "mcp", "a/path"}, "pattern"},
		{MatcherFacts{"*", strings.Repeat("a", 129), "x"}, "req_type"},
		{MatcherFacts{"*", "mcp", strings.Repeat("x", 129)}, "req_name"},
		{MatcherFacts{"mcp:a", "1mcp", "a..b"}, "pattern"},
		{MatcherFacts{"*", "é", "a/b"}, "req_type"},
		{MatcherFacts{"*", "", "a\x00b"}, "req_name"},
	} {
		p, err := EvaluateMatcherChecked(tc.f)
		want := CheckedMatcherDecision{DomainStatus: "INVALID_INPUT", InvalidField: tc.field}
		if err != nil || p.Facts != tc.f || p.Decision != want {
			t.Fatalf("checked matcher failed refusal: facts=%#v result=%+v err=%v want=%+v", tc.f, p, err, want)
		}
		if _, err := EvaluateMatcher(tc.f); err == nil {
			t.Fatal("legacy matcher promoted typed invalid into a proof", tc.f)
		}
	}
}

func checkedMalformedPatterns() []string {
	return []string{"mcp:a", "://a", "mcp://", "mcp://a/path", "mcp://a?q=x", "mcp://a#x", "mcp://user@a", "mcp://a:443", "mcp://*.a.*", "mcp://*.*", "mcp://a*b", "mcp://a..b", "mcp://.a", "mcp://a.", "mcp://*.", "mcp://.*", "mcp://a://b", "1mcp://x", "mcp://é", "mcp://a\x00b", "a://" + strings.Repeat("x", 125)}
}

func TestCheckedImpactInvalidSentinelAndPrecedence(t *testing.T) {
	cases := []struct {
		f     ImpactFacts
		field string
	}{
		{ImpactFacts{"*", "*", "bad/type", "x"}, "req_type"},
		{ImpactFacts{"*", "*", "mcp", "a/path"}, "req_name"},
		{ImpactFacts{"", "", "mcp", "a..b"}, "req_name"},
		{ImpactFacts{"*", "mcp://*.a.*", "", ""}, "after"},
		{ImpactFacts{"mcp://a", "mcp://a/path", "bad/type", "bad/name"}, "after"},
		{ImpactFacts{"*", "*", strings.Repeat("a", 129), ""}, "req_type"},
	}
	for _, bad := range checkedMalformedPatterns() {
		cases = append(cases,
			struct {
				f     ImpactFacts
				field string
			}{ImpactFacts{bad, "", "mcp", "x"}, "before"},
			struct {
				f     ImpactFacts
				field string
			}{ImpactFacts{"", bad, "mcp", "x"}, "after"},
			struct {
				f     ImpactFacts
				field string
			}{ImpactFacts{bad, bad, "mcp", "x"}, "before"},
			struct {
				f     ImpactFacts
				field string
			}{ImpactFacts{"*", bad, "", ""}, "after"})
	}
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			p, err := EvaluateImpactChecked(tc.f)
			want := CheckedImpactDecision{DomainStatus: "INVALID_INPUT", InvalidField: tc.field, Change: "INVALID_INPUT"}
			if err != nil || p.Facts != tc.f || p.Decision != want {
				t.Fatalf("checked impact confused invalid with a normal state: facts=%#v result=%+v err=%v want=%+v", tc.f, p, err, want)
			}
			if _, err := EvaluateImpact(tc.f); err == nil {
				t.Fatal("legacy impact promoted typed invalid into a proof")
			}
		})
	}
}

func TestCheckedRefinementInvalidSentinelAndPrecedence(t *testing.T) {
	for i, bad := range checkedMalformedPatterns() {
		for j, tc := range []struct {
			f     RefinementFacts
			field string
		}{
			{RefinementFacts{bad, ""}, "before"},
			{RefinementFacts{bad, bad}, "before"},
			{RefinementFacts{"*", bad}, "after"},
			{RefinementFacts{bad, "mcp://a"}, "before"},
			{RefinementFacts{"mcp://*", bad}, "after"},
			{RefinementFacts{"mcp://a.*", bad}, "after"},
			{RefinementFacts{"mcp://*.a", bad}, "after"},
		} {
			t.Run(fmt.Sprintf("%d/%d", i, j), func(t *testing.T) {
				p, err := EvaluateRefinementChecked(tc.f)
				want := CheckedRefinementDecision{DomainStatus: "INVALID_INPUT", InvalidField: tc.field, RefinementDecision: RefinementDecision{Status: "INVALID_INPUT"}}
				if err != nil || p.Facts != tc.f || p.Decision != want {
					t.Fatalf("checked refinement shortcut bypass: facts=%#v result=%+v err=%v want=%+v", tc.f, p, err, want)
				}
				if _, err := EvaluateRefinement(tc.f); err == nil {
					t.Fatal("legacy refinement promoted typed invalid into a proof")
				}
			})
		}
	}
}

func checkedJSONFields(t *testing.T, value any, names ...string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != len(names) {
		t.Fatalf("JSON shape changed: %s %v", raw, err)
	}
	for _, name := range names {
		if field, ok := fields[name]; !ok || string(field) == "null" {
			t.Fatalf("missing/null field %q in %s", name, raw)
		}
	}
}

func TestCheckedValidSemanticFieldsAndNativeCompatibility(t *testing.T) {
	read := func(name string, out any) {
		t.Helper()
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatal(err)
		}
	}
	var matcher []struct {
		Name string `json:"name"`
		MatcherFacts
		Matched bool `json:"matched"`
	}
	read("oracle/observations.json", &matcher)
	if len(matcher) != 63 {
		t.Fatal("matcher compatibility corpus changed")
	}
	for _, c := range matcher {
		t.Run("matcher/"+c.Name, func(t *testing.T) {
			p, err := EvaluateMatcherChecked(c.MatcherFacts)
			if err != nil || p.Facts != c.MatcherFacts || p.Decision != (CheckedMatcherDecision{DomainStatus: "VALID_INPUT", Matched: c.Matched}) {
				t.Fatalf("checked matcher differs from independent retained observation: %+v %v", p, err)
			}
			legacy, err := EvaluateMatcher(c.MatcherFacts)
			if err != nil || legacy.Facts != p.Facts || legacy.Matched != c.Matched || !reflect.DeepEqual(legacy.Bindings, p.Bindings) {
				t.Fatalf("native matcher compatibility differs: %+v %v", legacy, err)
			}
			checkedJSONFields(t, p.Decision, "domain_status", "invalid_field", "matched")
			checkedJSONFields(t, legacy, "facts", "matched", "bindings")
		})
	}
	var impacts []struct {
		Name string `json:"name"`
		ImpactFacts
		Want ImpactDecision `json:"want"`
	}
	read("impact-cases.json", &impacts)
	if len(impacts) != 16 {
		t.Fatal("impact compatibility corpus changed")
	}
	for _, c := range impacts {
		t.Run("impact/"+c.Name, func(t *testing.T) {
			p, err := EvaluateImpactChecked(c.ImpactFacts)
			want := CheckedImpactDecision{DomainStatus: "VALID_INPUT", BeforeMatch: c.Want.BeforeMatch, AfterMatch: c.Want.AfterMatch, Change: c.Want.Change}
			if err != nil || p.Facts != c.ImpactFacts || p.Decision != want {
				t.Fatalf("checked impact differs: %+v %v", p, err)
			}
			legacy, err := EvaluateImpact(c.ImpactFacts)
			if err != nil || legacy.Facts != p.Facts || legacy.Decision != c.Want || !reflect.DeepEqual(legacy.Bindings, p.Bindings) {
				t.Fatalf("native impact compatibility differs: %+v %v", legacy, err)
			}
			checkedJSONFields(t, p.Decision, "domain_status", "invalid_field", "before_match", "after_match", "change")
			checkedJSONFields(t, legacy.Decision, "before_match", "after_match", "change")
		})
	}
	var refinements []struct {
		Name string `json:"name"`
		RefinementFacts
		Want RefinementDecision `json:"want"`
	}
	read("refinement-cases.json", &refinements)
	if len(refinements) != 101 {
		t.Fatal("refinement compatibility corpus changed")
	}
	for _, c := range refinements {
		t.Run("refinement/"+c.Name, func(t *testing.T) {
			p, err := EvaluateRefinementChecked(c.RefinementFacts)
			want := CheckedRefinementDecision{DomainStatus: "VALID_INPUT", RefinementDecision: c.Want}
			if err != nil || p.Facts != c.RefinementFacts || p.Decision != want {
				t.Fatalf("checked refinement differs: %+v %v", p, err)
			}
			legacy, err := EvaluateRefinement(c.RefinementFacts)
			if err != nil || legacy.Facts != p.Facts || legacy.Decision != c.Want || !reflect.DeepEqual(legacy.Bindings, p.Bindings) {
				t.Fatalf("native refinement compatibility differs: %+v %v", legacy, err)
			}
			checkedJSONFields(t, p.Decision, "domain_status", "invalid_field", "status", "rule_id", "supported_subset", "has_witness", "witness_type", "witness_name", "before_match", "after_match")
			checkedJSONFields(t, legacy.Decision, "status", "rule_id", "supported_subset", "has_witness", "witness_type", "witness_name", "before_match", "after_match")
		})
	}
}

func TestCheckedDecisionRefusalContradictions(t *testing.T) {
	mf := MatcherFacts{"*", "bad/type", "bad/name"}
	md := CheckedMatcherDecision{DomainStatus: "INVALID_INPUT", InvalidField: "req_type"}
	if err := validateCheckedMatcherDecision(mf, md); err != nil {
		t.Fatal("valid matcher refusal rejected", err)
	}
	for _, change := range []func(*CheckedMatcherDecision){
		func(d *CheckedMatcherDecision) { d.DomainStatus = "" },
		func(d *CheckedMatcherDecision) { d.DomainStatus = "UNKNOWN" },
		func(d *CheckedMatcherDecision) { d.DomainStatus = "VALID_INPUT" },
		func(d *CheckedMatcherDecision) { d.InvalidField = "" },
		func(d *CheckedMatcherDecision) { d.InvalidField = "req_name" },
		func(d *CheckedMatcherDecision) { d.Matched = true },
	} {
		d := md
		change(&d)
		if err := validateCheckedMatcherDecision(mf, d); err == nil {
			t.Fatal("contradictory matcher refusal accepted", d)
		}
	}
	inf := ImpactFacts{"mcp://a/path", "mcp://*.a.*", "bad/type", "bad/name"}
	ind := CheckedImpactDecision{DomainStatus: "INVALID_INPUT", InvalidField: "before", Change: "INVALID_INPUT"}
	if err := validateCheckedImpactDecision(inf, ind); err != nil {
		t.Fatal("valid impact refusal rejected", err)
	}
	for _, change := range []func(*CheckedImpactDecision){
		func(d *CheckedImpactDecision) { d.DomainStatus = "UNKNOWN" },
		func(d *CheckedImpactDecision) { d.DomainStatus = "VALID_INPUT" },
		func(d *CheckedImpactDecision) { d.InvalidField = "" },
		func(d *CheckedImpactDecision) { d.InvalidField = "after" },
		func(d *CheckedImpactDecision) { d.BeforeMatch = true },
		func(d *CheckedImpactDecision) { d.AfterMatch = true },
		func(d *CheckedImpactDecision) { d.Change = "UNCHANGED_DENY" },
		func(d *CheckedImpactDecision) { d.Change = "UNPROVEN" },
	} {
		d := ind
		change(&d)
		if err := validateCheckedImpactDecision(inf, d); err == nil {
			t.Fatal("contradictory impact refusal accepted", d)
		}
	}
	rf := RefinementFacts{"mcp://a/path", ""}
	rd := CheckedRefinementDecision{DomainStatus: "INVALID_INPUT", InvalidField: "before", RefinementDecision: RefinementDecision{Status: "INVALID_INPUT"}}
	if err := validateCheckedRefinementDecision(rf, rd); err != nil {
		t.Fatal("valid refinement refusal rejected", err)
	}
	for _, change := range []func(*CheckedRefinementDecision){
		func(d *CheckedRefinementDecision) { d.DomainStatus = "UNKNOWN" },
		func(d *CheckedRefinementDecision) { d.DomainStatus = "VALID_INPUT" },
		func(d *CheckedRefinementDecision) { d.InvalidField = "" },
		func(d *CheckedRefinementDecision) { d.InvalidField = "after" },
		func(d *CheckedRefinementDecision) { d.Status = "UNPROVEN" },
		func(d *CheckedRefinementDecision) { d.Status = "SUPPORTED_SUBSET" },
		func(d *CheckedRefinementDecision) { d.RuleID = "CANDIDATE_EMPTY" },
		func(d *CheckedRefinementDecision) { d.SupportedSubset = true },
		func(d *CheckedRefinementDecision) { d.HasWitness = true },
		func(d *CheckedRefinementDecision) { d.WitnessType = "mcp" },
		func(d *CheckedRefinementDecision) { d.WitnessName = "x" },
		func(d *CheckedRefinementDecision) { d.BeforeMatch = true },
		func(d *CheckedRefinementDecision) { d.AfterMatch = true },
	} {
		d := rd
		change(&d)
		if err := validateCheckedRefinementDecision(rf, d); err == nil {
			t.Fatal("contradictory refinement refusal accepted", d)
		}
	}
}

func TestCheckedDecisionDecoderExactTypedFields(t *testing.T) {
	base := map[string]any{"domain_status": "INVALID_INPUT", "invalid_field": "req_type", "matched": false}
	keys := []string{"domain_status", "invalid_field", "matched"}
	var good CheckedMatcherDecision
	if err := decodeDecision(base, keys, &good); err != nil || good != (CheckedMatcherDecision{DomainStatus: "INVALID_INPUT", InvalidField: "req_type"}) {
		t.Fatalf("valid exact record refused: %+v %v", good, err)
	}
	for _, key := range keys {
		for _, mutation := range []string{"missing", "null", "wrong_type", "extra"} {
			t.Run(key+"/"+mutation, func(t *testing.T) {
				bad := map[string]any{}
				for k, v := range base {
					bad[k] = v
				}
				switch mutation {
				case "missing":
					delete(bad, key)
				case "null":
					bad[key] = nil
				case "wrong_type":
					bad[key] = 0 // In particular, false must not accept numeric zero.
				case "extra":
					bad["extra_"+key] = false
				}
				var d CheckedMatcherDecision
				if err := decodeDecision(bad, keys, &d); err == nil {
					t.Fatal("malformed typed record accepted", bad)
				}
			})
		}
	}
}

func TestCheckedDecoderSeparatesStructureFromGoooDomain(t *testing.T) {
	keys := []string{"pattern", "req_type", "req_name"}
	for _, value := range []string{"é", "한", "😀", "a\x00b", "a\nb", strings.Repeat("a", 129)} {
		f := MatcherFacts{Pattern: "*", ReqType: "mcp", ReqName: value}
		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeFacts(raw, keys); err != nil {
			t.Fatalf("structurally valid text did not reach the Gooo domain boundary: %q: %v", value, err)
		}
		if _, err := DecodeMatcher(raw); err == nil {
			t.Fatal("native decoder must retain domain refusal", value)
		}
		p, err := EvaluateMatcherChecked(f)
		if err != nil || p.Facts != f || p.Decision != (CheckedMatcherDecision{DomainStatus: "INVALID_INPUT", InvalidField: "req_name"}) {
			t.Fatalf("checked Gooo refused to classify raw domain-invalid text: %+v %v", p, err)
		}
	}
	for _, raw := range []string{
		`{"pattern":"*","req_type":"mcp","req_name":"\ud83d\ude00"}`,
		`{"pattern":"*","req_type":"mcp","req_name":"\u0000"}`,
	} {
		if err := decodeFacts([]byte(raw), keys); err != nil {
			t.Fatal("well-formed escaped text rejected before Gooo domain validation", err)
		}
		if _, err := DecodeMatcher([]byte(raw)); err == nil {
			t.Fatal("escaped non-domain text accepted")
		}
	}
	for _, value := range []string{`"\ud800"`, `"\udc00"`, `"\ud800x"`, `"\ud800\u0041"`, `"\ud800\ud800"`, `"\udc00\ud800"`, `"\ud83d\ude00\ud800"`, string([]byte{'"', 0xff, '"'})} {
		raw := []byte(`{"pattern":"*","req_type":"mcp","req_name":` + value + `}`)
		if err := decodeFacts(raw, keys); err == nil {
			t.Fatalf("lossy malformed Unicode was accepted structurally: %s", raw)
		}
	}
}

func TestCheckedNativeTextRefusesLossyUTF8Transport(t *testing.T) {
	bad := string([]byte{0xff})
	for _, f := range []MatcherFacts{{bad, "mcp", "x"}, {"*", bad, "x"}, {"*", "mcp", bad}} {
		if p, err := EvaluateMatcherChecked(f); err == nil || p.Decision.DomainStatus != "" {
			t.Fatalf("non-Text bytes must error before returning a domain decision: %+v %v", p, err)
		}
	}
	for _, f := range []ImpactFacts{{bad, "*", "mcp", "x"}, {"*", bad, "mcp", "x"}, {"*", "*", bad, "x"}, {"*", "*", "mcp", bad}} {
		if p, err := EvaluateImpactChecked(f); err == nil || p.Decision.DomainStatus != "" {
			t.Fatalf("non-Text impact bytes must error: %+v %v", p, err)
		}
	}
	for _, f := range []RefinementFacts{{bad, "*"}, {"*", bad}} {
		if p, err := EvaluateRefinementChecked(f); err == nil || p.Decision.DomainStatus != "" {
			t.Fatalf("non-Text refinement bytes must error: %+v %v", p, err)
		}
	}
}

func TestCheckedRehashedBoundaryBindingsRefuseDowngrade(t *testing.T) {
	types := []struct {
		kind      string
		canonical string
		pkg       string
		activity  string
		record    string
	}{
		{"matcher", "agentmesh-matcher.workspace.json", "gooo/service-policy/checked-matcher", "MatchServicePatternChecked", "CheckedMatcherDecision"},
		{"impact", "service-grant-impact.workspace.json", "gooo/service-policy/checked-impact", "CompareServiceGrantChecked", "CheckedImpactDecision"},
		{"refinement", "service-grant-refinement.workspace.json", "gooo/service-policy/checked-refinement", "AnalyzeServiceGrantChecked", "CheckedRefinementDecision"},
	}
	load := func(t *testing.T) (map[string][]byte, generation) {
		t.Helper()
		files := map[string][]byte{}
		for _, name := range boundNames {
			raw, err := assets.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			files[name] = raw
		}
		raw, err := assets.ReadFile("generation.json")
		if err != nil {
			t.Fatal(err)
		}
		var g generation
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatal(err)
		}
		if _, err := checkBindings(files, raw); err != nil {
			t.Fatal("unmodified binding fixture must pass before testing mutations", err)
		}
		return files, g
	}
	reject := func(t *testing.T, files map[string][]byte, g generation, changed string) {
		t.Helper()
		g.Assets[changed] = digest(files[changed])
		manifest, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := checkBindings(files, manifest); err == nil {
			t.Fatal("coherently rehashed checked-boundary contradiction accepted")
		}
	}
	obj := func(v any) map[string]any { return v.(map[string]any) }
	array := func(v any) []any { return v.([]any) }
	for _, tc := range types {
		t.Run(tc.kind+"/default-downgrade-to-raw", func(t *testing.T) {
			files, g := load(t)
			target := tc.kind + "-workspace.json"
			files[target] = append([]byte(nil), files[tc.kind+"-raw-workspace.json"]...)
			// Rehash both public source and embedded asset, so a hash-only check
			// cannot distinguish this complete valid raw manifest from a default.
			g.PublicSources[tc.canonical] = digest(files[target])
			reject(t, files, g, target)
		})
		t.Run(tc.kind+"/checked-record-domain-field-identity", func(t *testing.T) {
			files, g := load(t)
			target := "receipts/checked-" + tc.kind + "-body-codegen.json"
			var doc map[string]any
			if err := json.Unmarshal(files[target], &doc); err != nil {
				t.Fatal(err)
			}
			changed := false
			for _, record := range array(obj(doc["report"])["record_types"]) {
				r := obj(record)
				if r["name"] != tc.record {
					continue
				}
				for _, field := range array(r["fields"]) {
					f := obj(field)
					if f["name"] == "domain_status" {
						f["id"] = "gooo://service-policy/forged/domain_status"
						changed = true
					}
				}
			}
			if !changed {
				t.Fatal("missing checked domain-status field in generated record")
			}
			files[target], _ = json.Marshal(doc)
			reject(t, files, g, target)
		})
	}
	for _, mutation := range []string{"domain-export-name-id-pair", "domain-call-same-type-wrong-predicate"} {
		t.Run(mutation, func(t *testing.T) {
			files, g := load(t)
			target := "receipts/checked-matcher-program.json"
			var doc map[string]any
			if err := json.Unmarshal(files[target], &doc); err != nil {
				t.Fatal(err)
			}
			changed := false
			if mutation == "domain-export-name-id-pair" {
				for _, pkg := range array(obj(doc["workspace_image"])["packages"]) {
					p := obj(pkg)
					if p["path"] != "gooo/service-domain" {
						continue
					}
					for _, export := range array(p["exports"]) {
						e := obj(export)
						if e["name"] == "ValidateServiceName" && e["kind"] == "activity" {
							e["id"] = "gooo://service-domain/type"
							changed = true
						}
					}
				}
			} else {
				for _, site := range array(obj(doc["pure_calls"])["sites"]) {
					s := obj(site)
					if s["caller"] == "gooo/service-policy/checked-matcher:MatchServicePatternChecked" && s["callee"] == "gooo/service-domain:ValidateServicePattern" {
						s["callee"] = "gooo/service-domain:ValidateServiceName"
						changed = true
					}
				}
			}
			if !changed {
				t.Fatal("missing source-bound mutation target")
			}
			files[target], _ = json.Marshal(doc)
			reject(t, files, g, target)
		})
	}
}
