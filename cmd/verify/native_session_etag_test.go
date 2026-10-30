package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// This opt-in reads the immutable failed pair in place; no compiler, replay,
// process owner or copied private receipt is created by these offline checks.
func nativeSessionHTTPOriginalPair(t *testing.T) ([]byte, []byte, suite, expectedIdentity) {
	t.Helper()
	batch := os.Getenv("GOOO_NATIVE_SESSION_HTTP_RECORDS")
	if batch == "" {
		t.Skip("set GOOO_NATIVE_SESSION_HTTP_RECORDS to the original ScanETag batch000 directory for offline inspection only")
	}
	if !filepath.IsAbs(batch) || runtime.Version() != nativeSessionProfileGoVersion {
		t.Fatal("absolute original evidence path and pinned Go1.27.3 required")
	}
	read := func(name, expected string) []byte {
		t.Helper()
		raw, err := readNativeSessionEvidenceFile(filepath.Join(batch, name), nativeSessionEvidenceDecodedLimit)
		if err != nil || contentDigest(raw) != expected {
			t.Fatalf("immutable original %s differs: %v", name, err)
		}
		return raw
	}
	execution := read("execution.json", "sha256:575f096803e1c5149e44b5b447b1b0f6122f7e233c234962ca7d845e3fa59f7a")
	replay := read("replay.json", "sha256:ee1c36adbf4b00cb4f5c11c128d22150b34e2e6f8c3fe03fb933358ed03776cd")
	cases, err := readNativeSessionEvidenceFile(filepath.Join(batch, "cases.json"), nativeSessionEvidenceDecodedLimit)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := readNativeSessionEvidenceFile(filepath.Join(batch, "gooo.workspace.json"), nativeSessionEvidenceDecodedLimit)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var lock upstream
	lockBytes, err := os.ReadFile(filepath.Join(repository, "upstream.json"))
	if err != nil || json.Unmarshal(lockBytes, &lock) != nil || lock.CompilerRevision != uuidCompactCompilerRevision || lock.GoVersion != nativeSessionProfileGoVersion {
		t.Fatal("original205 oracle lock must remain unchanged", err)
	}
	suites, err := etagCorpus(repository)
	if err != nil {
		t.Fatal(err)
	}
	s := suites[0]
	if s.Package != etagPortPackage || s.Activity != etagScanActivity || len(s.Cases) != etagScanCases {
		t.Fatal("original192 ScanETag cases required")
	}
	catalog, err := freezeVerificationCatalog(filepath.Join(repository, "verification-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := catalog.workspaceForSuite(s)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := nativeCaseProfile("").policy(s, selected, lock)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := policy.prepare(s, 0)
	if err != nil || prepared.End != 55 {
		t.Fatal("original first55-case partition changed", prepared.End, err)
	}
	s.Cases = s.Cases[:prepared.End]
	if !bytes.Equal(prepared.Raw, cases) {
		t.Fatal("saved cases differ from unchanged Go oracle and original writer")
	}
	expectedManifest, err := json.MarshalIndent(selected.Manifest, "", "  ")
	if err != nil || !bytes.Equal(append(expectedManifest, '\n'), manifest) {
		t.Fatal("original selected workspace manifest changed", err)
	}
	for _, name := range selected.Names {
		copied, err := readNativeSessionEvidenceFile(filepath.Join(batch, name), nativeSessionEvidenceDecodedLimit)
		if err != nil || !bytes.Equal(copied, selected.Sources[name]) {
			t.Fatal("copied original source differs", name, err)
		}
	}
	identity := expectedIdentity{CompilerRevision: nativeSessionCompilerRevision, GoVersion: nativeSessionProfileGoVersion,
		ManifestDigest: contentDigest(manifest), CasesDigest: contentDigest(cases), Sources: selected.Identities, SourceBytes: selected.Sources}
	return execution, replay, s, identity
}

func TestNativeSessionHTTPOriginalScanETagPair(t *testing.T) {
	execution, replay, s, identity := nativeSessionHTTPOriginalPair(t)
	original, err := inspectSessionWithRuntime(execution, s, false, identity, nil)
	if err != nil {
		t.Fatal("immutable actual execute rejected by full inspector", err)
	}
	checkedReplay, err := inspectSessionWithRuntime(replay, s, true, identity, nil)
	if err != nil {
		t.Fatal("immutable actual replay rejected by full inspector", err)
	}
	if err := inspectReplayBinding(original, checkedReplay, contentDigest(execution)); err != nil {
		t.Fatal(err)
	}
	if err := inspectNativeSessionReplaySuite(execution, replay); err != nil {
		t.Fatal(err)
	}
}

// Small synthetic process metadata is always exercised without private
// evidence. These observations do not claim a producer or native execution.
func nativeSessionHTTPProcessFixture(t *testing.T, artifactReused, toolchainReused bool) []byte {
	t.Helper()
	raw, _, _ := nativeSessionInspectFixture(t, artifactReused)
	return nativeSessionInspectMutation(t, raw, func(_ map[string]any, r map[string]any) {
		version := "go version go1.27.3 synthetic/amd64"
		output := []byte(version + "\n")
		zero := 0
		check := nativeSessionProcess{Started: true, Completed: true, ExitCode: &zero, WallNS: 17,
			StdoutSHA256: contentDigest(output), StderrSHA256: contentDigest(nil),
			Timing: &nativeSessionProcessTiming{StartNS: 1, WaitNS: 16, ContextAtStartReturn: "ACTIVE"}}
		reference := nativeSessionToolchainReference{Schema: "gooo/owned-go-toolchain/v1", KeySHA256: contentDigest([]byte("synthetic toolchain key")),
			GoToolSHA256: contentDigest([]byte("synthetic Go bytes")), GoVersion: version,
			Reused: toolchainReused, BytesVerified: true, BuildInfoVerified: true, SourceCheck: check, SourceOutput: output}
		reference.SourceCheckSHA256 = contentDigest(mustJSON(t, []any{reference.KeySHA256, reference.GoToolSHA256, version, check, output}))
		r["go_version"], r["go_tool_selection"], r["go_tool_sha256"] = version, "explicit", reference.GoToolSHA256
		r["toolchain_reference"] = reference
		if toolchainReused {
			r["toolchain"] = nativeSessionProcess{}
		} else {
			r["toolchain"] = check
		}
	})
}

func nativeSessionHTTPMutate(t *testing.T, raw []byte, change func(map[string]any)) []byte {
	t.Helper()
	return nativeSessionInspectMutation(t, raw, func(_ map[string]any, r map[string]any) { change(r) })
}

func nativeSessionHTTPRebindCheck(t *testing.T, r map[string]any) {
	t.Helper()
	reference := r["toolchain_reference"].(map[string]any)
	check, err := decodeNativeSessionProcess(mustJSON(t, reference["source_check"]))
	if err != nil {
		t.Fatal(err)
	}
	var output []byte
	if err := json.Unmarshal(mustJSON(t, reference["source_output"]), &output); err != nil {
		t.Fatal(err)
	}
	reference["source_check_sha256"] = contentDigest(mustJSON(t, []any{reference["key_sha256"], reference["go_tool_sha256"], reference["go_version"], check, output}))
}

func nativeSessionHTTPSetSourceOutput(t *testing.T, r map[string]any, output []byte) {
	t.Helper()
	reference := r["toolchain_reference"].(map[string]any)
	// Marshal/unmarshal preserves the actual []byte wire representation rather
	// than supplying a text value to a field whose JSON encoding is base64.
	var encoded any
	if err := json.Unmarshal(mustJSON(t, output), &encoded); err != nil {
		t.Fatal(err)
	}
	reference["source_output"] = encoded
	reference["source_check"].(map[string]any)["stdout_sha256"] = contentDigest(output)
	nativeSessionHTTPRebindCheck(t, r)
}

func TestNativeSessionHTTPProcessReuseFlagsAreIndependent(t *testing.T) {
	for _, artifact := range []bool{false, true} {
		for _, toolchain := range []bool{false, true} {
			t.Run(fmt.Sprintf("artifact_%t_toolchain_%t", artifact, toolchain), func(t *testing.T) {
				if err := inspectNativeSessionETagProcesses(nativeSessionHTTPProcessFixture(t, artifact, toolchain)); err != nil {
					t.Fatal("independent fresh/reused roles refused", err)
				}
			})
		}
	}
	// Whitespace belongs to the original stdout digest even though version
	// selection uses its trimmed value.
	raw := nativeSessionHTTPProcessFixture(t, true, true)
	raw = nativeSessionHTTPMutate(t, raw, func(r map[string]any) {
		nativeSessionHTTPSetSourceOutput(t, r, []byte(" \tgo version go1.27.3 synthetic/amd64\n\n"))
	})
	if err := inspectNativeSessionETagProcesses(raw); err != nil {
		t.Fatal("trimmed version with correctly bound original output refused", err)
	}
}

func TestNativeSessionHTTPProcessRejectsToolchainRewrites(t *testing.T) {
	base := nativeSessionHTTPProcessFixture(t, true, true)
	ref := func(r map[string]any) map[string]any { return r["toolchain_reference"].(map[string]any) }
	tests := []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{"missing_reference", func(r map[string]any) { delete(r, "toolchain_reference") }, "toolchain reference"},
		{"wrong_schema", func(r map[string]any) { ref(r)["schema"] = "other" }, "bound toolchain identity"},
		{"malformed_key", func(r map[string]any) { ref(r)["key_sha256"] = "bad" }, "bound toolchain identity"},
		{"changed_bound_key", func(r map[string]any) { ref(r)["key_sha256"] = contentDigest([]byte("foreign key")) }, "historical toolchain digest"},
		{"different_tool_sha", func(r map[string]any) { ref(r)["go_tool_sha256"] = contentDigest([]byte("different binary")) }, "bound toolchain identity"},
		{"different_reference_version", func(r map[string]any) { ref(r)["go_version"] = "go version go1.27.2 synthetic/amd64" }, "bound toolchain identity"},
		{"bytes_not_verified", func(r map[string]any) { ref(r)["bytes_verified"] = false }, "bound toolchain identity"},
		{"build_info_not_verified", func(r map[string]any) { ref(r)["build_info_verified"] = false }, "bound toolchain identity"},
		{"unknown_reference_field", func(r map[string]any) { ref(r)["Reused"] = true }, "unsupported field"},
		{"missing_source_check_field", func(r map[string]any) { delete(ref(r)["source_check"].(map[string]any), "started") }, "missing or null field"},
		{"unknown_source_check_field", func(r map[string]any) { ref(r)["source_check"].(map[string]any)["Started"] = true }, "unsupported field"},
		{"failed_source_check", func(r map[string]any) {
			ref(r)["source_check"].(map[string]any)["completed"] = false
			nativeSessionHTTPRebindCheck(t, r)
		}, "historical toolchain completion"},
		{"source_check_stderr", func(r map[string]any) {
			ref(r)["source_check"].(map[string]any)["stderr_sha256"] = contentDigest([]byte("diagnostic"))
			nativeSessionHTTPRebindCheck(t, r)
		}, "historical toolchain completion"},
		{"bad_base64_output", func(r map[string]any) { ref(r)["source_output"] = "%%%" }, "toolchain reference"},
		{"trim_equivalent_output_without_stdout_binding", func(r map[string]any) {
			ref(r)["source_output"] = base64.StdEncoding.EncodeToString([]byte(" \tgo version go1.27.3 synthetic/amd64\n"))
			nativeSessionHTTPRebindCheck(t, r)
		}, "output/version binding"},
		{"different_output_version", func(r map[string]any) {
			nativeSessionHTTPSetSourceOutput(t, r, []byte("go version go1.27.2 synthetic/amd64\n"))
		}, "output/version binding"},
		{"oversized_bound_output", func(r map[string]any) {
			nativeSessionHTTPSetSourceOutput(t, r, []byte(strings.Repeat(" ", 256)+"go version go1.27.3 synthetic/amd64\n"))
		}, "output/version binding"},
		{"bad_source_check_digest", func(r map[string]any) { ref(r)["source_check_sha256"] = contentDigest([]byte("foreign check")) }, "historical toolchain digest"},
		{"reused_invents_current_work", func(r map[string]any) { r["toolchain"] = ref(r)["source_check"] }, "invents current work"},
		{"fresh_without_current_work", func(r map[string]any) { ref(r)["reused"] = false }, "fresh session HTTP toolchain"},
		{"fresh_different_current_work", func(r map[string]any) {
			ref(r)["reused"] = false
			var current map[string]any
			if err := json.Unmarshal(mustJSON(t, ref(r)["source_check"]), &current); err != nil {
				t.Fatal(err)
			}
			current["user_ns"] = float64(1)
			r["toolchain"] = current
		}, "fresh session HTTP toolchain"},
		{"zero_wall_with_coherent_timing", func(r map[string]any) {
			check := ref(r)["source_check"].(map[string]any)
			check["wall_ns"] = float64(0)
			check["timing"].(map[string]any)["start_ns"] = float64(0)
			check["timing"].(map[string]any)["wait_ns"] = float64(0)
			process, err := decodeNativeSessionProcess(mustJSON(t, check))
			if err != nil || successfulNativeSessionProcess(process) != nil {
				t.Fatal("generic zero-wall control must remain valid", err)
			}
			nativeSessionHTTPRebindCheck(t, r)
		}, "positive wall time"},
		{"platform_without_slash", func(r map[string]any) {
			r["go_version"] = "go version go1.27.3 synthetic-amd64"
			ref(r)["go_version"] = r["go_version"]
			nativeSessionHTTPSetSourceOutput(t, r, []byte(r["go_version"].(string)+"\n"))
		}, "runtime/toolchain profile"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := nativeSessionHTTPMutate(t, base, test.change)
			if err := inspectNativeSessionETagProcesses(raw); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatal("rewrite missed intended refusal", err, test.want)
			}
		})
	}
}

