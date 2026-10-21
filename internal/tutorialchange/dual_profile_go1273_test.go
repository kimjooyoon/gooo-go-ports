package tutorialchange

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func currentGo1273Records(t *testing.T) [4][]byte {
	t.Helper()
	var out [4][]byte
	for i, name := range []string{"before.json", "after.json", "replay.json", "compiler-build.json"} {
		var err error
		out[i], err = currentProfileFiles.ReadFile("testdata/go1273/" + name)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// Replace the last exact named string scalar without reordering JSON bytes.
// Runtime follows composition in the retained CLI order; duplicate source-hash
// keys therefore select the runtime coordinate, checked again by the test.
func currentScalar(t *testing.T, raw []byte, key, before, after string) []byte {
	t.Helper()
	marker := []byte(`"` + key + `":`)
	if bytes.Count(raw, marker) == 0 {
		t.Fatalf("missing scalar %s", key)
	}
	at := bytes.LastIndex(raw, marker) + len(marker)
	for at < len(raw) && bytes.ContainsRune([]byte(" \n\r\t"), rune(raw[at])) {
		at++
	}
	old, _ := json.Marshal(before)
	next, _ := json.Marshal(after)
	if !bytes.HasPrefix(raw[at:], old) {
		t.Fatalf("original scalar differs: %s", key)
	}
	changed := append([]byte{}, raw[:at]...)
	changed = append(changed, next...)
	return append(changed, raw[at+len(old):]...)
}

func TestCurrentGo1273ActualRecordsAndGoTypedResultReplayParent(t *testing.T) {
	raw := currentGo1273Records(t)
	summary, err := Check(raw[0], raw[1], raw[2], raw[3])
	if err != nil {
		t.Fatal(err)
	}
	if summary.Profile != "learn-gooo/threshold-60-to-80/go1273" || len(summary.Cases) != 4 {
		t.Fatalf("wrong current profile or case count: %+v", summary)
	}
	if summary.ReceiptCompiler["source_revision"] != "205896a413e5dea92366309c35ae85e542d055f6" || summary.ReceiptCompiler["go_version"] != "go version go1.27.3 darwin/arm64" {
		t.Fatalf("wrong actual producer: %#v", summary.ReceiptCompiler)
	}
	p, err := selectEvidenceProfile(raw[3])
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := pinnedResultBytesForProfile(raw[1], p.resultProfile, p.compiler)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := decode(raw[2])
	if err != nil {
		t.Fatal(err)
	}
	expected := stringAt(object(replay), "result", "replay", "prior_result_sha256")
	if digest(encoded) != expected || summary.ReplayTypedResultParent != expected {
		t.Fatalf("Go typed Result bytes do not bind actual replay: got=%s wanted=%s", digest(encoded), expected)
	}
	if summary.ReplayRawParent != digest(raw[1]) {
		t.Fatal("actual raw after parent is not bound")
	}
	t.Logf("actual Go typed Result bytes=%d SHA=%s; separate raw after SHA=%s", len(encoded), digest(encoded), digest(raw[1]))
}

func TestDualProfileCrossRecordAndBuildMixIsRefused(t *testing.T) {
	old := originals(t)
	current := currentGo1273Records(t)
	for _, side := range []bool{false, true} {
		for slot := 0; slot < 4; slot++ {
			name := "legacy-base"
			raw, other := old, current
			if side {
				name = "current-base"
				raw, other = current, old
			}
			raw[slot] = other[slot]
			t.Run(name+"/"+[]string{"before", "after", "replay", "build"}[slot], func(t *testing.T) {
				if _, err := Check(raw[0], raw[1], raw[2], raw[3]); err == nil {
					t.Fatal("accepted cross-profile record or build")
				}
			})
		}
	}
}

func TestCurrentGo1273TamperedProducerAndSourceOriginAreRefused(t *testing.T) {
	current := currentGo1273Records(t)
	for _, key := range []string{"producer_source_sha", "original_source_sha256"} {
		t.Run(key, func(t *testing.T) {
			raw := current
			value, err := decode(raw[1])
			if err != nil {
				t.Fatal(err)
			}
			before := stringAt(object(value), "result", "runtime", key)
			raw[1] = currentScalar(t, raw[1], key, before, "sha256:"+strings.Repeat("0", 64))
			changed, err := decode(raw[1])
			if err != nil {
				t.Fatal(err)
			}
			if stringAt(object(changed), "result", "runtime", key) == before {
				t.Fatal("test did not change selected runtime origin")
			}
			if key == "original_source_sha256" && stringAt(object(changed), "result", "composition", key) != stringAt(object(value), "result", "composition", key) {
				t.Fatal("test changed other source coordinate")
			}
			if _, err := Check(raw[0], raw[1], raw[2], raw[3]); err == nil {
				t.Fatal("accepted changed origin")
			}
		})
	}
}

func TestCurrentGo1273RawAndTypedReplayParentsAreIndependentlyRefused(t *testing.T) {
	current := currentGo1273Records(t)
	for _, key := range []string{"replayed_from_sha256", "prior_result_sha256"} {
		t.Run(key, func(t *testing.T) {
			raw := current
			value, err := decode(raw[2])
			if err != nil {
				t.Fatal(err)
			}
			before := stringAt(object(value), key)
			wanted := "raw parent"
			if key == "prior_result_sha256" {
				before = stringAt(object(value), "result", "replay", key)
				wanted = "typed Result parent"
			}
			raw[2] = currentScalar(t, raw[2], key, before, "sha256:"+strings.Repeat("0", 64))
			_, err = Check(raw[0], raw[1], raw[2], raw[3])
			if err == nil || !strings.Contains(err.Error(), wanted) {
				t.Fatalf("wanted independent %s refusal, got %v", wanted, err)
			}
		})
	}
}

func TestCurrentGo1273UnknownDirtyBuildAndWrongTypedProfileAreRefused(t *testing.T) {
	current := currentGo1273Records(t)
	for _, key := range []string{"compiler_source_sha", "vcs_modified"} {
		t.Run(key, func(t *testing.T) {
			raw := current
			before, after := "205896a413e5dea92366309c35ae85e542d055f6", strings.Repeat("0", 40)
			if key == "vcs_modified" {
				before, after = "false", "true"
			}
			raw[3] = currentScalar(t, raw[3], key, before, after)
			_, err := Check(raw[0], raw[1], raw[2], raw[3])
			if err == nil || !strings.Contains(err.Error(), "compiler-build") {
				t.Fatalf("accepted unsupported build: %v", err)
			}
		})
	}
	p, err := selectEvidenceProfile(current[3])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pinnedResultBytesForProfile(current[1], p.resultProfile, compilerRevision); err == nil {
		t.Fatal("accepted mismatched typed profile compiler identity")
	}
}
