package revisionpolicy

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func allowedFacts(action string) original.APIFacts {
	return original.APIFacts{
		Facts:         original.Facts{GroupPresent: true, GroupValue: "admin", PrincipalPresent: true, PrincipalValue: "synthetic:user"},
		Authenticated: true, CallerTenant: "tenant-a", ResourceTenant: "tenant-a", RequestedAction: action, AllowedAction: action,
	}
}

// Literal expected observations describe synthetic facts, not provider accuracy.
func literalAllowedDecision() original.APIDecision {
	return original.APIDecision{
		Allowed: true, Reason: "ALLOW", Authenticated: true, TenantPresent: true, TenantMatched: true,
		ActionPresent: true, ActionMatched: true, AccessAllowed: true, AccessError: false,
		AccessResult: "ALLOW", GroupMatched: true, PrincipalMatched: false, GroupMissing: false,
		PrincipalDefaulted: false, EffectivePrincipal: "synthetic:user",
	}
}

func nativeReport(t *testing.T, facts original.APIFacts) ComparisonReport {
	t.Helper()
	baseline, err := Baseline(facts)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Compare(baseline)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestNativeReadonlyComparisonAllFifteenFieldsAndReplay(t *testing.T) {
	for _, action := range []string{"read", "write"} {
		t.Run(action, func(t *testing.T) {
			facts := allowedFacts(action)
			report := nativeReport(t, facts)
			before, after := literalAllowedDecision(), literalAllowedDecision()
			differences := DifferenceFlags{}
			summary := Summary{Classification: "UNCHANGED", ChangedCount: 0}
			if action == "write" {
				after.Allowed, after.Reason = false, "ACTION_NOT_READ_ONLY"
				differences = DifferenceFlags{Allowed: true, Reason: true}
				summary = Summary{Classification: "NEWLY_DENIED", ChangedCount: 2}
			}
			if report.Schema != ComparisonSchema || report.Status != ComparisonStatus || report.Scope != ComparisonScope ||
				report.BeforeProfile != OriginalProfile || report.CandidateProfile != CandidateProfile ||
				report.Before.Facts != facts || report.After.Facts != facts || report.Before.Decision != before ||
				report.After.Decision != after || report.Differences != differences || report.Summary != summary {
				t.Fatalf("literal readonly report mismatch: %+v", report)
			}
			decoded, err := DecodeComparisonReport(marshalTest(t, report))
			if err != nil {
				t.Fatal(err)
			}
			verified, err := VerifyComparison(decoded)
			if err != nil || !reflect.DeepEqual(verified, report) {
				t.Fatalf("complete replay mismatch: %v", err)
			}
			if _, err := original.VerifyAPIProof(report.After); !errors.Is(err, original.ErrAPIGenerationMismatch) {
				t.Fatalf("candidate was incorrectly accepted by original same-generation verifier: %v", err)
			}
		})
	}
}

func TestReadonlyPreservesBaseDenialPriorityAndAllFields(t *testing.T) {
	rows := []struct {
		name   string
		change func(*original.APIFacts)
		reason string
	}{
		{"authentication before tenant", func(f *original.APIFacts) { f.Authenticated = false; f.CallerTenant = "" }, "NOT_AUTHENTICATED"},
		{"empty tenant", func(f *original.APIFacts) { f.ResourceTenant = "" }, "TENANT_UNSPECIFIED"},
		{"mismatched tenant", func(f *original.APIFacts) { f.ResourceTenant = "other" }, "TENANT_MISMATCH"},
		{"empty action", func(f *original.APIFacts) { f.RequestedAction = "" }, "ACTION_UNSPECIFIED"},
		{"action mismatch", func(f *original.APIFacts) { f.AllowedAction = "other" }, "ACTION_MISMATCH"},
		{"CEL missing group", func(f *original.APIFacts) { f.GroupPresent = false }, "ACCESS_ERROR"},
		{"CEL denial", func(f *original.APIFacts) { f.GroupValue = "viewer" }, "ACCESS_DENIED"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			facts := allowedFacts("write")
			row.change(&facts)
			report := nativeReport(t, facts)
			if report.Before.Decision.Allowed || report.Before.Decision.Reason != row.reason ||
				report.After.Decision != report.Before.Decision || report.Before.Facts != facts || report.After.Facts != facts ||
				report.Differences != (DifferenceFlags{}) || report.Summary != (Summary{Classification: "UNCHANGED"}) {
				t.Fatalf("readonly changed a base denial: %+v", report)
			}
		})
	}
}

func TestCompareRejectsMixedUnknownProfilesAndUnverifiedOriginal(t *testing.T) {
	request, err := Baseline(allowedFacts("write"))
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*CompareRequest){
		func(r *CompareRequest) { r.BeforeProfile = "unknown" },
		func(r *CompareRequest) { r.CandidateProfile = "unknown" },
		func(r *CompareRequest) { r.BeforeProfile = CandidateProfile },
		func(r *CompareRequest) { r.CandidateProfile = OriginalProfile },
	} {
		current := request
		change(&current)
		if _, err := Compare(current); !errors.Is(err, ErrProfileMismatch) {
			t.Fatalf("mixed/unknown profile accepted: %v", err)
		}
	}
	request.Before.Decision.Reason = "forged"
	if _, err := Compare(request); !errors.Is(err, original.ErrAPIDecisionMismatch) {
		t.Fatalf("unverified original accepted: %v", err)
	}
}

