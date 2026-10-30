package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func nativeSessionInspectFixture(t *testing.T, reused bool) ([]byte, suite, expectedIdentity) {
	t.Helper()
	raw, s, identity := fixture(t)
	identity.CompilerRevision = nativeSessionCompilerRevision
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	e.Result.Runtime.Schema = "gooo/body-composition-runtime/v2"
	e.Result.Runtime.Producer = identity.CompilerRevision
	e.Result.Runtime.Elapsed = json.RawMessage("100")
	e.Result.Runtime.Toolchain = json.RawMessage(`{"wall_ns":0}`)
	var document map[string]any
	if err := json.Unmarshal(mustJSON(t, e), &document); err != nil {
		t.Fatal(err)
	}
	runtime := document["result"].(map[string]any)["runtime"].(map[string]any)
	zero := 0
	process := nativeSessionProcess{Started: true, Completed: true, ExitCode: &zero, WallNS: 10,
		StdoutSHA256: contentDigest([]byte("synthetic output")), StderrSHA256: contentDigest(nil),
		Timing: &nativeSessionProcessTiming{StartNS: 1, WaitNS: 9, ContextAtStartReturn: "ACTIVE"}}
	executable := contentDigest([]byte("synthetic executable"))
	artifact := nativeSessionArtifact{Schema: "gooo/owned-native-artifact/v1", KeySHA256: contentDigest([]byte("synthetic key")),
		Reused: reused, ExecutableVerified: true, SourceBuild: process, MissReason: "empty"}
	if reused {
		artifact.MissReason = ""
		runtime["build"] = nativeSessionProcess{}
	} else {
		runtime["build"] = process
	}
	artifact.SourceBuildSHA256 = contentDigest(mustJSON(t, []any{artifact.KeySHA256, executable, process}))
	runtime["artifact"], runtime["executable_sha256"] = artifact, executable
	runtime["runs"] = []nativeSessionProcess{process, process}
	return mustJSON(t, document), s, identity
}

func nativeSessionInspectMutation(t *testing.T, raw []byte, change func(map[string]any, map[string]any)) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	change(document, document["result"].(map[string]any)["runtime"].(map[string]any))
	return mustJSON(t, document)
}

func TestNativeSessionInspectProfilesRemainSeparate(t *testing.T) {
	legacy, s, legacyIdentity := fixture(t)
	if _, err := inspect(legacy, s, false, legacyIdentity); err != nil {
		t.Fatal("original v1 fixture refused", err)
	}
	if _, err := inspectSessionWithRuntime(legacy, s, false, legacyIdentity, nil); err == nil {
		t.Fatal("legacy identity silently selected session support")
	}
	raw, s, identity := nativeSessionInspectFixture(t, false)
	if _, err := inspect(raw, s, false, identity); err == nil {
		t.Fatal("default v1 reader accepted v2")
	}
	for _, name := range []string{"default205", "other_producer", "other_go", "v1_schema"} {
		t.Run(name, func(t *testing.T) {
			current := identity
			bad := raw
			switch name {
			case "default205":
				current.CompilerRevision = "205896a413e5dea92366309c35ae85e542d055f6"
			case "other_producer":
				current.CompilerRevision = strings.Repeat("a", 40)
			case "other_go":
				current.GoVersion = "go1.27.2"
			case "v1_schema":
				bad = bytes.Replace(raw, []byte("gooo/body-composition-runtime/v2"), []byte("gooo/body-composition-runtime/v1"), 1)
			}
			var diagnostic nativeReportedRuntime
			if _, err := inspectSessionWithRuntime(bad, s, false, current, &diagnostic); err == nil || diagnostic.Status != "" {
				t.Fatal("unsupported session identity/schema accepted or leaked diagnostics", err, diagnostic)
			}
		})
	}
}

func TestNativeSessionInspectFreshMissAndReuse(t *testing.T) {
	for _, reason := range []string{"empty", "key_changed", "executable_changed_or_missing", "tool_dependencies_unbound", "reused"} {
		t.Run(reason, func(t *testing.T) {
			raw, s, identity := nativeSessionInspectFixture(t, reason == "reused")
			if reason != "reused" {
				raw = nativeSessionInspectMutation(t, raw, func(_ map[string]any, runtime map[string]any) {
					runtime["artifact"].(map[string]any)["miss_reason"] = reason
				})
			}
			var diagnostic nativeReportedRuntime
			if _, err := inspectSessionWithRuntime(raw, s, false, identity, &diagnostic); err != nil {
				t.Fatal("valid fresh miss/reuse refused", err)
			}
			wantBuild := int64(10)
			if reason == "reused" {
				wantBuild = 0
			}
			if diagnostic.Status != "REPORTED" || *diagnostic.BuildWallNS != wantBuild || *diagnostic.RunWallNS != [2]int64{10, 10} {
				t.Fatal("current diagnostics replaced by historical build", diagnostic)
			}
		})
	}
}

