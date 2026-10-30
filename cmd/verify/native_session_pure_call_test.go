package main

import (
	"encoding/json"
	"testing"
)

// Synthetic v2 consistency controls for the explicitly pinned276 producer.
// Native compatibility of the complete source/corpus remains a separate run.
func TestNativeSessionPureCallAuthoredIDsAndTampering(t *testing.T) {
	suites, err := pathAliasCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range suites {
		t.Run(original.Package, func(t *testing.T) {
			base, s, identity := pathAliasReceiptFixture(t, original, 0)
			identity.CompilerRevision = nativeSessionCompilerRevision
			pathAliasTestGeneration(t, base)["compiler_source_sha"] = identity.CompilerRevision
			v2, _, _ := nativeSessionInspectFixture(t, false)
			var template map[string]any
			if err := json.Unmarshal(v2, &template); err != nil {
				t.Fatal(err)
			}
			runtime := pathAliasTestMap(t, base, "result", "runtime")
			current := pathAliasTestMap(t, template, "result", "runtime")
			for _, key := range []string{"schema", "producer_source_sha", "elapsed_ns", "toolchain", "artifact", "executable_sha256", "build", "runs", "runtime_suite_sha256"} {
				runtime[key] = current[key]
			}
			activities := pathAliasTestMap(t, base, "result", "program", "pure_calls")["activities"].([]any)
			for _, row := range activities {
				item := row.(map[string]any)
				ref := item["activity"].(map[string]any)
				bodies, err := decimalPayloadBodies(identity.SourceBytes[item["source"].(string)])
				if err != nil {
					t.Fatal(err)
				}
				id := bodies[ref["activity"].(string)].ID
				if id == "" {
					t.Fatal("missing independent authored ID")
				}
				item["activity_id"], item["lowered_activity_id"] = id, id
			}
			if _, err := inspectSessionWithRuntime(mustJSON(t, base), s, false, identity, nil); err != nil {
				t.Fatal("source276 authored identities refused", err)
			}
			for i := range activities {
				for _, field := range []string{"activity_id", "lowered_activity_id"} {
					changed := pathAliasTestClone(t, base)
					rows := pathAliasTestMap(t, changed, "result", "program", "pure_calls")["activities"].([]any)
					rows[i].(map[string]any)[field] = "gooo://foreign-activity"
					if _, err := inspectSessionWithRuntime(mustJSON(t, changed), s, false, identity, nil); err == nil {
						t.Fatal("foreign authored pure-call identity accepted", i, field)
					}
				}
			}
		})
	}
	for _, revision := range []string{nativeSessionCompilerRevision[:7], nativeSessionCompilerRevision + "-modified"} {
		if _, err := selectPureCallIdentityProfile(revision); err == nil {
			t.Fatal("unqualified session producer accepted", revision)
		}
	}
}
