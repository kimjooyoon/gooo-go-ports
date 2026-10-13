package nativepolicy

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func replaySample(t *testing.T) APIProof {
	t.Helper()
	p, err := EvaluateAPI(APIFacts{Facts: Facts{GroupPresent: true, GroupValue: "admin", PrincipalPresent: true, PrincipalValue: "synthetic:user"}, Authenticated: true, CallerTenant: "tenant", ResourceTenant: "tenant", RequestedAction: "read", AllowedAction: "read"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func cloneProof(t *testing.T, p APIProof) APIProof {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := DecodeAPIProof(raw)
	if err != nil {
		t.Fatal(err)
	}
	return copy
}

func TestSavedAllIndependentAPIObservations(t *testing.T) {
	raw, err := os.ReadFile("oracle/api-composition-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID       string      `json:"id"`
		Facts    APIFacts    `json:"facts"`
		Expected APIDecision `json:"expected"`
	}
	if json.Unmarshal(raw, &cases) != nil || len(cases) != 32 {
		t.Fatal("independent API cases")
	}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			original, err := EvaluateAPI(c.Facts)
			if err != nil {
				t.Fatal(err)
			}
			decoded := cloneProof(t, original)
			if !reflect.DeepEqual(original, decoded) {
				t.Fatal("save/decode changed facts or binding bytes")
			}
			replay, err := VerifyAPIProof(decoded)
			if err != nil || replay.Decision != c.Expected {
				t.Fatalf("saved decision differs from independent record: %+v %v", replay, err)
			}
			if replay.Schema != "gooo/api-proof-replay/v1" || replay.Status != "HISTORICALLY_CONSISTENT" || replay.Scope != "same_bound_source_generation" || replay.VerifiedDecisionFields != 15 || !sameBindings(replay.Bindings, original.Bindings) {
				t.Fatal("historical result contract")
			}
		})
	}
}

func TestEverySavedDecisionFieldTamperRejected(t *testing.T) {
	original := replaySample(t)
	value := reflect.ValueOf(original.Decision)
	for i := 0; i < value.NumField(); i++ {
		t.Run(value.Type().Field(i).Tag.Get("json"), func(t *testing.T) {
			changed := cloneProof(t, original)
			field := reflect.ValueOf(&changed.Decision).Elem().Field(i)
			if field.Kind() == reflect.Bool {
				field.SetBool(!field.Bool())
			} else {
				field.SetString(field.String() + "tampered")
			}
			if _, err := VerifyAPIProof(changed); !errors.Is(err, ErrAPIDecisionMismatch) {
				t.Fatalf("saved decision field accepted: %v", err)
			}
		})
	}
}

func TestEverySavedBindingAssetTamperRejected(t *testing.T) {
	original := replaySample(t)
	for _, name := range boundNames {
		t.Run(name, func(t *testing.T) {
			changed := cloneProof(t, original)
			changed.Bindings.Assets[name] = "sha256:" + strings.Repeat("0", 64)
			if _, err := VerifyAPIProof(changed); !errors.Is(err, ErrAPIGenerationMismatch) {
				t.Fatalf("saved asset digest accepted: %v", err)
			}
		})
	}
}

func TestSavedBindingIdentityAndRosterRejectedBeforeFacts(t *testing.T) {
	mutations := map[string]func(*APIProof){
		"compiler_pin":      func(p *APIProof) { p.Bindings.CompilerSourceSHA = strings.Repeat("0", 40) },
		"generation_digest": func(p *APIProof) { p.Bindings.GenerationSHA256 = "sha256:" + strings.Repeat("0", 64) },
		"missing_asset":     func(p *APIProof) { delete(p.Bindings.Assets, boundNames[0]) },
		"extra_asset":       func(p *APIProof) { p.Bindings.Assets["extra.go"] = "sha256:" + strings.Repeat("0", 64) },
		"source_generation_identity": func(p *APIProof) {
			p.Bindings.Assets["canonical/main.gooo"] = p.Bindings.Assets["canonical/scalar_access.gooo"]
		},
	}
	original := replaySample(t)
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := cloneProof(t, original)
			mutate(&changed)
			changed.Facts.GroupValue = string([]byte{0xff})
			if _, err := VerifyAPIProof(changed); !errors.Is(err, ErrAPIGenerationMismatch) {
				t.Fatalf("binding gate did not precede invalid facts: %v", err)
			}
		})
	}
}

