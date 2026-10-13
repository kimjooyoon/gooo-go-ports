package nativepolicy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func editSavedObject(t *testing.T, raw []byte, path []string, edit func(map[string]json.RawMessage)) []byte {
	t.Helper()
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		t.Fatal("test object")
	}
	if len(path) == 0 {
		edit(obj)
	} else {
		obj[path[0]] = editSavedObject(t, obj[path[0]], path[1:], edit)
	}
	result, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func replaceSavedObject(t *testing.T, raw []byte, path []string, replace func([]byte) []byte) []byte {
	t.Helper()
	if len(path) == 0 {
		return replace(raw)
	}
	return editSavedObject(t, raw, path[:len(path)-1], func(obj map[string]json.RawMessage) { obj[path[len(path)-1]] = replace(obj[path[len(path)-1]]) })
}

func TestSavedSchemaRejectsEveryMissingNullWrongDuplicateField(t *testing.T) {
	raw, _ := json.Marshal(replaySample(t))
	for _, path := range [][]string{nil, {"facts"}, {"decision"}, {"bindings"}} {
		var root map[string]json.RawMessage
		json.Unmarshal(raw, &root)
		object := root
		if len(path) != 0 {
			object = map[string]json.RawMessage{}
			json.Unmarshal(root[path[0]], &object)
		}
		for key, value := range object {
			for _, kind := range []string{"missing", "null", "wrong_type", "duplicate", "escaped_duplicate"} {
				t.Run(strings.Join(append(append([]string{}, path...), key, kind), "/"), func(t *testing.T) {
					var bad []byte
					if kind == "duplicate" || kind == "escaped_duplicate" {
						duplicateKey, _ := json.Marshal(key)
						if kind == "escaped_duplicate" {
							duplicateKey = []byte(fmt.Sprintf(`"\u%04x%s"`, []rune(key)[0], key[1:]))
						}
						bad = replaceSavedObject(t, raw, path, func(object []byte) []byte {
							return []byte(strings.TrimSuffix(string(object), "}") + "," + string(duplicateKey) + ":" + string(value) + "}")
						})
					} else {
						bad = editSavedObject(t, raw, path, func(obj map[string]json.RawMessage) {
							switch kind {
							case "missing":
								delete(obj, key)
							case "null":
								obj[key] = []byte("null")
							case "wrong_type":
								obj[key] = []byte("17")
							}
						})
					}
					if _, err := DecodeAPIProof(bad); err == nil {
						t.Fatal("malformed saved field accepted")
					}
				})
			}
		}
		label := strings.Join(path, "/")
		if label == "" {
			label = "root"
		}
		t.Run(label+"/unknown", func(t *testing.T) {
			bad := editSavedObject(t, raw, path, func(obj map[string]json.RawMessage) { obj["extra"] = []byte("true") })
			if _, err := DecodeAPIProof(bad); err == nil {
				t.Fatal("unknown saved field accepted")
			}
		})
	}
}

func TestSavedAssetMapRejectsMalformedEntries(t *testing.T) {
	raw, _ := json.Marshal(replaySample(t))
	name := boundNames[0]
	for label, value := range map[string]string{"null": "null", "boolean": "true", "number": "1", "object": "{}", "array": "[]", "high_surrogate": `"\ud800"`, "low_surrogate": `"\udc00"`} {
		t.Run(label, func(t *testing.T) {
			bad := editSavedObject(t, raw, []string{"bindings", "assets"}, func(obj map[string]json.RawMessage) { obj[name] = []byte(value) })
			if _, err := DecodeAPIProof(bad); err == nil {
				t.Fatal("malformed asset value accepted")
			}
		})
	}
	for _, label := range []string{"duplicate", "escaped_duplicate", "surrogate_key", "oversized_key", "too_many", "empty"} {
		t.Run(label, func(t *testing.T) {
			bad := replaceSavedObject(t, raw, []string{"bindings", "assets"}, func(object []byte) []byte {
				if label == "empty" {
					return []byte("{}")
				}
				if label == "too_many" {
					m := map[string]string{}
					for i := 0; i < maxAssetCount+1; i++ {
						m[fmt.Sprint(i)] = "sha256:0"
					}
					v, _ := json.Marshal(m)
					return v
				}
				key, _ := json.Marshal(name)
				if label == "escaped_duplicate" {
					key = []byte(fmt.Sprintf(`"\u%04x%s"`, []rune(name)[0], name[1:]))
				}
				if label == "surrogate_key" {
					key = []byte(`"\ud800"`)
				}
				if label == "oversized_key" {
					key, _ = json.Marshal(strings.Repeat("x", maxAssetNameBytes+1))
				}
				return []byte(strings.TrimSuffix(string(object), "}") + "," + string(key) + `:"sha256:0"}`)
			})
			if _, err := DecodeAPIProof(bad); err == nil {
				t.Fatal("malformed asset map accepted")
			}
		})
	}
}