func TestNativeSessionInspectRejectsOwnershipAndCurrentRunRewrites(t *testing.T) {
	tests := []struct {
		name   string
		reused bool
		change func(map[string]any)
	}{
		{"missing_artifact", false, func(r map[string]any) { delete(r, "artifact") }},
		{"null_artifact", false, func(r map[string]any) { r["artifact"] = nil }},
		{"wrong_artifact_schema", false, func(r map[string]any) { r["artifact"].(map[string]any)["schema"] = "other" }},
		{"missing_verified", false, func(r map[string]any) { delete(r["artifact"].(map[string]any), "executable_verified") }},
		{"unverified_executable", false, func(r map[string]any) { r["artifact"].(map[string]any)["executable_verified"] = false }},
		{"malformed_key", false, func(r map[string]any) { r["artifact"].(map[string]any)["key_sha256"] = "not-a-digest" }},
		{"swapped_key", false, func(r map[string]any) {
			r["artifact"].(map[string]any)["key_sha256"] = contentDigest([]byte("other key"))
		}},
		{"swapped_executable", false, func(r map[string]any) { r["executable_sha256"] = contentDigest([]byte("other executable")) }},
		{"wrong_source_build_digest", false, func(r map[string]any) {
			r["artifact"].(map[string]any)["source_build_sha256"] = contentDigest([]byte("other build"))
		}},
		{"changed_historical_build", false, func(r map[string]any) { r["artifact"].(map[string]any)["source_build"].(map[string]any)["user_ns"] = 1 }},
		{"negative_wait", false, func(r map[string]any) { r["artifact"].(map[string]any)["wait_ns"] = -1 }},
		{"unknown_miss", false, func(r map[string]any) { r["artifact"].(map[string]any)["miss_reason"] = "pretend_hit" }},
		{"fresh_without_build", false, func(r map[string]any) { r["build"] = nativeSessionProcess{} }},
		{"fresh_other_current_build", false, func(r map[string]any) { r["build"].(map[string]any)["user_ns"] = 1 }},
		{"reuse_invents_build", true, func(r map[string]any) { r["build"] = r["artifact"].(map[string]any)["source_build"] }},
		{"reuse_with_miss", true, func(r map[string]any) { r["artifact"].(map[string]any)["miss_reason"] = "key_changed" }},
		{"missing_runs", true, func(r map[string]any) { delete(r, "runs") }},
		{"one_run", true, func(r map[string]any) { r["runs"] = r["runs"].([]any)[:1] }},
		{"third_run", true, func(r map[string]any) { r["runs"] = append(r["runs"].([]any), r["runs"].([]any)[0]) }},
		{"current_run_unstarted", true, func(r map[string]any) { r["runs"].([]any)[1].(map[string]any)["started"] = false }},
		{"current_run_incomplete", true, func(r map[string]any) { r["runs"].([]any)[1].(map[string]any)["completed"] = false }},
		{"current_run_failed", true, func(r map[string]any) { r["runs"].([]any)[1].(map[string]any)["exit_code"] = 1 }},
		{"current_run_no_exit", true, func(r map[string]any) { r["runs"].([]any)[1].(map[string]any)["exit_code"] = nil }},
		{"current_run_canceled", true, func(r map[string]any) { r["runs"].([]any)[1].(map[string]any)["canceled"] = true }},
		{"current_run_timeout", true, func(r map[string]any) { r["runs"].([]any)[1].(map[string]any)["timed_out"] = true }},
		{"current_run_truncated", true, func(r map[string]any) { r["runs"].([]any)[1].(map[string]any)["output_truncated"] = true }},
		{"current_run_diagnostics", true, func(r map[string]any) { r["runs"].([]any)[1].(map[string]any)["diagnostics_bytes"] = 1 }},
		{"missing_process_status", true, func(r map[string]any) { delete(r["runs"].([]any)[1].(map[string]any), "canceled") }},
		{"unbound_extra_process_field", false, func(r map[string]any) {
			r["artifact"].(map[string]any)["source_build"].(map[string]any)["extra"] = true
		}},
		{"case_folded_process_field", true, func(r map[string]any) {
			p := r["runs"].([]any)[1].(map[string]any)
			p["Completed"] = p["completed"]
			delete(p, "completed")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, s, identity := nativeSessionInspectFixture(t, tt.reused)
			raw = nativeSessionInspectMutation(t, raw, func(_ map[string]any, runtime map[string]any) { tt.change(runtime) })
			diagnostic := nativeReportedRuntime{Status: "unchanged"}
			if _, err := inspectSessionWithRuntime(raw, s, false, identity, &diagnostic); err == nil || diagnostic.Status != "unchanged" {
				t.Fatal("invalid ownership/current work accepted or leaked diagnostics", err, diagnostic)
			}
		})
	}
	raw, s, identity := nativeSessionInspectFixture(t, false)
	raw = bytes.Replace(raw, []byte(`"executable_verified":true`), []byte(`"executable_verified":true,"executable_verified":true`), 1)
	if _, err := inspectSessionWithRuntime(raw, s, false, identity, nil); err == nil {
		t.Fatal("duplicate ownership field bypassed the common duplicate gate")
	}
}