func TestNativeSessionHTTPProcessKeepsOwnershipAndFreshRunChecks(t *testing.T) {
	base := nativeSessionHTTPProcessFixture(t, true, true)
	for _, name := range []string{"missing_artifact", "unverified_artifact", "changed_source_build", "missing_runs", "one_run", "failed_run", "canceled_run", "truncated_run"} {
		t.Run(name, func(t *testing.T) {
			raw := nativeSessionHTTPMutate(t, base, func(r map[string]any) {
				switch name {
				case "missing_artifact":
					delete(r, "artifact")
				case "unverified_artifact":
					r["artifact"].(map[string]any)["executable_verified"] = false
				case "changed_source_build":
					r["artifact"].(map[string]any)["source_build_sha256"] = contentDigest([]byte("foreign build"))
				case "missing_runs":
					delete(r, "runs")
				case "one_run":
					r["runs"] = r["runs"].([]any)[:1]
				case "failed_run":
					r["runs"].([]any)[0].(map[string]any)["exit_code"] = float64(1)
				case "canceled_run":
					r["runs"].([]any)[0].(map[string]any)["canceled"] = true
				case "truncated_run":
					r["runs"].([]any)[0].(map[string]any)["output_truncated"] = true
				}
			})
			if err := inspectNativeSessionETagProcesses(raw); err == nil || !strings.Contains(err.Error(), "session HTTP ownership") {
				t.Fatal("HTTP profile bypassed artifact/current-run validation", err)
			}
		})
	}
}