func cloneReport(t *testing.T, report ComparisonReport) ComparisonReport {
	t.Helper()
	var cloned ComparisonReport
	if err := json.Unmarshal(marshalTest(t, report), &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func changeScalar(t *testing.T, value reflect.Value) {
	t.Helper()
	switch value.Kind() {
	case reflect.Bool:
		value.SetBool(!value.Bool())
	case reflect.String:
		value.SetString(value.String() + "!")
	case reflect.Int:
		value.SetInt(value.Int() + 1)
	default:
		t.Fatalf("unexpected scalar mutation kind %s", value.Kind())
	}
}

func TestEverySavedScalarAndBindingTamperRefuses(t *testing.T) {
	good := nativeReport(t, allowedFacts("write"))
	refuse := func(t *testing.T, current ComparisonReport) {
		t.Helper()
		if _, err := VerifyComparison(current); err == nil {
			t.Fatal("tampered complete comparison accepted")
		}
	}
	for _, field := range []string{"Schema", "Status", "Scope", "BeforeProfile", "CandidateProfile"} {
		t.Run(field, func(t *testing.T) {
			current := cloneReport(t, good)
			changeScalar(t, reflect.ValueOf(&current).Elem().FieldByName(field))
			refuse(t, current)
		})
	}
	for _, side := range []string{"Before", "After"} {
		for _, part := range []string{"Facts", "Decision"} {
			prototype := reflect.ValueOf(good).FieldByName(side).FieldByName(part)
			// APIFacts embeds Facts; recurse so all nine raw scalar inputs are covered.
			var leaves func(reflect.Value, []int)
			leaves = func(value reflect.Value, path []int) {
				for index := 0; index < value.NumField(); index++ {
					next := append(append([]int(nil), path...), index)
					if value.Field(index).Kind() == reflect.Struct {
						leaves(value.Field(index), next)
						continue
					}
					name := value.Type().Field(index).Tag.Get("json")
					t.Run(side+"/"+part+"/"+name, func(t *testing.T) {
						current := cloneReport(t, good)
						changed := reflect.ValueOf(&current).Elem().FieldByName(side).FieldByName(part).FieldByIndex(next)
						changeScalar(t, changed)
						refuse(t, current)
					})
				}
			}
			leaves(prototype, nil)
		}
	}
	for _, part := range []string{"Differences", "Summary"} {
		prototype := reflect.ValueOf(good).FieldByName(part)
		for index := 0; index < prototype.NumField(); index++ {
			t.Run(part+"/"+prototype.Type().Field(index).Name, func(t *testing.T) {
				current := cloneReport(t, good)
				changeScalar(t, reflect.ValueOf(&current).Elem().FieldByName(part).Field(index))
				refuse(t, current)
			})
		}
	}
	for _, target := range []string{"Before", "After", "ClassifierBindings"} {
		access := func(report *ComparisonReport) *original.Bindings {
			if target == "Before" {
				return &report.Before.Bindings
			}
			if target == "After" {
				return &report.After.Bindings
			}
			return &report.ClassifierBindings
		}
		for _, field := range []string{"CompilerSourceSHA", "GenerationSHA256"} {
			t.Run(target+"/"+field, func(t *testing.T) {
				current := cloneReport(t, good)
				changeScalar(t, reflect.ValueOf(access(&current)).Elem().FieldByName(field))
				refuse(t, current)
			})
		}
		for name := range access(&good).Assets {
			t.Run(target+"/asset/"+name, func(t *testing.T) {
				current := cloneReport(t, good)
				access(&current).Assets[name] = "sha256:" + strings.Repeat("0", 64)
				refuse(t, current)
			})
		}
		t.Run(target+"/missing-roster", func(t *testing.T) {
			current := cloneReport(t, good)
			access(&current).Assets = nil
			refuse(t, current)
		})
	}
}

func TestRawInactiveFactsAndConsistentSyntheticForgeryRemainExplicit(t *testing.T) {
	facts := allowedFacts("read")
	facts.GroupPresent, facts.GroupValue = false, "\U0001f600\x00\r\n"
	facts.PrincipalValue = "user:me@acme.co"
	report := nativeReport(t, facts)
	if report.Before.Facts != facts || report.After.Facts != facts || report.Summary != (Summary{Classification: "UNCHANGED"}) {
		t.Fatal("inactive raw fact was normalized or lost")
	}
	if _, err := VerifyComparison(report); err != nil {
		t.Fatal(err)
	}
	// An entirely synthetic new baseline is permitted by this unsigned contract.
	// Neither baseline nor comparison can attest a tenant/authentication event.
	facts.CallerTenant, facts.ResourceTenant = "fabricated-tenant", "fabricated-tenant"
	forged := nativeReport(t, facts)
	if _, err := VerifyComparison(forged); err != nil {
		t.Fatalf("consistent unsigned synthetic record unexpectedly rejected: %v", err)
	}
}