func TestNativeSessionInspectPreservesSemanticAndReplayChecks(t *testing.T) {
	raw, s, identity := nativeSessionInspectFixture(t, false)
	for _, name := range []string{"oracle", "expected", "input", "source", "manifest", "cases", "entry", "generated", "projection", "producer"} {
		t.Run(name, func(t *testing.T) {
			bad := nativeSessionInspectMutation(t, raw, func(document, runtime map[string]any) {
				result := document["result"].(map[string]any)
				delivery := runtime["traces"].([]any)[0].(map[string]any)["deliveries"].([]any)[0].(map[string]any)
				switch name {
				case "oracle":
					delivery["actual"] = 0
				case "expected":
					delivery["expected"] = 0
				case "input":
					delivery["inputs"].([]any)[0].(map[string]any)["value"] = 55296
				case "source":
					result["program"].(map[string]any)["workspace_image"].(map[string]any)["packages"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)["source_digest"] = contentDigest([]byte("other source"))
				case "manifest":
					document["manifest_digest"] = contentDigest([]byte("other manifest"))
				case "cases":
					document["cases_digest"] = contentDigest([]byte("other cases"))
				case "entry":
					result["program"].(map[string]any)["entry"].(map[string]any)["activity"] = "Other"
				case "generated":
					runtime["generated_sha256"] = contentDigest([]byte("other generated"))
				case "projection":
					runtime["projection_replayed"] = false
				case "producer":
					runtime["producer_source_sha"] = strings.Repeat("a", 40)
				}
			})
			if _, err := inspectSessionWithRuntime(bad, s, false, identity, nil); err == nil {
				t.Fatal("common semantic check bypassed", name)
			}
		})
	}
	original, err := inspectSessionWithRuntime(raw, s, false, identity, nil)
	if err != nil {
		t.Fatal(err)
	}
	parent := contentDigest(raw)
	replayed := nativeSessionInspectMutation(t, raw, func(document, _ map[string]any) {
		document["replayed_from_sha256"] = parent
		document["result"].(map[string]any)["replay"] = map[string]any{"model_calls": 0}
	})
	replay, err := inspectSessionWithRuntime(replayed, s, true, identity, nil)
	if err != nil || inspectReplayBinding(original, replay, parent) != nil {
		t.Fatal("valid own-parent replay refused", err)
	}
	if inspectReplayBinding(original, replay, contentDigest([]byte("other batch receipt"))) == nil {
		t.Fatal("swapped saved-parent bytes accepted")
	}
	if _, err := inspectSessionWithRuntime(replayed, s, false, identity, nil); err == nil {
		t.Fatal("replay accepted as first execution")
	}
	if _, err := inspectSessionWithRuntime(raw, s, true, identity, nil); err == nil {
		t.Fatal("first execution accepted as saved replay")
	}
}

func TestNativeSessionSourceBuildEncodingMatchesAccepted276Capture(t *testing.T) {
	// Small historical binding extracted from the saved accepted276 response
	// SHA256 560ea4ae76e5c4693d0ed290a57341af2aaae4d0bbce1d707809f4c81508c1dd.
	// This checks wire encoding only; it is not a new execution or authentication.
	const sourceBuild = `{"started":true,"completed":true,"canceled":false,"timed_out":false,"diagnostics_bytes":0,"output_truncated":false,"exit_code":0,"wall_ns":423873542,"user_ns":206729000,"system_ns":106151000,"peak_rss_bytes":89178112,"stdout_sha256":"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","stderr_sha256":"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","timing":{"start_ns":958542,"wait_ns":422915000,"deadline_remaining_ns":59937433334,"context_at_start_return":"ACTIVE"}}`
	p, err := decodeNativeSessionProcess([]byte(sourceBuild))
	if err != nil || successfulNativeSessionProcess(p) != nil {
		t.Fatal("historical successful build refused", err)
	}
	bound := mustJSON(t, []any{"sha256:f36ac8b6bcb6441414f4d720c997bbfe934ed9d95aa8af6389b5fefbdb6811fd", "sha256:748ac0cb2275f45dbe815d4fd8ec2df403f7118958fbb67dd915d76d95f6c9d8", p})
	if contentDigest(bound) != "sha256:98a763dd2be84845be9e5266a088a0d6a24f8451f5dc25719de47ca7f29cc6e8" {
		t.Fatal("accepted276 process field order/encoding changed", string(bound))
	}
}