func TestNativeSessionHTTPOriginalV1GuardAndStdoutRemainStrict(t *testing.T) {
	execution, replay, s, identity := nativeSessionHTTPOriginalPair(t)
	// A synthetic v1 metadata projection is only a reader control. Original
	// actual276 receipt bytes remain untouched and are not relabeled as205 runs.
	projectV1 := func(raw []byte) []byte {
		t.Helper()
		return nativeSessionInspectMutation(t, raw, func(doc map[string]any, r map[string]any) {
			r["schema"], r["producer_source_sha"] = "gooo/body-composition-runtime/v1", uuidCompactCompilerRevision
			composition := doc["result"].(map[string]any)["composition"].(map[string]any)
			report := composition["steps"].([]any)[0].(map[string]any)["generation"].(map[string]any)["report"].(map[string]any)
			report["compiler_source_sha"] = uuidCompactCompilerRevision
		})
	}
	legacyIdentity := identity
	legacyIdentity.CompilerRevision = uuidCompactCompilerRevision
	if _, err := inspect(projectV1(execution), s, false, legacyIdentity); err != nil {
		t.Fatal("fresh v1 reader control rejected", err)
	}
	if _, err := inspect(projectV1(replay), s, true, legacyIdentity); err == nil || !strings.Contains(err.Error(), "etag internal native process completion") {
		t.Fatal("default v1 lost its zero-current-work rejection", err)
	}
	changedStdout := nativeSessionHTTPMutate(t, replay, func(r map[string]any) {
		r["runs"].([]any)[0].(map[string]any)["stdout_sha256"] = contentDigest([]byte("different valid digest"))
	})
	if _, err := inspectSessionWithRuntime(changedStdout, s, true, identity, nil); err == nil || !strings.Contains(err.Error(), "etag independent Go matrix raw stdout digest") {
		t.Fatal("session hook bypassed independent Go stdout", err)
	}
}