func TestSavedEnvelopeUnicodeSizeAndTrailingRefused(t *testing.T) {
	p := replaySample(t)
	raw, _ := json.Marshal(p)
	for label, bad := range map[string][]byte{"null": []byte("null"), "array": []byte("[]"), "empty": []byte("{}"), "trailing": append(append([]byte{}, raw...), []byte(" {}")...), "invalid_utf8": append(append([]byte{}, raw...), 0xff), "over4MiB": []byte(strings.Repeat(" ", MaxBatchBytes+1))} {
		t.Run(label, func(t *testing.T) {
			if _, err := DecodeAPIProof(bad); err == nil {
				t.Fatal("invalid saved envelope accepted")
			}
		})
	}
	for _, path := range [][]string{{"facts", "group_value"}, {"facts", "principal_value"}, {"facts", "caller_tenant"}, {"facts", "resource_tenant"}, {"facts", "requested_action"}, {"facts", "allowed_action"}, {"decision", "reason"}, {"decision", "access_result"}, {"decision", "effective_principal"}, {"bindings", "compiler_source_sha"}, {"bindings", "generation_sha256"}} {
		for label, value := range map[string]string{"high_surrogate": `"\ud800"`, "low_surrogate": `"\udc00"`, "bad_pair": `"\ud800\u0061"`, "oversized_text": `"` + strings.Repeat("x", MaxTextBytes+1) + `"`} {
			t.Run(strings.Join(path, "/")+"/"+label, func(t *testing.T) {
				bad := replaceSavedObject(t, raw, path, func([]byte) []byte { return []byte(value) })
				if _, err := DecodeAPIProof(bad); err == nil {
					t.Fatal("lossy/outside-domain saved text accepted")
				}
			})
		}
	}
	for _, path := range [][]string{nil, {"facts"}, {"decision"}, {"bindings"}} {
		bad := replaceSavedObject(t, raw, path, func(object []byte) []byte { return []byte(strings.TrimSuffix(string(object), "}") + `,"\ud800":true}`) })
		if _, err := DecodeAPIProof(bad); err == nil {
			t.Fatal("isolated surrogate object key accepted")
		}
	}
	for _, value := range []string{"😀", "\ufffd", strings.Repeat("x", MaxTextBytes)} {
		p.Facts.PrincipalValue = value
		p, err := EvaluateAPI(p.Facts)
		if err != nil {
			t.Fatal(err)
		}
		decoded := cloneProof(t, p)
		if !reflect.DeepEqual(decoded, p) {
			t.Fatal("valid Unicode/boundary text changed")
		}
		if _, err := VerifyAPIProof(decoded); err != nil {
			t.Fatal(err)
		}
	}
	paired := replaceSavedObject(t, raw, []string{"facts", "group_value"}, func([]byte) []byte { return []byte(`"\ud83d\ude00"`) })
	if p, err := DecodeAPIProof(paired); err != nil || p.Facts.GroupValue != "😀" {
		t.Fatal("paired surrogate refused or changed")
	}
}

func TestSavedBatchLimitsAndLaterMalformedMember(t *testing.T) {
	raw, _ := json.Marshal(replaySample(t))
	good := []byte("[" + string(raw) + "]")
	if proofs, err := DecodeAPIProofBatch(good); err != nil || len(proofs) != 1 {
		t.Fatal("valid proof batch refused")
	}
	for _, bad := range [][]byte{[]byte("[]"), []byte("null"), raw, []byte("[" + string(raw) + ",{}]"), append(good, []byte(" {}")...), append(good, 0xff), []byte(strings.Repeat(" ", MaxBatchBytes+1))} {
		if _, err := DecodeAPIProofBatch(bad); err == nil {
			t.Fatal("invalid saved batch accepted")
		}
	}
	// A small structurally valid proof isolates the count limit from size and
	// source identity gates; VerifyAPIProof separately refuses this asset roster.
	small := editSavedObject(t, raw, []string{"bindings"}, func(obj map[string]json.RawMessage) { obj["assets"] = []byte(`{"a":"b"}`) })
	boundary := []byte("[" + strings.TrimSuffix(strings.Repeat(string(small)+",", MaxBatchCount), ",") + "]")
	if len(boundary) > MaxBatchBytes {
		t.Fatal("count probe unexpectedly over byte bound")
	}
	if rows, err := DecodeAPIProofBatch(boundary); err != nil || len(rows) != MaxBatchCount {
		t.Fatal("bounded batch count refused")
	}
	oversize := []byte("[" + strings.TrimSuffix(strings.Repeat(string(small)+",", MaxBatchCount+1), ",") + "]")
	if _, err := DecodeAPIProofBatch(oversize); err == nil {
		t.Fatal("oversized batch count accepted")
	}
}
