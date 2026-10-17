package revisionpolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"testing"
)

func revisionBindingFixture(t *testing.T) (map[string][]byte, []byte) {
	t.Helper()
	files := map[string][]byte{}
	err := fs.WalkDir(revisionAssets, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || name == "generation.json" {
			return nil
		}
		raw, err := revisionAssets.ReadFile(name)
		if err == nil {
			files[name] = append([]byte(nil), raw...)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := revisionAssets.ReadFile("generation.json")
	if err != nil {
		t.Fatal(err)
	}
	return files, append([]byte(nil), manifest...)
}

func bindingTestJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func mutateBindingJSON(t *testing.T, files map[string][]byte, name string, mutate func(map[string]any)) {
	t.Helper()
	value := bindingTestJSON(t, files[name])
	mutate(value)
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	files[name] = raw
	// Keep the outer raw-execution reference valid so mutations of execution
	// semantics still reach their intended inner predicate.
	for _, config := range revisionConfigs {
		if name == "receipts/"+config.prefix+"-execution.json" {
			replayName := "receipts/" + config.prefix + "-replay.json"
			replay := bindingTestJSON(t, files[replayName])
			replay["replayed_from_sha256"] = fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
			encoded, err := json.Marshal(replay)
			if err != nil {
				t.Fatal(err)
			}
			files[replayName] = encoded
		}
	}
}

// Resealing deliberately bypasses byte-hash failures. The checker must reject
// contradictory producer/source/native evidence inside an otherwise sealed set.
func resealBindingFixture(t *testing.T, files map[string][]byte, raw []byte) []byte {
	t.Helper()
	manifest := bindingTestJSON(t, raw)
	assets := map[string]string{}
	for name, content := range files {
		assets[name] = fmt.Sprintf("sha256:%x", sha256.Sum256(content))
	}
	manifest["assets"] = assets
	sealed, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func requireResealedBindingRefusal(t *testing.T, files map[string][]byte, manifest []byte) {
	t.Helper()
	if _, err := checkRevisionBindings(files, resealBindingFixture(t, files, manifest)); err == nil {
		t.Fatal("accepted contradictory evidence after asset hashes were correctly resealed")
	}
}

func bindingRuntime(receipt map[string]any) map[string]any {
	return receipt["result"].(map[string]any)["runtime"].(map[string]any)
}

func TestRevisionBindingsActualEmbeddedEvidence(t *testing.T) {
	files, manifest := revisionBindingFixture(t)
	actual, err := checkRevisionBindings(files, manifest)
	if err != nil {
		t.Fatal(err)
	}
	public, err := RevisionBindings()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 38 || len(actual.Assets) != 38 || !reflect.DeepEqual(actual, public) || actual.CompilerSourceSHA != CompilerPin || actual.GenerationSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(manifest)) {
		t.Fatal("embedded entrypoint did not preserve the complete checked source bindings")
	}
}

func TestRevisionBindingsRequireSealedOriginalModuleChecksums(t *testing.T) {
	const moduleVersion = "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy v0.0.0-20261014100709-8cddcc4e65af"
	expected := []byte(moduleVersion + " h1:fvrka7xThFrx1JkbTEp+lZ7hjcUsK4csF/XbpmF4VuM=\n" + moduleVersion + "/go.mod h1:8bu+v2g7hKzRE72QrBMPJRCTJRVvlk4pqwTaA4DQrtc=\n")
	files, manifest := revisionBindingFixture(t)
	if !bytes.Equal(files["go.sum"], expected) {
		t.Fatal("original module checksums differ from the verified public module")
	}
	if _, err := checkRevisionBindings(files, manifest); err != nil {
		t.Fatal("valid checksum fixture was refused", err)
	}
	for _, attack := range []string{"missing-file", "altered-file", "missing-generation-entry"} {
		t.Run(attack, func(t *testing.T) {
			files, manifest := revisionBindingFixture(t)
			switch attack {
			case "missing-file":
				delete(files, "go.sum")
			case "altered-file":
				files["go.sum"] = append(files["go.sum"], '\n')
			case "missing-generation-entry":
				generation := bindingTestJSON(t, manifest)
				delete(generation["assets"].(map[string]any), "go.sum")
				encoded, err := json.Marshal(generation)
				if err != nil {
					t.Fatal(err)
				}
				manifest = encoded
			}
			if _, err := checkRevisionBindings(files, manifest); err == nil {
				t.Fatal("accepted a missing or changed sealed checksum asset")
			}
		})
	}
}

func TestRevisionBindingsOptionalErrorExtension(t *testing.T) {
	for _, test := range []struct {
		name             string
		value            any
		present, allowed bool
	}{
		{"absent_v3_field", nil, false, true},
		{"null_extension", nil, true, true},
		{"empty_extension", []any{}, true, true},
		{"reported_failure", []any{"typecheck failed"}, true, false},
		{"malformed_empty_object", map[string]any{}, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			files, manifest := revisionBindingFixture(t)
			mutateBindingJSON(t, files, "receipts/candidate-body-codegen.json", func(body map[string]any) {
				report := body["report"].(map[string]any)
				if test.present {
					report["errors"] = test.value
				} else {
					delete(report, "errors")
				}
			})
			_, err := checkRevisionBindings(files, resealBindingFixture(t, files, manifest))
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%v, error=%v", test.allowed, err)
			}
		})
	}
}