func TestEveryActiveSavedFactMismatch(t *testing.T) {
	changes := map[string]func(*APIFacts){
		"group_present": func(f *APIFacts) { f.GroupPresent = false }, "group_value": func(f *APIFacts) { f.GroupValue = "reader" },
		"principal_present": func(f *APIFacts) { f.PrincipalPresent = false }, "principal_value": func(f *APIFacts) { f.PrincipalValue = "user:me@acme.co" },
		"authenticated": func(f *APIFacts) { f.Authenticated = false }, "caller_tenant": func(f *APIFacts) { f.CallerTenant = "other" },
		"resource_tenant": func(f *APIFacts) { f.ResourceTenant = "other" }, "requested_action": func(f *APIFacts) { f.RequestedAction = "write" },
		"allowed_action": func(f *APIFacts) { f.AllowedAction = "write" },
	}
	original := replaySample(t)
	for name, mutate := range changes {
		t.Run(name, func(t *testing.T) {
			p := cloneProof(t, original)
			mutate(&p.Facts)
			if _, err := VerifyAPIProof(p); !errors.Is(err, ErrAPIDecisionMismatch) {
				t.Fatalf("active fact change accepted: %v", err)
			}
		})
	}
}

func TestSavedAbsentPlaceholdersPreservedAndIgnored(t *testing.T) {
	p := replaySample(t)
	p.Facts.GroupPresent = false
	p.Facts.PrincipalPresent = false
	p.Facts.GroupValue = "ignored original group"
	p.Facts.PrincipalValue = "ignored original principal"
	p, err := EvaluateAPI(p.Facts)
	if err != nil {
		t.Fatal(err)
	}
	changed := cloneProof(t, p)
	changed.Facts.GroupValue = "admin"
	changed.Facts.PrincipalValue = "user:me@acme.co"
	decoded := cloneProof(t, changed)
	if decoded.Facts != changed.Facts {
		t.Fatal("ignored placeholders lost on save/decode")
	}
	r, err := VerifyAPIProof(decoded)
	if err != nil || r.Decision != p.Decision || r.Decision.AccessResult != "ERROR_NO_SUCH_KEY_GROUP" || r.Decision.EffectivePrincipal != "" {
		t.Fatalf("absent placeholder semantics changed: %+v %v", r, err)
	}
	for _, bad := range []string{strings.Repeat("x", MaxTextBytes+1), string([]byte{0xff})} {
		changed.Facts.GroupValue = bad
		if _, err := VerifyAPIProof(changed); err == nil {
			t.Fatal("ignored placeholder outside text domain accepted")
		}
	}
}

func TestConsistentSyntheticForgeryIsNotIssuerAuthentication(t *testing.T) {
	original := replaySample(t)
	changed := cloneProof(t, original)
	changed.Facts.Authenticated = false
	if _, err := VerifyAPIProof(changed); !errors.Is(err, ErrAPIDecisionMismatch) {
		t.Fatal("inconsistent forgery accepted")
	}
	forged, err := EvaluateAPI(changed.Facts)
	if err != nil {
		t.Fatal(err)
	}
	changed.Decision = forged.Decision
	r, err := VerifyAPIProof(changed)
	if err != nil || r.Decision.Allowed || r.Decision.Reason != "NOT_AUTHENTICATED" {
		t.Fatalf("consistent synthetic proof must remain consistent, not authenticated: %+v %v", r, err)
	}
}

func TestPreviousGenerationRejectedExplicitly(t *testing.T) {
	p := replaySample(t)
	raw, err := os.ReadFile("testdata/previous-generation.json")
	if err != nil {
		t.Fatal(err)
	}
	var old generation
	if json.Unmarshal(raw, &old) != nil || len(old.Assets) != 25 {
		t.Fatal("prior PR73 generation fixture")
	}
	p.Bindings = Bindings{CompilerSourceSHA: CompilerPin, Assets: old.Assets, GenerationSHA256: digest(raw)}
	if _, err := VerifyAPIProof(p); !errors.Is(err, ErrAPIGenerationMismatch) {
		t.Fatalf("prior source generation accepted: %v", err)
	}
}
