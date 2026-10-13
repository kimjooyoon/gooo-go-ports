package nativepolicy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExactFactSchemaRejectsMalformed(t *testing.T) {
	good := `{"group_present":false,"group_value":"admin","principal_present":false,"principal_value":"user:me@acme.co"}`
	if _, err := DecodeScalar([]byte(good)); err != nil {
		t.Fatal(err)
	}
	cases := []string{
		`null`, `[]`, `{}`,
		strings.Replace(good, `"group_present":false,`, "", 1),
		strings.Replace(good, `"group_present":false`, `"group_present":null`, 1),
		strings.Replace(good, `"group_present":false`, `"group_present":0`, 1),
		strings.Replace(good, `"group_value":"admin"`, `"group_value":null`, 1),
		strings.Replace(good, `"group_value":"admin"`, `"group_value":true`, 1),
		strings.TrimSuffix(good, "}") + `,"allowed":true}`,
		strings.TrimSuffix(good, "}") + `,"group_present":true}`,
		good + ` {}`,
		strings.Replace(good, `"group_value":"admin"`, `"group_value":"\ud800"`, 1),
		strings.Replace(good, `"group_value":"admin"`, `"group_value":"\udc00"`, 1),
		strings.Replace(good, `"group_value":"admin"`, `"group_value":"\ud800\u0061"`, 1),
	}
	for i, raw := range cases {
		if _, err := DecodeScalar([]byte(raw)); err == nil {
			t.Fatalf("malformed case %d accepted", i)
		}
	}
	oversize := Facts{GroupPresent: true, GroupValue: strings.Repeat("x", MaxTextBytes+1)}
	raw, _ := json.Marshal(oversize)
	if _, err := DecodeScalar(raw); err == nil {
		t.Fatal("oversize text accepted")
	}
	if _, err := DecodeScalar(append([]byte(good), 0xff)); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	paired := strings.Replace(good, `"group_value":"admin"`, `"group_value":"\ud83d\ude00"`, 1)
	if decoded, err := DecodeScalar([]byte(paired)); err != nil || decoded.GroupValue != "😀" {
		t.Fatal("valid Unicode pair rejected or changed")
	}
}

func TestTextBoundAndAPIRawSchema(t *testing.T) {
	f := Facts{GroupPresent: true, GroupValue: strings.Repeat("x", MaxTextBytes), PrincipalPresent: true}
	raw, _ := json.Marshal(f)
	if _, err := DecodeScalar(raw); err != nil {
		t.Fatal(err)
	}
	api := APIFacts{Facts: f, Authenticated: true, CallerTenant: "tenant", ResourceTenant: "tenant", RequestedAction: "read", AllowedAction: "read"}
	raw, _ = json.Marshal(api)
	if _, err := DecodeAPI(raw); err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	json.Unmarshal(raw, &object)
	delete(object, "authenticated")
	raw, _ = json.Marshal(object)
	if _, err := DecodeAPI(raw); err == nil {
		t.Fatal("missing authentication fact accepted")
	}
}

func TestNativeEntryRefusesOutsideTextDomain(t *testing.T) {
	for _, value := range []string{strings.Repeat("x", MaxTextBytes+1), string([]byte{0xff})} {
		if _, err := EvaluateScalar(Facts{GroupValue: value}); err == nil {
			t.Fatal("scalar native entry accepted outside-domain text")
		}
		if _, err := EvaluateAPI(APIFacts{CallerTenant: value}); err == nil {
			t.Fatal("API native entry accepted outside-domain text")
		}
		if _, err := EvaluateAPI(APIFacts{RequestedAction: value}); err == nil {
			t.Fatal("API native entry accepted outside-domain action text")
		}
	}
	bound := strings.Repeat("x", MaxTextBytes)
	if _, err := EvaluateScalar(Facts{GroupValue: bound, PrincipalValue: bound}); err != nil {
		t.Fatal("scalar valid boundary rejected", err)
	}
	if _, err := EvaluateAPI(APIFacts{Facts: Facts{GroupValue: bound, PrincipalValue: bound}, CallerTenant: bound, ResourceTenant: bound, RequestedAction: bound, AllowedAction: bound}); err != nil {
		t.Fatal("API valid boundary rejected", err)
	}
}