func TestRevisionBindingsPreserveEveryProgramField(t *testing.T) {
	files, manifest := revisionBindingFixture(t)
	mutateBindingJSON(t, files, "receipts/candidate-execution.json", func(receipt map[string]any) {
		program := receipt["result"].(map[string]any)["program"].(map[string]any)
		program["unprojected_evidence"] = map[string]any{"source_binding": "changed only in execution"}
	})
	requireResealedBindingRefusal(t, files, manifest)
}

func TestRevisionBindingsRequireActualNativeCompletion(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing_completed", func(run map[string]any) { delete(run, "completed") }},
		{"missing_exit_code", func(run map[string]any) { delete(run, "exit_code") }},
		{"nonzero_exit", func(run map[string]any) { run["exit_code"] = json.Number("1") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			files, manifest := revisionBindingFixture(t)
			mutateBindingJSON(t, files, "receipts/candidate-execution.json", func(receipt map[string]any) {
				runs := bindingRuntime(receipt)["runs"].([]any)
				test.mutate(runs[0].(map[string]any))
			})
			requireResealedBindingRefusal(t, files, manifest)
		})
	}
}

func TestRevisionBindingsRejectDifferentNativeProducer(t *testing.T) {
	files, manifest := revisionBindingFixture(t)
	mutateBindingJSON(t, files, "receipts/candidate-execution.json", func(receipt map[string]any) {
		bindingRuntime(receipt)["producer_source_sha"] = "0000000000000000000000000000000000000000"
	})
	requireResealedBindingRefusal(t, files, manifest)
}

func TestRevisionBindingsRequireExactNativeProjection(t *testing.T) {
	files, manifest := revisionBindingFixture(t)
	files["internal/flags/generated.go"] = append(files["internal/flags/generated.go"], []byte("\n// Resealed projection differs from compiler output.\n")...)
	requireResealedBindingRefusal(t, files, manifest)
}

func TestRevisionBindingsRequireClassifierInputRecordIdentity(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"field_id", func(record map[string]any) {
			field := record["fields"].([]any)[0].(map[string]any)
			field["id"] = "gooo://api-example/decision/unrelated"
		}},
		{"missing_field", func(record map[string]any) {
			record["fields"] = record["fields"].([]any)[1:]
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			files, manifest := revisionBindingFixture(t)
			mutateBindingJSON(t, files, "receipts/flags-body-codegen.json", func(body map[string]any) {
				records := body["report"].(map[string]any)["record_types"].([]any)
				found := false
				for _, raw := range records {
					record := raw.(map[string]any)
					if record["name"] == "ApiDecision" {
						test.mutate(record)
						found = true
					}
				}
				if !found {
					t.Fatal("actual classifier fixture has no ApiDecision input record")
				}
			})
			requireResealedBindingRefusal(t, files, manifest)
		})
	}
}

func TestRevisionBindingsKeepOriginalProfileFrozen(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"raw_generation_identity", func(original map[string]any) {
			original["generation_sha256"] = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		}},
		{"original_asset_map", func(original map[string]any) {
			original["assets"].(map[string]any)["bindings.go"] = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			files, manifest := revisionBindingFixture(t)
			mutateBindingJSON(t, files, "profiles.json", func(profiles map[string]any) { test.mutate(profiles["original"].(map[string]any)) })
			requireResealedBindingRefusal(t, files, manifest)
		})
	}
}

// A fresh execution with the same program is not a saved replay. Each attack
// retains otherwise complete evidence and a correctly resealed outer manifest.
func TestRevisionBindingsRequireExactSavedReplayOrigin(t *testing.T) {
	for _, attack := range []string{"fresh-substitute", "wrong-origin", "missing-replay", "wrong-schema", "new-model-call"} {
		t.Run(attack, func(t *testing.T) {
			files, manifest := revisionBindingFixture(t)
			name := "receipts/candidate-replay.json"
			if attack == "fresh-substitute" {
				files[name] = append([]byte(nil), files["receipts/candidate-execution.json"]...)
			} else {
				mutateBindingJSON(t, files, name, func(receipt map[string]any) {
					result := receipt["result"].(map[string]any)
					replay := result["replay"].(map[string]any)
					switch attack {
					case "wrong-origin":
						receipt["replayed_from_sha256"] = "sha256:wrong"
					case "missing-replay":
						delete(result, "replay")
					case "wrong-schema":
						replay["schema"] = "other/replay/v1"
					case "new-model-call":
						replay["model_calls"] = 1
					}
				})
			}
			requireResealedBindingRefusal(t, files, manifest)
		})
	}
}
